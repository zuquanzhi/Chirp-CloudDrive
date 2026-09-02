package service

import (
	"context"
	"errors"

	"github.com/zuquanzhi/Chirp/backend/internal/domain"
)

// FolderService handles pure folder CRUD (no trash orchestration — see TrashService).
type FolderService struct {
	folderRepo domain.FolderRepository
}

func NewFolderService(folderRepo domain.FolderRepository) *FolderService {
	return &FolderService{folderRepo: folderRepo}
}

// List returns the non-deleted child folders of parentID (nil = drive root).
func (s *FolderService) List(ctx context.Context, ownerID int64, parentID *int64) ([]domain.Folder, error) {
	if parentID != nil {
		if _, err := s.OwnedFolder(ctx, ownerID, *parentID); err != nil {
			return nil, err
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
		if _, err := s.OwnedFolder(ctx, ownerID, *parentID); err != nil {
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
	f, err := s.OwnedFolder(ctx, ownerID, folderID)
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
	f, err := s.OwnedFolder(ctx, ownerID, folderID)
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

// OwnedFolder returns a folder only if it belongs to the user and is not deleted.
func (s *FolderService) OwnedFolder(ctx context.Context, ownerID, folderID int64) (*domain.Folder, error) {
	f, err := s.folderRepo.GetByID(ctx, folderID)
	if err != nil {
		return nil, err
	}
	if f == nil || f.OwnerID != ownerID || f.DeletedAt != nil {
		return nil, errors.New("folder not found")
	}
	return f, nil
}
