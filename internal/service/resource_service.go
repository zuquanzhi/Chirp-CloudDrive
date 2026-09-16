package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"mime/multipart"
	"path/filepath"
	"strconv"

	"github.com/google/uuid"
	"github.com/zuquanzhi/Chirp/backend/internal/domain"
)

type ResourceService struct {
	repo       domain.ResourceRepository
	storage    FileStorage
	userRepo   domain.UserRepository
	folderRepo domain.FolderRepository
	activity   *ActivityRecorder
}

func NewResourceService(repo domain.ResourceRepository, storage FileStorage, userRepo domain.UserRepository, folderRepo domain.FolderRepository) *ResourceService {
	return &ResourceService{
		repo:       repo,
		storage:    storage,
		userRepo:   userRepo,
		folderRepo: folderRepo,
	}
}

// SetActivityRecorder attaches the activity log recorder (optional).
func (s *ResourceService) SetActivityRecorder(rec *ActivityRecorder) {
	s.activity = rec
}

func (s *ResourceService) Upload(ctx context.Context, ownerID *int64, title, desc, subject, resourceType string, file multipart.File, header *multipart.FileHeader) (*domain.Resource, error) {
	// Calculate Hash
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, err
	}
	fileHash := hex.EncodeToString(hash.Sum(nil))

	// Reset file pointer
	file.Seek(0, 0)

	id := uuid.New().String()
	ext := filepath.Ext(header.Filename)
	storedName := id + ext

	// Use Storage Interface
	savedName, size, err := s.storage.Save(ctx, file, storedName)
	if err != nil {
		return nil, err
	}

	res := &domain.Resource{
		OwnerID:      ownerID,
		Title:        title,
		Description:  desc,
		Filename:     savedName, // Store the key/path returned by storage
		OriginalName: header.Filename,
		Size:         size,
		FileHash:     fileHash,
		Status:       domain.ResourceStatusPending,
		Subject:      subject,
		Type:         resourceType,
	}

	if err := s.repo.Create(ctx, res); err != nil {
		return nil, err
	}

	// Populate URL
	res.URL = s.storage.GetPublicURL(savedName)

	return res, nil
}

func (s *ResourceService) List(ctx context.Context, status domain.ResourceStatus, search string) ([]domain.Resource, error) {
	list, err := s.repo.List(ctx, status, search)
	if err != nil {
		return nil, err
	}
	// Populate URLs
	for i := range list {
		list[i].URL = s.storage.GetPublicURL(list[i].Filename)
	}
	return list, nil
}

func (s *ResourceService) GetDownloadPath(ctx context.Context, id int64) (*domain.Resource, string, error) {
	res, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, "", err
	}
	if res == nil {
		return nil, "", nil
	}
	// Note: This method signature implies returning a local path, which might not work for OSS.
	// Ideally, we should return a ReadCloser or a URL.
	// For now, let's keep it compatible with LocalStorage logic in Handler,
	// but in a real OSS scenario, the Handler should use s.storage.Get() or s.storage.GetPublicURL().
	// We will refactor the Handler to use the Service's GetContent method instead.
	return res, res.Filename, nil
}

// New method to get file content
func (s *ResourceService) GetFileContent(ctx context.Context, id int64) (*domain.Resource, io.ReadCloser, error) {
	res, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if res == nil {
		return nil, nil, nil
	}

	reader, err := s.storage.Get(ctx, res.Filename)
	if err != nil {
		return nil, nil, err
	}
	return res, reader, nil
}

func (s *ResourceService) Review(ctx context.Context, id int64, status domain.ResourceStatus) error {
	return s.repo.UpdateStatus(ctx, id, status)
}

func (s *ResourceService) CheckDuplicate(ctx context.Context, hash string) ([]domain.Resource, error) {
	return s.repo.GetByHash(ctx, hash)
}

// ---- Drive file operations ----

