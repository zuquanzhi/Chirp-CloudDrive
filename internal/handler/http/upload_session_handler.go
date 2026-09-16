package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

// InitChunkedUpload POST /api/drive/uploads/init
// {"filename","size","folder_id","file_hash","chunk_size"} → instant hit or a session.
func (h *DriveHandler) InitChunkedUpload(w http.ResponseWriter, r *http.Request) {
	u := GetUserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		Filename  string `json:"filename"`
		Size      int64  `json:"size"`
		FolderID  *int64 `json:"folder_id"`
		FileHash  string `json:"file_hash"`
		ChunkSize int64  `json:"chunk_size"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	result, err := h.uploadSvc.Init(r.Context(), u.ID, req.FolderID, req.Filename, req.Size, req.FileHash, req.ChunkSize)
	if err != nil {
		writeDriveError(w, err)
		return
	}
	json.NewEncoder(w).Encode(result)
}

// GetChunkedUpload GET /api/drive/uploads/{id} — session state with uploaded chunk indexes.
func (h *DriveHandler) GetChunkedUpload(w http.ResponseWriter, r *http.Request) {
	u := GetUserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	session, err := h.uploadSvc.Get(r.Context(), u.ID, mux.Vars(r)["id"])
	if err != nil {
		writeDriveError(w, err)
		return
	}
	json.NewEncoder(w).Encode(session)
}

// UploadChunk PUT /api/drive/uploads/{id}/chunks/{index} — raw chunk body.
func (h *DriveHandler) UploadChunk(w http.ResponseWriter, r *http.Request) {
	u := GetUserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	index, err := strconv.Atoi(mux.Vars(r)["index"])
	if err != nil {
		http.Error(w, "invalid chunk index", http.StatusBadRequest)
		return
	}
	if err := h.uploadSvc.SaveChunk(r.Context(), u.ID, mux.Vars(r)["id"], index, r.Body); err != nil {
		writeDriveError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CompleteChunkedUpload POST /api/drive/uploads/{id}/complete
func (h *DriveHandler) CompleteChunkedUpload(w http.ResponseWriter, r *http.Request) {
	u := GetUserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	res, err := h.uploadSvc.Complete(r.Context(), u.ID, mux.Vars(r)["id"])
	if err != nil {
		writeDriveError(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(res)
}

// AbortChunkedUpload DELETE /api/drive/uploads/{id}
func (h *DriveHandler) AbortChunkedUpload(w http.ResponseWriter, r *http.Request) {
	u := GetUserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err := h.uploadSvc.Abort(r.Context(), u.ID, mux.Vars(r)["id"]); err != nil {
		writeDriveError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListActivities GET /api/activities?limit=
func (h *DriveHandler) ListActivities(w http.ResponseWriter, r *http.Request) {
	u := GetUserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	limit := 50
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			limit = n
		}
	}
	list, err := h.activity.List(r.Context(), u.ID, limit)
	if err != nil {
		writeDriveError(w, err)
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"activities": list})
}
