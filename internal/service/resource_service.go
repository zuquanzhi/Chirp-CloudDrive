package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"mime/multipart"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/zuquanzhi/Chirp/backend/internal/domain"
)

type ResourceService struct {
	repo       domain.ResourceRepository
	storage    FileStorage
	userRepo   domain.UserRepository
	folderRepo domain.FolderRepository
}

func NewResourceService(repo domain.ResourceRepository, storage FileStorage, userRepo domain.UserRepository, folderRepo domain.FolderRepository) *ResourceService {
	return &ResourceService{
		repo:       repo,
		storage:    storage,
		userRepo:   userRepo,
		folderRepo: folderRepo,
	}
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

// UploadToFolder uploads a file into the user's drive folder (nil = root) with quota accounting.
func (s *ResourceService) UploadToFolder(ctx context.Context, ownerID int64, folderID *int64, file multipart.File, header *multipart.FileHeader) (*domain.Resource, error) {
	// Quota check
	u, err := s.userRepo.GetByID(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, errors.New("user not found")
	}
	if u.Used+header.Size > u.Quota {
		return nil, errors.New("quota exceeded")
	}

	// Folder must belong to the user and not be deleted
	if folderID != nil {
		f, err := s.folderRepo.GetByID(ctx, *folderID)
		if err != nil {
			return nil, err
		}
		if f == nil || f.OwnerID != ownerID || f.DeletedAt != nil {
			return nil, errors.New("folder not found")
		}
	}

	res, err := s.Upload(ctx, &ownerID, header.Filename, "", "", "", file, header)
	if err != nil {
		return nil, err
	}
	res.FolderID = folderID
	res.Status = domain.ResourceStatusApproved // personal drive files need no review
	if err := s.repo.Update(ctx, res); err != nil {
		return nil, err
	}
	// repo.Update only writes name/folder; fix status separately
	if err := s.repo.UpdateStatus(ctx, res.ID, domain.ResourceStatusApproved); err != nil {
		return nil, err
	}

	if err := s.userRepo.AddUsed(ctx, ownerID, res.Size); err != nil {
		return nil, err
	}
	return res, nil
}

// ListDriveFiles lists non-deleted files of a folder (nil = root), optional name search.
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
	res.OriginalName = name
	res.Title = name
	if err := s.repo.Update(ctx, res); err != nil {
		return nil, err
	}
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
	return res, nil
}

// SoftDeleteFile moves a file to trash.
func (s *ResourceService) SoftDeleteFile(ctx context.Context, ownerID, fileID int64) error {
	if _, err := s.ownedFile(ctx, ownerID, fileID); err != nil {
		return err
	}
	return s.repo.SoftDelete(ctx, fileID)
}

// ListTrashFiles returns deleted files that are not inside a deleted folder.
func (s *ResourceService) ListTrashFiles(ctx context.Context, ownerID int64) ([]domain.Resource, error) {
	deleted, err := s.repo.ListDeleted(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	deletedFolders, err := s.folderRepo.ListDeleted(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	deletedFolderIDs := make(map[int64]bool, len(deletedFolders))
	for _, f := range deletedFolders {
		deletedFolderIDs[f.ID] = true
	}
	var out []domain.Resource
	for _, res := range deleted {
		if res.FolderID != nil && deletedFolderIDs[*res.FolderID] {
			continue // shown together with its folder
		}
		res.URL = s.storage.GetPublicURL(res.Filename)
		out = append(out, res)
	}
	return out, nil
}

// RestoreFile clears the trash mark; files whose folder is still deleted go back to root.
func (s *ResourceService) RestoreFile(ctx context.Context, ownerID, fileID int64) error {
	res, err := s.repo.GetByID(ctx, fileID)
	if err != nil {
		return err
	}
	if res == nil || res.OwnerID == nil || *res.OwnerID != ownerID || res.DeletedAt == nil {
		return errors.New("file not found in trash")
	}
	folderID := res.FolderID
	if folderID != nil {
		f, err := s.folderRepo.GetByID(ctx, *folderID)
		if err != nil {
			return err
		}
		if f == nil || f.DeletedAt != nil {
			folderID = nil
		}
	}
	return s.repo.Restore(ctx, fileID, folderID)
}

// HardDeleteFile permanently removes a file: physical object, row, and quota accounting.
func (s *ResourceService) HardDeleteFile(ctx context.Context, ownerID, fileID int64) error {
	res, err := s.repo.GetByID(ctx, fileID)
	if err != nil {
		return err
	}
	if res == nil || res.OwnerID == nil || *res.OwnerID != ownerID || res.DeletedAt == nil {
		return errors.New("file not found in trash")
	}
	if err := s.storage.Delete(ctx, res.Filename); err != nil {
		return err
	}
	if err := s.repo.HardDelete(ctx, fileID); err != nil {
		return err
	}
	return s.userRepo.AddUsed(ctx, ownerID, -res.Size)
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