// saveDriveFile is the shared core of every drive upload path (multipart, instant,
// chunked merge). It handles hash-based dedup (instant upload), version history,
// quota accounting and the activity log.
//
// content is required unless an existing object with the same hash is found.
// hash may be empty only when content is provided (it is computed while streaming).
func (s *ResourceService) saveDriveFile(ctx context.Context, ownerID int64, folderID *int64, name, hash string, declaredSize int64, content io.Reader) (*domain.Resource, bool, error) {
	// Quota check
	u, err := s.userRepo.GetByID(ctx, ownerID)
	if err != nil {
		return nil, false, err
	}
	if u == nil {
		return nil, false, errors.New("user not found")
	}
	if u.Used+declaredSize > u.Quota {
		return nil, false, errors.New("quota exceeded")
	}

	// Folder must belong to the user and not be deleted
	if folderID != nil {
		f, err := s.folderRepo.GetByID(ctx, *folderID)
		if err != nil {
			return nil, false, err
		}
		if f == nil || f.OwnerID != ownerID || f.DeletedAt != nil {
			return nil, false, errors.New("folder not found")
		}
	}

	// Dedup: reuse the physical object of an identical file (instant upload).
	instant := false
	var storedName string
	var size int64
	if hash != "" {
		if existing, err := s.repo.FindByHash(ctx, hash); err != nil {
			return nil, false, err
		} else if existing != nil {
			instant = true
			storedName = existing.Filename
			size = existing.Size
		}
	}
	if !instant {
		if content == nil {
			return nil, false, errors.New("content not found for hash")
		}
		ext := filepath.Ext(name)
		sn, sz, err := s.storage.Save(ctx, content, uuid.New().String()+ext)
		if err != nil {
			return nil, false, err
		}
		storedName, size = sn, sz
	}

	// Version history: same name in the same folder becomes a new version.
	group := ""
	version := 1
	prev, err := s.repo.GetLatestByName(ctx, ownerID, folderID, name)
	if err != nil {
		return nil, false, err
	}
	if prev != nil {
		group = prev.VersionGroup
		if group == "" {
			group = uuid.New().String()
			if err := s.repo.SetVersionGroup(ctx, prev.ID, group); err != nil {
				return nil, false, err
			}
		}
		if err := s.repo.SetLatest(ctx, prev.ID, false); err != nil {
			return nil, false, err
		}
		version = prev.Version + 1
	}

	res := &domain.Resource{
		OwnerID:      &ownerID,
		FolderID:     folderID,
		Title:        name,
		Filename:     storedName,
		OriginalName: name,
		Size:         size,
		FileHash:     hash,
		Status:       domain.ResourceStatusApproved, // personal drive files need no review
		VersionGroup: group,
		Version:      version,
		IsLatest:     true,
	}
	if err := s.repo.Create(ctx, res); err != nil {
		return nil, false, err
	}
	if err := s.userRepo.AddUsed(ctx, ownerID, size); err != nil {
		return nil, false, err
	}

	res.URL = s.storage.GetPublicURL(storedName)

	// Activity log (best effort)
	action := domain.ActivityUpload
	switch {
	case instant:
		action = domain.ActivityInstantUpload
	case prev != nil:
		action = domain.ActivityNewVersion
	}
	detail := ""
	if prev != nil {
		detail = "v" + strconv.Itoa(version)
	}
	s.activity.Log(ctx, ownerID, action, "file", res.ID, name, detail)

	return res, instant, nil
}

// UploadToFolder uploads a file into the user's drive folder (nil = root) with quota accounting.
func (s *ResourceService) UploadToFolder(ctx context.Context, ownerID int64, folderID *int64, file multipart.File, header *multipart.FileHeader) (*domain.Resource, error) {
	res, _, err := s.UploadToFolderEx(ctx, ownerID, folderID, file, header)
	return res, err
}

