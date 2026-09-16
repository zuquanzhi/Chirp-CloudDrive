package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

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

// DownloadFile GET /api/drive/files/{id}/download — add ?inline=1 for in-browser preview.
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

	disposition := "attachment"
	contentType := "application/octet-stream"
	if r.URL.Query().Get("inline") == "1" {
		disposition = "inline"
		if ct := mime.TypeByExtension(strings.ToLower(filepath.Ext(res.OriginalName))); ct != "" {
			contentType = ct
		} else {
			// Sniff the first bytes for unknown extensions.
			head := make([]byte, 512)
			n, _ := io.ReadFull(reader, head)
			contentType = http.DetectContentType(head[:n])
			reader = io.NopCloser(io.MultiReader(io.NewSectionReader(bytes.NewReader(head[:n]), 0, int64(n)), reader))
		}
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename=\"%s\"", disposition, url.QueryEscape(res.OriginalName)))
	w.Header().Set("Content-Type", contentType)
	io.Copy(w, reader)
}

// InstantUpload POST /api/drive/files/instant — {"name","hash","size","folder_id"}
// Creates the file without transferring content when the hash already exists (秒传).
func (h *DriveHandler) InstantUpload(w http.ResponseWriter, r *http.Request) {
	u := GetUserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		Name     string `json:"name"`
		Hash     string `json:"hash"`
		Size     int64  `json:"size"`
		FolderID *int64 `json:"folder_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Hash == "" {
		http.Error(w, "name and hash required", http.StatusBadRequest)
		return
	}
	res, err := h.resourceSvc.InstantUpload(r.Context(), u.ID, req.FolderID, req.Name, req.Hash, req.Size)
	if err != nil {
		writeDriveError(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(res)
}

// ListFileVersions GET /api/drive/files/{id}/versions
func (h *DriveHandler) ListFileVersions(w http.ResponseWriter, r *http.Request) {
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
	versions, err := h.resourceSvc.ListVersions(r.Context(), u.ID, fileID)
	if err != nil {
		writeDriveError(w, err)
		return
	}
	if versions == nil {
		versions = []domain.Resource{}
	}
	json.NewEncoder(w).Encode(map[string]any{"versions": versions})
}

// RestoreFileVersion POST /api/drive/files/{id}/versions/{vid}/restore
func (h *DriveHandler) RestoreFileVersion(w http.ResponseWriter, r *http.Request) {
	u := GetUserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	vars := mux.Vars(r)
	fileID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, "invalid file id", http.StatusBadRequest)
		return
	}
	versionID, err := strconv.ParseInt(vars["vid"], 10, 64)
	if err != nil {
		http.Error(w, "invalid version id", http.StatusBadRequest)
		return
	}
	res, err := h.resourceSvc.RestoreVersion(r.Context(), u.ID, fileID, versionID)
	if err != nil {
		writeDriveError(w, err)
		return
	}
	json.NewEncoder(w).Encode(res)
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
