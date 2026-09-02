package service

import (
	"context"
	"errors"

	"github.com/zuquanzhi/Chirp/backend/internal/domain"
)

// TrashService orchestrates soft-delete / restore / permanent-delete across
// folders, files, physical storage and quota accounting.
type TrashService struct {
	folderRepo   domain.FolderRepository
	resourceRepo domain.ResourceRepository
	userRepo     domain.UserRepository
	storage      FileStorage
}

func NewTrashService(folderRepo domain.FolderRepository, resourceRepo domain.ResourceRepository, userRepo domain.UserRepository, storage FileStorage) *TrashService {
	return &TrashService{folderRepo: folderRepo, resourceRepo: resourceRepo, userRepo: userRepo, storage: storage}
}

// ---- Soft delete (move to trash) ----

// DeleteFolder soft-deletes a folder, its descendant folders, and all files inside the subtree.
func (s *TrashService) DeleteFolder(ctx context.Context, ownerID, folderID int64) error {
	if err := s.ownedFolder(ctx, ownerID, folderID); err != nil {
		return err
	}
	subtree, err := s.subtreeIDs(ctx, folderID)
	if err != nil {
		return err
	}
	for _, id := range subtree {
		if err := s.resourceRepo.SoftDeleteByFolder(ctx, id); err != nil {
			return err
		}
		if err := s.folderRepo.SoftDelete(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// ---- Trash listing ----

// ListTrashFolders returns top-level soft-deleted folders (those whose parent is not deleted).
func (s *TrashService) ListTrashFolders(ctx context.Context, ownerID int64) ([]domain.Folder, error) {
	deleted, err := s.folderRepo.ListDeleted(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	deletedIDs := make(map[int64]bool, len(deleted))
	for _, f := range deleted {
		deletedIDs[f.ID] = true
	}
	var top []domain.Folder
	for _, f := range deleted {
		if f.ParentID == nil || !deletedIDs[*f.ParentID] {
			top = append(top, f)
		}
	}
	return top, nil
}

// ListTrashFiles returns deleted files that are not inside a deleted folder.
func (s *TrashService) ListTrashFiles(ctx context.Context, ownerID int64) ([]domain.Resource, error) {
	deleted, err := s.resourceRepo.ListDeleted(ctx, ownerID)
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

// ---- Restore ----

// RestoreFolder restores a folder, its descendant folders and all files in the subtree.
// If the original parent was deleted too, the folder is re-attached to the drive root.
func (s *TrashService) RestoreFolder(ctx context.Context, ownerID, folderID int64) error {
	f, err := s.folderRepo.GetByID(ctx, folderID)
	if err != nil {
		return err
	}
	if f == nil || f.OwnerID != ownerID || f.DeletedAt == nil {
		return errors.New("folder not found in trash")
	}

	// Re-attach to root if parent is gone or still deleted
	if f.ParentID != nil {
		parent, err := s.folderRepo.GetByID(ctx, *f.ParentID)
		if err != nil {
			return err
		}
		if parent == nil || parent.DeletedAt != nil {
			f.ParentID = nil
			if err := s.folderRepo.Update(ctx, f); err != nil {
				return err
			}
		}
	}

	subtree, err := s.subtreeIDs(ctx, folderID)
	if err != nil {
		return err
	}
	for _, id := range subtree {
		if err := s.folderRepo.Restore(ctx, id); err != nil {
			return err
		}
		if err := s.resourceRepo.RestoreByFolder(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// RestoreFile clears the trash mark; files whose folder is still deleted go back to root.
func (s *TrashService) RestoreFile(ctx context.Context, ownerID, fileID int64) error {
	res, err := s.trashedFile(ctx, ownerID, fileID)
	if err != nil {
		return err
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
	return s.resourceRepo.Restore(ctx, fileID, folderID)
}

// ---- Permanent delete ----

// HardDeleteFolder permanently removes a folder subtree: physical files, rows, and quota accounting.
func (s *TrashService) HardDeleteFolder(ctx context.Context, ownerID, folderID int64) error {
	f, err := s.folderRepo.GetByID(ctx, folderID)
	if err != nil {
		return err
	}
	if f == nil || f.OwnerID != ownerID || f.DeletedAt == nil {
		return errors.New("folder not found in trash")
	}

	subtree, err := s.subtreeIDs(ctx, folderID)
	if err != nil {
		return err
	}

	var freed int64
	for _, id := range subtree {
		files, err := s.resourceRepo.ListByFolderIncludingDeleted(ctx, id)
		if err != nil {
			return err
		}
		for _, file := range files {
			if err := s.storage.Delete(ctx, file.Filename); err != nil {
				return err
			}
			freed += file.Size
		}
		if err := s.resourceRepo.HardDeleteByFolder(ctx, id); err != nil {
			return err
		}
		if err := s.folderRepo.HardDelete(ctx, id); err != nil {
			return err
		}
	}
	if freed > 0 {
		return s.userRepo.AddUsed(ctx, ownerID, -freed)
	}
	return nil
}

// HardDeleteFile permanently removes a file: physical object, row, and quota accounting.
func (s *TrashService) HardDeleteFile(ctx context.Context, ownerID, fileID int64) error {
	res, err := s.trashedFile(ctx, ownerID, fileID)
	if err != nil {
		return err
	}
	if err := s.storage.Delete(ctx, res.Filename); err != nil {
		return err
	}
	if err := s.resourceRepo.HardDelete(ctx, fileID); err != nil {
		return err
	}
	return s.userRepo.AddUsed(ctx, ownerID, -res.Size)
}

// ---- helpers ----

func (s *TrashService) subtreeIDs(ctx context.Context, folderID int64) ([]int64, error) {
	desc, err := s.folderRepo.ListDescendantIDs(ctx, folderID)
	if err != nil {
		return nil, err
	}
	return append([]int64{folderID}, desc...), nil
}

func (s *TrashService) ownedFolder(ctx context.Context, ownerID, folderID int64) error {
	f, err := s.folderRepo.GetByID(ctx, folderID)
	if err != nil {
		return err
	}
	if f == nil || f.OwnerID != ownerID || f.DeletedAt != nil {
		return errors.New("folder not found")
	}
	return nil
}

func (s *TrashService) trashedFile(ctx context.Context, ownerID, fileID int64) (*domain.Resource, error) {
	res, err := s.resourceRepo.GetByID(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if res == nil || res.OwnerID == nil || *res.OwnerID != ownerID || res.DeletedAt == nil {
		return nil, errors.New("file not found in trash")
	}
	return res, nil
}
