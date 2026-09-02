package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/zuquanzhi/Chirp/backend/internal/domain"
	"github.com/zuquanzhi/Chirp/backend/internal/service"
)

type DriveHandler struct {
	folderSvc   *service.FolderService
	resourceSvc *service.ResourceService
}

func NewDriveHandler(folderSvc *service.FolderService, resourceSvc *service.ResourceService) *DriveHandler {
	return &DriveHandler{folderSvc: folderSvc, resourceSvc: resourceSvc}
}

// GetQuota GET /api/drive/quota
func (h *DriveHandler) GetQuota(w http.ResponseWriter, r *http.Request) {
	u := GetUserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	quota, used, err := h.folderSvc.GetQuota(r.Context(), u.ID)
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

	var parentID *int64
	if s := r.URL.Query().Get("parent_id"); s != "" {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			http.Error(w, "invalid parent_id", http.StatusBadRequest)
			return
		}
		parentID = &id
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

// DeleteFolder DELETE /api/drive/folders/{id}
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

	if err := h.folderSvc.Delete(r.Context(), u.ID, folderID); err != nil {
		writeDriveError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Files ----

// ListItems GET /api/drive/items?folder_id=&q=
// Returns folders and files of one directory (nil folder_id = drive root).
func (h *DriveHandler) ListItems(w http.ResponseWriter, r *http.Request) {
	u := GetUserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	folderID, ok := parseOptionalID(w, r, "folder_id")
	if !ok {
		return
	}
	search := r.URL.Query().Get("q")

	folders, err := h.folderSvc.List(r.Context(), u.ID, folderID)
	if err != nil {
		writeDriveError(w, err)
		return
	}
	files, err := h.resourceSvc.ListDriveFiles(r.Context(), u.ID, folderID, search)
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

// UploadFile POST /api/drive/files (multipart: file + optional folder_id)
func (h *DriveHandler) UploadFile(w http.ResponseWriter, r *http.Request) {
	u := GetUserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	var folderID *int64
	if s := r.FormValue("folder_id"); s != "" {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			http.Error(w, "invalid folder_id", http.StatusBadRequest)
			return
		}
		folderID = &id
	}

	f, fh, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file required", http.StatusBadRequest)
		return
	}
	defer f.Close()

	res, err := h.resourceSvc.UploadToFolder(r.Context(), u.ID, folderID, f, fh)
	if err != nil {
		writeDriveError(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(res)
}

// DownloadFile GET /api/drive/files/{id}/download
func (h *DriveHandler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	u := GetUserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	fileID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		http.Error(w, "invalid file id", http.StatusBadRequest)
		return
	}

	res, reader, err := h.resourceSvc.DownloadDriveFile(r.Context(), u.ID, fileID)
	if err != nil {
		writeDriveError(w, err)
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", res.OriginalName))
	w.Header().Set("Content-Type", "application/octet-stream")
	io.Copy(w, reader)
}

// UpdateFile PATCH /api/drive/files/{id} — {"name": "..."} rename, {"folder_id": 3|null} move
func (h *DriveHandler) UpdateFile(w http.ResponseWriter, r *http.Request) {
	u := GetUserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	fileID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		http.Error(w, "invalid file id", http.StatusBadRequest)
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

	if rawName, ok := raw["name"]; ok {
		var name string
		if err := json.Unmarshal(rawName, &name); err != nil {
			http.Error(w, "invalid name", http.StatusBadRequest)
			return
		}
		if _, err := h.resourceSvc.RenameFile(r.Context(), u.ID, fileID, name); err != nil {
			writeDriveError(w, err)
			return
		}
	}

	if rawFolder, ok := raw["folder_id"]; ok {
		var folderID *int64
		if string(rawFolder) != "null" {
			var fid int64
			if err := json.Unmarshal(rawFolder, &fid); err != nil {
				http.Error(w, "invalid folder_id", http.StatusBadRequest)
				return
			}
			folderID = &fid
		}
		if _, err := h.resourceSvc.MoveFile(r.Context(), u.ID, fileID, folderID); err != nil {
			writeDriveError(w, err)
			return
		}
	}

	json.NewEncoder(w).Encode(map[string]string{"message": "updated"})
}

// DeleteFile DELETE /api/drive/files/{id} — move to trash
func (h *DriveHandler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	u := GetUserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	fileID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		http.Error(w, "invalid file id", http.StatusBadRequest)
		return
	}
	if err := h.resourceSvc.SoftDeleteFile(r.Context(), u.ID, fileID); err != nil {
		writeDriveError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Trash ----

// ListTrash GET /api/drive/trash — deleted folders (top-level) and files
func (h *DriveHandler) ListTrash(w http.ResponseWriter, r *http.Request) {
	u := GetUserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	folders, err := h.folderSvc.ListTrash(r.Context(), u.ID)
	if err != nil {
		writeDriveError(w, err)
		return
	}
	files, err := h.resourceSvc.ListTrashFiles(r.Context(), u.ID)
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
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	switch vars["kind"] {
	case "folders":
		err = h.folderSvc.RestoreFolder(r.Context(), u.ID, id)
	case "files":
		err = h.resourceSvc.RestoreFile(r.Context(), u.ID, id)
	default:
		http.Error(w, "invalid kind", http.StatusBadRequest)
		return
	}
	if err != nil {
		writeDriveError(w, err)
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
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	switch vars["kind"] {
	case "folders":
		err = h.folderSvc.HardDeleteFolder(r.Context(), u.ID, id)
	case "files":
		err = h.resourceSvc.HardDeleteFile(r.Context(), u.ID, id)
	default:
		http.Error(w, "invalid kind", http.StatusBadRequest)
		return
	}
	if err != nil {
		writeDriveError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
