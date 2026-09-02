package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/zuquanzhi/Chirp/backend/internal/domain"
)

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
