package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/zuquanzhi/Chirp/backend/internal/domain"
)

// GetQuota GET /api/drive/quota
func (h *DriveHandler) GetQuota(w http.ResponseWriter, r *http.Request) {
	u := GetUserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	quota, used, err := h.authSvc.GetQuota(r.Context(), u.ID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"quota": quota, "used": used})
}

// ListFolders GET /api/drive/folders?parent_id=
func (h *DriveHandler) ListFolders(w http.ResponseWriter, r *http.Request) {
	u := GetUserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	parentID, ok := parseOptionalID(w, r, "parent_id")
	if !ok {
		return
	}

	folders, err := h.folderSvc.List(r.Context(), u.ID, parentID)
	if err != nil {
		writeDriveError(w, err)
		return
	}
	if folders == nil {
		folders = []domain.Folder{}
	}
	json.NewEncoder(w).Encode(folders)
}

// CreateFolder POST /api/drive/folders
func (h *DriveHandler) CreateFolder(w http.ResponseWriter, r *http.Request) {
	u := GetUserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Name     string `json:"name"`
		ParentID *int64 `json:"parent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	f, err := h.folderSvc.Create(r.Context(), u.ID, req.Name, req.ParentID)
	if err != nil {
		writeDriveError(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(f)
}

// UpdateFolder PATCH /api/drive/folders/{id}
// Body: {"name": "new name"} to rename, {"parent_id": 3} (or null for root) to move. Both may be combined.
func (h *DriveHandler) UpdateFolder(w http.ResponseWriter, r *http.Request) {
	u := GetUserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	folderID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		http.Error(w, "invalid folder id", http.StatusBadRequest)
		return
	}

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if len(raw) == 0 {
		http.Error(w, "nothing to update", http.StatusBadRequest)
		return
	}

	// Rename
	if rawName, ok := raw["name"]; ok {
		var name string
		if err := json.Unmarshal(rawName, &name); err != nil {
			http.Error(w, "invalid name", http.StatusBadRequest)
			return
		}
		if _, err := h.folderSvc.Rename(r.Context(), u.ID, folderID, name); err != nil {
			writeDriveError(w, err)
			return
		}
	}

	// Move (key present; null = move to root)
	if rawParent, ok := raw["parent_id"]; ok {
		var parentID *int64
		if string(rawParent) != "null" {
			var pid int64
			if err := json.Unmarshal(rawParent, &pid); err != nil {
				http.Error(w, "invalid parent_id", http.StatusBadRequest)
				return
			}
			parentID = &pid
		}
		if _, err := h.folderSvc.Move(r.Context(), u.ID, folderID, parentID); err != nil {
			writeDriveError(w, err)
			return
		}
	}

	json.NewEncoder(w).Encode(map[string]string{"message": "updated"})
}

// DeleteFolder DELETE /api/drive/folders/{id} — soft delete (trash)
func (h *DriveHandler) DeleteFolder(w http.ResponseWriter, r *http.Request) {
	u := GetUserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	folderID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		http.Error(w, "invalid folder id", http.StatusBadRequest)
		return
	}

	if err := h.trashSvc.DeleteFolder(r.Context(), u.ID, folderID); err != nil {
		writeDriveError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
