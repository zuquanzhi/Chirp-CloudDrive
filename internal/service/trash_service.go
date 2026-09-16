package service

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/zuquanzhi/Chirp/backend/internal/domain"
)

// TrashRetention is how long trashed items are kept before automatic cleanup.
const TrashRetention = 30 * 24 * time.Hour

// TrashService orchestrates soft-delete / restore / permanent-delete across
// folders, files, physical storage and quota accounting.
type TrashService struct {
	folderRepo   domain.FolderRepository
	resourceRepo domain.ResourceRepository
	userRepo     domain.UserRepository
	storage      FileStorage
	activity     *ActivityRecorder
}

func NewTrashService(folderRepo domain.FolderRepository, resourceRepo domain.ResourceRepository, userRepo domain.UserRepository, storage FileStorage) *TrashService {
	return &TrashService{folderRepo: folderRepo, resourceRepo: resourceRepo, userRepo: userRepo, storage: storage}
}

// SetActivityRecorder attaches the activity log recorder (optional).
func (s *TrashService) SetActivityRecorder(rec *ActivityRecorder) {
	s.activity = rec
}

// ---- Soft delete (move to trash) ----

// DeleteFolder soft-deletes a folder, its descendant folders, and all files inside the subtree.
func (s *TrashService) DeleteFolder(ctx context.Context, ownerID, folderID int64) error {
	f, err := s.folderRepo.GetByID(ctx, folderID)
	if err != nil {
		return err
	}
	if f == nil || f.OwnerID != ownerID || f.DeletedAt != nil {
		return errors.New("folder not found")
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
	s.activity.Log(ctx, ownerID, domain.ActivityDelete, "folder", folderID, f.Name, "移入回收站")
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
	s.activity.Log(ctx, ownerID, domain.ActivityRestore, "folder", folderID, f.Name, "从回收站还原")
	return nil
}

// RestoreFile clears the trash mark of a file and all its versions;
// files whose folder is still deleted go back to root.
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
	if res.VersionGroup != "" {
		if err := s.resourceRepo.RestoreByGroup(ctx, res.VersionGroup); err != nil {
			return err
		}
	}
	if err := s.resourceRepo.Restore(ctx, fileID, folderID); err != nil {
		return err
	}
	s.activity.Log(ctx, ownerID, domain.ActivityRestore, "file", fileID, res.OriginalName, "从回收站还原")
	return nil
}

// ---- Permanent delete ----

// deletePhysical removes the stored object only when no other row references it (dedup refcount).
func (s *TrashService) deletePhysical(ctx context.Context, filename string) error {
	refs, err := s.resourceRepo.CountByFilename(ctx, filename)
	if err != nil {
		return err
	}
	if refs > 0 {
		return nil // still referenced by another logical file / version
	}
	return s.storage.Delete(ctx, filename)
}

// deleteFileRows permanently removes file rows and their unreferenced physical objects.
// Returns the total bytes freed (sum of row sizes, matching quota accounting).
func (s *TrashService) deleteFileRows(ctx context.Context, files []domain.Resource) (int64, error) {
	var freed int64
	filenames := make(map[string]bool)
	for _, f := range files {
		freed += f.Size
		filenames[f.Filename] = true
	}
	for _, f := range files {
		if err := s.resourceRepo.HardDelete(ctx, f.ID); err != nil {
			return freed, err
		}
	}
	for fn := range filenames {
		if err := s.deletePhysical(ctx, fn); err != nil {
			return freed, err
		}
	}
	return freed, nil
}

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
		n, err := s.deleteFileRows(ctx, files)
		freed += n
		if err != nil {
			return err
		}
		if err := s.folderRepo.HardDelete(ctx, id); err != nil {
			return err
		}
	}
	if freed > 0 {
		if err := s.userRepo.AddUsed(ctx, ownerID, -freed); err != nil {
			return err
		}
	}
	s.activity.Log(ctx, ownerID, domain.ActivityHardDelete, "folder", folderID, f.Name, "彻底删除")
	return nil
}

// HardDeleteFile permanently removes a file and all its versions: physical objects, rows, quota.
func (s *TrashService) HardDeleteFile(ctx context.Context, ownerID, fileID int64) error {
	res, err := s.trashedFile(ctx, ownerID, fileID)
	if err != nil {
		return err
	}

	files := []domain.Resource{*res}
	if res.VersionGroup != "" {
		versions, err := s.resourceRepo.ListVersions(ctx, res.VersionGroup)
		if err != nil {
			return err
		}
		if len(versions) > 0 {
			files = versions
		}
	}

	freed, err := s.deleteFileRows(ctx, files)
	if err != nil {
		return err
	}
	if err := s.userRepo.AddUsed(ctx, ownerID, -freed); err != nil {
		return err
	}
	s.activity.Log(ctx, ownerID, domain.ActivityHardDelete, "file", fileID, res.OriginalName, "彻底删除")
	return nil
}

// ---- Automatic cleanup ----

// CleanupExpired permanently deletes files and folders whose deleted_at is older than retention.
// Returns the number of file rows and folder rows removed.
func (s *TrashService) CleanupExpired(ctx context.Context, retention time.Duration) (int, int, error) {
	cutoff := time.Now().Add(-retention)

	files, err := s.resourceRepo.ListDeletedBefore(ctx, cutoff)
	if err != nil {
		return 0, 0, err
	}
	filesRemoved := 0
	for _, f := range files {
		freed, err := s.deleteFileRows(ctx, []domain.Resource{f})
		if err != nil {
			log.Printf("trash cleanup: delete file %d failed: %v", f.ID, err)
			continue
		}
		if f.OwnerID != nil && freed > 0 {
			if err := s.userRepo.AddUsed(ctx, *f.OwnerID, -freed); err != nil {
				log.Printf("trash cleanup: quota adjust failed: %v", err)
			}
		}
		filesRemoved++
	}

	folders, err := s.folderRepo.ListDeletedBefore(ctx, cutoff)
	if err != nil {
		return filesRemoved, 0, err
	}
	foldersRemoved := 0
	for _, fo := range folders {
		// Files inside were handled above; just remove the folder row.
		if err := s.folderRepo.HardDelete(ctx, fo.ID); err != nil {
			log.Printf("trash cleanup: delete folder %d failed: %v", fo.ID, err)
			continue
		}
		foldersRemoved++
	}
	return filesRemoved, foldersRemoved, nil
}

// ---- helpers ----

func (s *TrashService) subtreeIDs(ctx context.Context, folderID int64) ([]int64, error) {
	desc, err := s.folderRepo.ListDescendantIDs(ctx, folderID)
	if err != nil {
		return nil, err
	}
	return append([]int64{folderID}, desc...), nil
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
