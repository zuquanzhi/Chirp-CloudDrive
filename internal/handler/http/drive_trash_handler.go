package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/zuquanzhi/Chirp/backend/internal/domain"
)

// ListTrash GET /api/drive/trash — deleted folders (top-level) and files
func (h *DriveHandler) ListTrash(w http.ResponseWriter, r *http.Request) {
	u := GetUserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	folders, err := h.trashSvc.ListTrashFolders(r.Context(), u.ID)
	if err != nil {
		writeDriveError(w, err)
		return
	}
	files, err := h.trashSvc.ListTrashFiles(r.Context(), u.ID)
	if err != nil {
		writeDriveError(w, err)
		return
	}
	if folders == nil {
		folders = []domain.Folder{}
	}
	if files == nil {
		files = []domain.Resource{}
	}
	json.NewEncoder(w).Encode(map[string]any{"folders": folders, "files": files})
}

// RestoreTrashItem POST /api/drive/trash/{kind}/{id}/restore — kind: folders|files
func (h *DriveHandler) RestoreTrashItem(w http.ResponseWriter, r *http.Request) {
	u := GetUserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	kind, id, ok := parseTrashPath(w, r)
	if !ok {
		return
	}

	switch kind {
	case "folders":
		err := h.trashSvc.RestoreFolder(r.Context(), u.ID, id)
		if err != nil {
			writeDriveError(w, err)
			return
		}
	case "files":
		err := h.trashSvc.RestoreFile(r.Context(), u.ID, id)
		if err != nil {
			writeDriveError(w, err)
			return
		}
	default:
		http.Error(w, "invalid kind", http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"message": "restored"})
}

// HardDeleteTrashItem DELETE /api/drive/trash/{kind}/{id} — permanent delete
func (h *DriveHandler) HardDeleteTrashItem(w http.ResponseWriter, r *http.Request) {
	u := GetUserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	kind, id, ok := parseTrashPath(w, r)
	if !ok {
		return
	}

	switch kind {
	case "folders":
		err := h.trashSvc.HardDeleteFolder(r.Context(), u.ID, id)
		if err != nil {
			writeDriveError(w, err)
			return
		}
	case "files":
		err := h.trashSvc.HardDeleteFile(r.Context(), u.ID, id)
		if err != nil {
			writeDriveError(w, err)
			return
		}
	default:
		http.Error(w, "invalid kind", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseTrashPath(w http.ResponseWriter, r *http.Request) (string, int64, bool) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return "", 0, false
	}
	return vars["kind"], id, true
}