// UploadToFolderEx additionally reports whether the upload was served by dedup (instant).
func (s *ResourceService) UploadToFolderEx(ctx context.Context, ownerID int64, folderID *int64, file multipart.File, header *multipart.FileHeader) (*domain.Resource, bool, error) {
	// Hash first so dedup can skip the physical write.
	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return nil, false, err
	}
	hash := hex.EncodeToString(h.Sum(nil))
	if _, err := file.Seek(0, 0); err != nil {
		return nil, false, err
	}
	return s.saveDriveFile(ctx, ownerID, folderID, header.Filename, hash, header.Size, file)
}

// InstantUpload creates a file entry reusing already-stored content (秒传).
// Returns an error when no stored object matches the hash.
func (s *ResourceService) InstantUpload(ctx context.Context, ownerID int64, folderID *int64, name, hash string, size int64) (*domain.Resource, error) {
	if hash == "" {
		return nil, errors.New("file hash required")
	}
	res, _, err := s.saveDriveFile(ctx, ownerID, folderID, name, hash, size, nil)
	return res, err
}

// CompleteUpload stores merged content (e.g. from a chunked upload) as a drive file.
func (s *ResourceService) CompleteUpload(ctx context.Context, ownerID int64, folderID *int64, name, hash string, size int64, content io.Reader) (*domain.Resource, error) {
	res, _, err := s.saveDriveFile(ctx, ownerID, folderID, name, hash, size, content)
	return res, err
}

// ListDriveFiles lists non-deleted latest-version files of a folder (nil = root), optional name search.
func (s *ResourceService) ListDriveFiles(ctx context.Context, ownerID int64, folderID *int64, search string) ([]domain.Resource, error) {
	list, err := s.repo.ListByFolder(ctx, ownerID, folderID, search)
	if err != nil {
		return nil, err
	}
	for i := range list {
		list[i].URL = s.storage.GetPublicURL(list[i].Filename)
	}
	return list, nil
}

// RenameFile changes the display name of a drive file.
func (s *ResourceService) RenameFile(ctx context.Context, ownerID, fileID int64, name string) (*domain.Resource, error) {
	if name == "" {
		return nil, errors.New("file name required")
	}
	res, err := s.ownedFile(ctx, ownerID, fileID)
	if err != nil {
		return nil, err
	}
	old := res.OriginalName
	res.OriginalName = name
	res.Title = name
	if err := s.repo.Update(ctx, res); err != nil {
		return nil, err
	}
	s.activity.Log(ctx, ownerID, domain.ActivityRename, "file", res.ID, name, "原文件名: "+old)
	return res, nil
}

// MoveFile puts a file into another folder (nil = root).
func (s *ResourceService) MoveFile(ctx context.Context, ownerID, fileID int64, folderID *int64) (*domain.Resource, error) {
	res, err := s.ownedFile(ctx, ownerID, fileID)
	if err != nil {
		return nil, err
	}
	if folderID != nil {
		f, err := s.folderRepo.GetByID(ctx, *folderID)
		if err != nil {
			return nil, err
		}
		if f == nil || f.OwnerID != ownerID || f.DeletedAt != nil {
			return nil, errors.New("target folder not found")
		}
	}
	res.FolderID = folderID
	if err := s.repo.Update(ctx, res); err != nil {
		return nil, err
	}
	s.activity.Log(ctx, ownerID, domain.ActivityMove, "file", res.ID, res.OriginalName, "")
	return res, nil
}

// SoftDeleteFile moves a file (and all its versions) to trash.
func (s *ResourceService) SoftDeleteFile(ctx context.Context, ownerID, fileID int64) error {
	res, err := s.ownedFile(ctx, ownerID, fileID)
	if err != nil {
		return err
	}
	if res.VersionGroup != "" {
		if err := s.repo.SoftDeleteByGroup(ctx, res.VersionGroup); err != nil {
			return err
		}
	} else if err := s.repo.SoftDelete(ctx, fileID); err != nil {
		return err
	}
	s.activity.Log(ctx, ownerID, domain.ActivityDelete, "file", res.ID, res.OriginalName, "移入回收站")
	return nil
}

