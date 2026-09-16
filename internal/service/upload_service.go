package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/google/uuid"
	"github.com/zuquanzhi/Chirp/backend/internal/domain"
)

// DefaultChunkSize is used when the client does not specify one (8 MiB).
const DefaultChunkSize int64 = 8 << 20

// UploadService implements chunked / resumable uploads.
type UploadService struct {
	sessionRepo domain.UploadSessionRepository
	resourceSvc *ResourceService
	chunkDir    string
	activity    *ActivityRecorder
}

// NewUploadService stores session chunks under <uploadDir>/.chunks.
func NewUploadService(sessionRepo domain.UploadSessionRepository, resourceSvc *ResourceService, uploadDir string) (*UploadService, error) {
	chunkDir := filepath.Join(uploadDir, ".chunks")
	if err := os.MkdirAll(chunkDir, 0o755); err != nil {
		return nil, err
	}
	return &UploadService{sessionRepo: sessionRepo, resourceSvc: resourceSvc, chunkDir: chunkDir}, nil
}

func (s *UploadService) SetActivityRecorder(rec *ActivityRecorder) {
	s.activity = rec
}

// InitResult is returned by Init: either an instant-upload hit or a session to fill.
type InitResult struct {
	Instant  bool                 `json:"instant"`
	Resource *domain.Resource     `json:"resource,omitempty"`
	Session  *domain.UploadSession `json:"session,omitempty"`
}

// Init starts a chunked upload. If the file hash matches stored content, the file
// is created immediately (秒传) and no chunks are needed.
func (s *UploadService) Init(ctx context.Context, ownerID int64, folderID *int64, filename string, size int64, fileHash string, chunkSize int64) (*InitResult, error) {
	if filename == "" || size <= 0 {
		return nil, errors.New("filename and size required")
	}
	if fileHash != "" {
		res, err := s.resourceSvc.InstantUpload(ctx, ownerID, folderID, filename, fileHash, size)
		if err == nil {
			return &InitResult{Instant: true, Resource: res}, nil
		}
		// Only fall through to a real session when the content is unknown;
		// other failures (quota, folder) must surface to the caller.
		if err.Error() != "content not found for hash" {
			return nil, err
		}
	}

	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}
	total := int((size + chunkSize - 1) / chunkSize)
	session := &domain.UploadSession{
		ID:          uuid.New().String(),
		OwnerID:     ownerID,
		FolderID:    folderID,
		Filename:    filename,
		Size:        size,
		ChunkSize:   chunkSize,
		TotalChunks: total,
		FileHash:    fileHash,
	}
	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return nil, err
	}
	session.UploadedChunks = []int{}
	return &InitResult{Session: session}, nil
}

// Get returns the session together with the indexes already on disk (for resume).
func (s *UploadService) Get(ctx context.Context, ownerID int64, sessionID string) (*domain.UploadSession, error) {
	session, err := s.ownedSession(ctx, ownerID, sessionID)
	if err != nil {
		return nil, err
	}
	session.UploadedChunks = s.uploadedChunks(sessionID)
	return session, nil
}

// SaveChunk persists one chunk (index within [0, TotalChunks)).
func (s *UploadService) SaveChunk(ctx context.Context, ownerID int64, sessionID string, index int, content io.Reader) error {
	session, err := s.ownedSession(ctx, ownerID, sessionID)
	if err != nil {
		return err
	}
	if index < 0 || index >= session.TotalChunks {
		return errors.New("chunk index out of range")
	}
	dir := filepath.Join(s.chunkDir, sessionID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(dir, strconv.Itoa(index)))
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, content)
	return err
}

// Complete merges all chunks, verifies size/hash and creates the drive file.
func (s *UploadService) Complete(ctx context.Context, ownerID int64, sessionID string) (*domain.Resource, error) {
	session, err := s.ownedSession(ctx, ownerID, sessionID)
	if err != nil {
		return nil, err
	}
	uploaded := s.uploadedChunks(sessionID)
	if len(uploaded) != session.TotalChunks {
		return nil, fmt.Errorf("missing chunks: %d/%d uploaded", len(uploaded), session.TotalChunks)
	}

	// Merge into a temp file while hashing.
	tmp, err := os.CreateTemp(s.chunkDir, sessionID+".merged-*")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	h := sha256.New()
	out := io.MultiWriter(tmp, h)
	dir := filepath.Join(s.chunkDir, sessionID)
	for i := 0; i < session.TotalChunks; i++ {
		chunk, err := os.Open(filepath.Join(dir, strconv.Itoa(i)))
		if err != nil {
			tmp.Close()
			return nil, fmt.Errorf("read chunk %d: %w", i, err)
		}
		_, copyErr := io.Copy(out, chunk)
		chunk.Close()
		if copyErr != nil {
			tmp.Close()
			return nil, copyErr
		}
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}

	info, err := os.Stat(tmpPath)
	if err != nil {
		return nil, err
	}
	if info.Size() != session.Size {
		return nil, fmt.Errorf("size mismatch: expected %d, got %d", session.Size, info.Size())
	}
	hash := hex.EncodeToString(h.Sum(nil))
	if session.FileHash != "" && session.FileHash != hash {
		return nil, errors.New("file hash mismatch")
	}

	merged, err := os.Open(tmpPath)
	if err != nil {
		return nil, err
	}
	defer merged.Close()

	res, err := s.resourceSvc.CompleteUpload(ctx, ownerID, session.FolderID, session.Filename, hash, session.Size, merged)
	if err != nil {
		return nil, err
	}

	// Cleanup session row + chunk files.
	_ = s.sessionRepo.Delete(ctx, sessionID)
	_ = os.RemoveAll(dir)
	s.activity.Log(ctx, ownerID, domain.ActivityChunkedUpload, "file", res.ID, res.OriginalName, fmt.Sprintf("%d 个分片", session.TotalChunks))
	return res, nil
}

// Abort discards a session and its chunks.
func (s *UploadService) Abort(ctx context.Context, ownerID int64, sessionID string) error {
	if _, err := s.ownedSession(ctx, ownerID, sessionID); err != nil {
		return err
	}
	if err := s.sessionRepo.Delete(ctx, sessionID); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(s.chunkDir, sessionID))
}

func (s *UploadService) ownedSession(ctx context.Context, ownerID int64, sessionID string) (*domain.UploadSession, error) {
	session, err := s.sessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil || session.OwnerID != ownerID {
		return nil, errors.New("upload session not found")
	}
	return session, nil
}

// uploadedChunks scans the chunk directory for written chunk indexes.
func (s *UploadService) uploadedChunks(sessionID string) []int {
	entries, err := os.ReadDir(filepath.Join(s.chunkDir, sessionID))
	if err != nil {
		return []int{}
	}
	out := make([]int, 0, len(entries))
	for _, e := range entries {
		if n, err := strconv.Atoi(e.Name()); err == nil {
			out = append(out, n)
		}
	}
	sort.Ints(out)
	return out
}
