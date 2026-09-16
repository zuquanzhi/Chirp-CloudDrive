package http

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"time"
)

// BatchDelete POST /api/drive/batch/delete — {"file_ids":[], "folder_ids":[]}
// Moves every item to trash; per-item failures are reported, not fatal.
func (h *DriveHandler) BatchDelete(w http.ResponseWriter, r *http.Request) {
	u := GetUserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		FileIDs   []int64 `json:"file_ids"`
		FolderIDs []int64 `json:"folder_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	failed := map[string]string{}
	for _, id := range req.FileIDs {
		if err := h.resourceSvc.SoftDeleteFile(r.Context(), u.ID, id); err != nil {
			failed[fmt.Sprintf("file:%d", id)] = err.Error()
		}
	}
	for _, id := range req.FolderIDs {
		if err := h.trashSvc.DeleteFolder(r.Context(), u.ID, id); err != nil {
			failed[fmt.Sprintf("folder:%d", id)] = err.Error()
		}
	}
	json.NewEncoder(w).Encode(map[string]any{
		"deleted": len(req.FileIDs) + len(req.FolderIDs) - len(failed),
		"failed":  failed,
	})
}

// BatchMove POST /api/drive/batch/move — {"file_ids":[], "folder_id": 3|null}
func (h *DriveHandler) BatchMove(w http.ResponseWriter, r *http.Request) {
	u := GetUserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		FileIDs  []int64 `json:"file_ids"`
		FolderID *int64  `json:"folder_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	failed := map[string]string{}
	for _, id := range req.FileIDs {
		if _, err := h.resourceSvc.MoveFile(r.Context(), u.ID, id, req.FolderID); err != nil {
			failed[fmt.Sprintf("file:%d", id)] = err.Error()
		}
	}
	json.NewEncoder(w).Encode(map[string]any{
		"moved":  len(req.FileIDs) - len(failed),
		"failed": failed,
	})
}

// BatchDownload POST /api/drive/batch/download — {"file_ids":[]}
// Streams a zip archive of all owned files.
func (h *DriveHandler) BatchDownload(w http.ResponseWriter, r *http.Request) {
	u := GetUserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		FileIDs []int64 `json:"file_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if len(req.FileIDs) == 0 {
		http.Error(w, "no files selected", http.StatusBadRequest)
		return
	}

	zipName := fmt.Sprintf("chirp-%s.zip", time.Now().Format("20060102-150405"))
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", url.QueryEscape(zipName)))

	zw := zip.NewWriter(w)
	usedNames := map[string]int{}
	for _, id := range req.FileIDs {
		res, reader, err := h.resourceSvc.DownloadDriveFile(r.Context(), u.ID, id)
		if err != nil {
			continue // skip files that cannot be read
		}
		name := uniqueZipName(usedNames, res.OriginalName)
		entry, err := zw.Create(name)
		if err != nil {
			reader.Close()
			continue
		}
		copyEntry(entry, reader)
		reader.Close()
	}
	zw.Close()
}

func uniqueZipName(used map[string]int, name string) string {
	if used[name] == 0 {
		used[name] = 1
		return name
	}
	used[name]++
	ext := path.Ext(name)
	base := name[:len(name)-len(ext)]
	return fmt.Sprintf("%s (%d)%s", base, used[name]-1, ext)
}

func copyEntry(dst interface{ Write([]byte) (int, error) }, src interface{ Read([]byte) (int, error) }) {
	buf := make([]byte, 64*1024)
	for {
		n, rerr := src.Read(buf)
		if n > 0 {
			dst.Write(buf[:n])
		}
		if rerr != nil {
			return
		}
	}
}
