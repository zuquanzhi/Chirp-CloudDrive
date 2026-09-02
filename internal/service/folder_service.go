package service

import (
	"context"
	"errors"

	"github.com/zuquanzhi/Chirp/backend/internal/domain"
)

type FolderService struct {
	folderRepo   domain.FolderRepository
	resourceRepo domain.ResourceRepository
	userRepo     domain.UserRepository
	storage      FileStorage
}

func NewFolderService(folderRepo domain.FolderRepository, resourceRepo domain.ResourceRepository, userRepo domain.UserRepository, storage FileStorage) *FolderService {
	return &FolderService{folderRepo: folderRepo, resourceRepo: resourceRepo, userRepo: userRepo, storage: storage}
}

// GetQuota returns the user's quota and used bytes.
func (s *FolderService) GetQuota(ctx context.Context, userID int64) (quota, used int64, err error) {
	u, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return 0, 0, err
	}
	if u == nil {
		return 0, 0, errors.New("user not found")
	}
	return u.Quota, u.Used, nil
}

// List returns the non-deleted child folders of parentID (nil = drive root).
func (s *FolderService) List(ctx context.Context, ownerID int64, parentID *int64) ([]domain.Folder, error) {
	if parentID != nil {
		parent, err := s.folderRepo.GetByID(ctx, *parentID)
		if err != nil {
			return nil, err
		}
		if parent == nil || parent.OwnerID != ownerID || parent.DeletedAt != nil {
			return nil, errors.New("folder not found")
		}
	}
	return s.folderRepo.ListByParent(ctx, ownerID, parentID)
}

// Create makes a new folder under parentID (nil = drive root).
func (s *FolderService) Create(ctx context.Context, ownerID int64, name string, parentID *int64) (*domain.Folder, error) {
	if name == "" {
		return nil, errors.New("folder name required")
	}
	if parentID != nil {
		parent, err := s.folderRepo.GetByID(ctx, *parentID)
		if err != nil {
			return nil, err
		}
		if parent == nil || parent.OwnerID != ownerID || parent.DeletedAt != nil {
			return nil, errors.New("parent folder not found")
		}
	}

	f := &domain.Folder{OwnerID: ownerID, ParentID: parentID, Name: name}
	if err := s.folderRepo.Create(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

// Rename changes a folder's name.
func (s *FolderService) Rename(ctx context.Context, ownerID, folderID int64, name string) (*domain.Folder, error) {
	if name == "" {
		return nil, errors.New("folder name required")
	}
	f, err := s.ownedFolder(ctx, ownerID, folderID)
	if err != nil {
		return nil, err
	}
	f.Name = name
	if err := s.folderRepo.Update(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

// Move puts a folder under newParentID (nil = drive root), rejecting moves into itself or its descendants.
func (s *FolderService) Move(ctx context.Context, ownerID, folderID int64, newParentID *int64) (*domain.Folder, error) {
	f, err := s.ownedFolder(ctx, ownerID, folderID)
	if err != nil {
		return nil, err
	}

	if newParentID != nil {
		if *newParentID == folderID {
			return nil, errors.New("cannot move folder into itself")
		}
		parent, err := s.folderRepo.GetByID(ctx, *newParentID)
		if err != nil {
			return nil, err
		}
		if parent == nil || parent.OwnerID != ownerID || parent.DeletedAt != nil {
			return nil, errors.New("target folder not found")
		}
		// Cycle check: target must not be inside the moved subtree
		descIDs, err := s.folderRepo.ListDescendantIDs(ctx, folderID)
		if err != nil {
			return nil, err
		}
		for _, id := range descIDs {
			if id == *newParentID {
				return nil, errors.New("cannot move folder into its own descendant")
			}
		}
	}

	f.ParentID = newParentID
	if err := s.folderRepo.Update(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

// Delete soft-deletes a folder, its descendant folders, and all files inside the subtree (trash).
func (s *FolderService) Delete(ctx context.Context, ownerID, folderID int64) error {
	if _, err := s.ownedFolder(ctx, ownerID, folderID); err != nil {
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

// ListTrash returns top-level soft-deleted folders (those whose parent is not deleted).
func (s *FolderService) ListTrash(ctx context.Context, ownerID int64) ([]domain.Folder, error) {
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

// RestoreFolder restores a folder, its descendant folders and all files in the subtree.
// If the original parent was deleted too, the folder is re-attached to the drive root.
func (s *FolderService) RestoreFolder(ctx context.Context, ownerID, folderID int64) error {
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

// HardDeleteFolder permanently removes a folder subtree: physical files, rows, and quota accounting.
func (s *FolderService) HardDeleteFolder(ctx context.Context, ownerID, folderID int64) error {
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

func (s *FolderService) subtreeIDs(ctx context.Context, folderID int64) ([]int64, error) {
	desc, err := s.folderRepo.ListDescendantIDs(ctx, folderID)
	if err != nil {
		return nil, err
	}
	return append([]int64{folderID}, desc...), nil
}

func (s *FolderService) ownedFolder(ctx context.Context, ownerID, folderID int64) (*domain.Folder, error) {
	f, err := s.folderRepo.GetByID(ctx, folderID)
	if err != nil {
		return nil, err
	}
	if f == nil || f.OwnerID != ownerID || f.DeletedAt != nil {
		return nil, errors.New("folder not found")
	}
	return f, nil
}
