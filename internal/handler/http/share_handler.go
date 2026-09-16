package http

import (
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
)

// ---- Authenticated share management ----

// CreateShare POST /api/drive/shares — {"resource_id", "password"?, "expire_days"?}
func (h *DriveHandler) CreateShare(w http.ResponseWriter, r *http.Request) {
	u := GetUserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		ResourceID int64  `json:"resource_id"`
		Password   string `json:"password"`
		ExpireDays int    `json:"expire_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.ResourceID == 0 {
		http.Error(w, "resource_id required", http.StatusBadRequest)
		return
	}
	share, err := h.shareSvc.Create(r.Context(), u.ID, req.ResourceID, req.Password, req.ExpireDays)
	if err != nil {
		writeDriveError(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(share)
}

// ListShares GET /api/drive/shares
func (h *DriveHandler) ListShares(w http.ResponseWriter, r *http.Request) {
	u := GetUserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	shares, err := h.shareSvc.List(r.Context(), u.ID)
	if err != nil {
		writeDriveError(w, err)
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"shares": shares})
}

// DeleteShare DELETE /api/drive/shares/{id}
func (h *DriveHandler) DeleteShare(w http.ResponseWriter, r *http.Request) {
	u := GetUserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		http.Error(w, "invalid share id", http.StatusBadRequest)
		return
	}
	if err := h.shareSvc.Cancel(r.Context(), u.ID, id); err != nil {
		writeDriveError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Public share access (no auth) ----

// GetShareInfo GET /api/shares/{token}
func (h *DriveHandler) GetShareInfo(w http.ResponseWriter, r *http.Request) {
	share, _, err := h.shareSvc.GetInfo(r.Context(), mux.Vars(r)["token"])
	if err != nil {
		writeDriveError(w, err)
		return
	}
	json.NewEncoder(w).Encode(share)
}

// DownloadShare GET /api/shares/{token}/download?password=
func (h *DriveHandler) DownloadShare(w http.ResponseWriter, r *http.Request) {
	res, reader, err := h.shareSvc.Download(r.Context(), mux.Vars(r)["token"], r.URL.Query().Get("password"))
	if err != nil {
		writeDriveError(w, err)
		return
	}
	defer reader.Close()

	contentType := "application/octet-stream"
	disposition := "attachment"
	if r.URL.Query().Get("inline") == "1" {
		disposition = "inline"
		if ct := mime.TypeByExtension(strings.ToLower(filepath.Ext(res.OriginalName))); ct != "" {
			contentType = ct
		}
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename=\"%s\"", disposition, url.QueryEscape(res.OriginalName)))
	w.Header().Set("Content-Type", contentType)
	io.Copy(w, reader)
}
