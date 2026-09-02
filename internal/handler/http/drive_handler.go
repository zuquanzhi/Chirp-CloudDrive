package http

import (
	"net/http"
	"strconv"

	"github.com/zuquanzhi/Chirp/backend/internal/service"
)

// DriveHandler groups all /api/drive endpoints (folders, files, trash, quota).
type DriveHandler struct {
	folderSvc   *service.FolderService
	resourceSvc *service.ResourceService
	trashSvc    *service.TrashService
	authSvc     *service.AuthService
}

func NewDriveHandler(folderSvc *service.FolderService, resourceSvc *service.ResourceService, trashSvc *service.TrashService, authSvc *service.AuthService) *DriveHandler {
	return &DriveHandler{folderSvc: folderSvc, resourceSvc: resourceSvc, trashSvc: trashSvc, authSvc: authSvc}
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
		"file not found", "file not found in trash", "folder not found in trash":
		http.Error(w, err.Error(), http.StatusNotFound)
	case "folder name required", "file name required",
		"cannot move folder into itself", "cannot move folder into its own descendant":
		http.Error(w, err.Error(), http.StatusBadRequest)
	case "quota exceeded":
		http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
	default:
		http.Error(w, "server error", http.StatusInternalServerError)
	}
}