// DownloadDriveFile returns file content after ownership check.
func (s *ResourceService) DownloadDriveFile(ctx context.Context, ownerID, fileID int64) (*domain.Resource, io.ReadCloser, error) {
	res, err := s.repo.GetByID(ctx, fileID)
	if err != nil {
		return nil, nil, err
	}
	if res == nil || res.OwnerID == nil || *res.OwnerID != ownerID || res.DeletedAt != nil {
		return nil, nil, errors.New("file not found")
	}
	reader, err := s.storage.Get(ctx, res.Filename)
	if err != nil {
		return nil, nil, err
	}
	return res, reader, nil
}

// ---- Version history ----

// ListVersions returns all versions of the logical file containing fileID, newest first.
func (s *ResourceService) ListVersions(ctx context.Context, ownerID, fileID int64) ([]domain.Resource, error) {
	res, err := s.ownedFile(ctx, ownerID, fileID)
	if err != nil {
		return nil, err
	}
	if res.VersionGroup == "" {
		return []domain.Resource{*res}, nil
	}
	versions, err := s.repo.ListVersions(ctx, res.VersionGroup)
	if err != nil {
		return nil, err
	}
	// Hide versions the user cannot see (e.g. soft-deleted ones are kept out of the UI).
	out := make([]domain.Resource, 0, len(versions))
	for _, v := range versions {
		if v.DeletedAt != nil {
			continue
		}
		v.URL = s.storage.GetPublicURL(v.Filename)
		out = append(out, v)
	}
	return out, nil
}

// RestoreVersion creates a new latest version pointing at the content of versionID.
func (s *ResourceService) RestoreVersion(ctx context.Context, ownerID, fileID, versionID int64) (*domain.Resource, error) {
	base, err := s.ownedFile(ctx, ownerID, fileID)
	if err != nil {
		return nil, err
	}
	if base.VersionGroup == "" {
		return nil, errors.New("file has no version history")
	}
	target, err := s.repo.GetByID(ctx, versionID)
	if err != nil {
		return nil, err
	}
	if target == nil || target.OwnerID == nil || *target.OwnerID != ownerID ||
		target.VersionGroup != base.VersionGroup || target.DeletedAt != nil {
		return nil, errors.New("version not found")
	}

	latest, err := s.repo.GetLatestByName(ctx, ownerID, base.FolderID, base.OriginalName)
	if err != nil {
		return nil, err
	}
	nextVersion := base.Version
	if latest != nil {
		nextVersion = latest.Version
		if err := s.repo.SetLatest(ctx, latest.ID, false); err != nil {
			return nil, err
		}
	}
	nextVersion++

	res := &domain.Resource{
		OwnerID:      &ownerID,
		FolderID:     base.FolderID,
		Title:        base.OriginalName,
		Filename:     target.Filename, // refcounted physical reuse
		OriginalName: base.OriginalName,
		Size:         target.Size,
		FileHash:     target.FileHash,
		Status:       domain.ResourceStatusApproved,
		VersionGroup: base.VersionGroup,
		Version:      nextVersion,
		IsLatest:     true,
	}
	if err := s.repo.Create(ctx, res); err != nil {
		return nil, err
	}
	res.URL = s.storage.GetPublicURL(res.Filename)
	s.activity.Log(ctx, ownerID, domain.ActivityRestoreVersion, "file", res.ID, res.OriginalName, "恢复到 v"+strconv.Itoa(target.Version))
	return res, nil
}

func (s *ResourceService) ownedFile(ctx context.Context, ownerID, fileID int64) (*domain.Resource, error) {
	res, err := s.repo.GetByID(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if res == nil || res.OwnerID == nil || *res.OwnerID != ownerID || res.DeletedAt != nil {
		return nil, errors.New("file not found")
	}
	return res, nil
}

