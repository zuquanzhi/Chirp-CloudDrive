package http

import (
	"net/http"
	"strconv"

	"github.com/zuquanzhi/Chirp/backend/internal/service"
)

// DriveHandler groups all /api/drive endpoints (folders, files, trash, quota,
// shares, chunked uploads, batch operations, activity log).
type DriveHandler struct {
	folderSvc   *service.FolderService
	resourceSvc *service.ResourceService
	trashSvc    *service.TrashService
	authSvc     *service.AuthService
	shareSvc    *service.ShareService
	uploadSvc   *service.UploadService
	activity    *service.ActivityRecorder
}

func NewDriveHandler(
	folderSvc *service.FolderService,
	resourceSvc *service.ResourceService,
	trashSvc *service.TrashService,
	authSvc *service.AuthService,
	shareSvc *service.ShareService,
	uploadSvc *service.UploadService,
	activity *service.ActivityRecorder,
) *DriveHandler {
	return &DriveHandler{
		folderSvc:   folderSvc,
		resourceSvc: resourceSvc,
		trashSvc:    trashSvc,
		authSvc:     authSvc,
		shareSvc:    shareSvc,
		uploadSvc:   uploadSvc,
		activity:    activity,
	}
}

func parseOptionalID(w http.ResponseWriter, r *http.Request, key string) (*int64, bool) {
	s := r.URL.Query().Get(key)
	if s == "" {
		return nil, true
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		http.Error(w, "invalid "+key, http.StatusBadRequest)
		return nil, false
	}
	return &id, true
}

func writeDriveError(w http.ResponseWriter, err error) {
	switch err.Error() {
	case "folder not found", "parent folder not found", "target folder not found",
		"file not found", "file not found in trash", "folder not found in trash",
		"share not found", "content not found for hash", "upload session not found":
		http.Error(w, err.Error(), http.StatusNotFound)
	case "folder name required", "file name required", "file hash required",
		"cannot move folder into itself", "cannot move folder into its own descendant",
		"filename and size required", "chunk index out of range", "file hash mismatch",
		"file has no version history", "version not found":
		http.Error(w, err.Error(), http.StatusBadRequest)
	case "share expired":
		http.Error(w, err.Error(), http.StatusGone)
	case "wrong extraction code":
		http.Error(w, err.Error(), http.StatusForbidden)
	case "shared file no longer exists":
		http.Error(w, err.Error(), http.StatusNotFound)
	case "quota exceeded":
		http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
	default:
		if msg := err.Error(); len(msg) >= 15 && msg[:15] == "missing chunks:" {
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
	}
}
