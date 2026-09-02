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

type ResourceHandler struct {
	svc *service.ResourceService
}

func NewResourceHandler(svc *service.ResourceService) *ResourceHandler {
	return &ResourceHandler{svc: svc}
}

func (h *ResourceHandler) Upload(w http.ResponseWriter, r *http.Request) {
	// Optional User
	u := GetUserFromContext(r.Context())
	var ownerID *int64
	if u != nil {
		ownerID = &u.ID
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	// Handle multiple files if needed, but for now let's stick to single file per request or iterate
	// The requirement says "File/File Group Upload".
	// Let's support multiple files with key "files"
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		// Fallback to single "file"
		f, fh, err := r.FormFile("file")
		if err == nil {
			defer f.Close()
			title := r.FormValue("title")
			desc := r.FormValue("description")
			subject := r.FormValue("subject")
			resourceType := r.FormValue("type")
			res, err := h.svc.Upload(r.Context(), ownerID, title, desc, subject, resourceType, f, fh)
			if err != nil {
				fmt.Printf("[Error] Upload failed: %v\n", err)                              // Add logging
				http.Error(w, "server error: "+err.Error(), http.StatusInternalServerError) // Return error details for debugging
				return
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(res)
			return
		}
		http.Error(w, "file required", http.StatusBadRequest)
		return
	}

	var results []*domain.Resource
	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			continue
		}
		defer f.Close()
		// For group upload, title might be shared or filename used
		title := r.FormValue("title")
		if title == "" {
			title = fh.Filename
		}
		desc := r.FormValue("description")
		subject := r.FormValue("subject")
		resourceType := r.FormValue("type")

		res, err := h.svc.Upload(r.Context(), ownerID, title, desc, subject, resourceType, f, fh)
		if err == nil {
			results = append(results, res)
		}
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(results)
}

func (h *ResourceHandler) List(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("q")
	// Public list only shows APPROVED resources? Or all for MVP?
	// Let's show all for now or filter by status if needed.
	// Requirement: "Search".
	list, err := h.svc.List(r.Context(), "", search)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(list)
}

func (h *ResourceHandler) Download(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}

	res, reader, err := h.svc.GetFileContent(r.Context(), id)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if res == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", res.OriginalName))
	w.Header().Set("Content-Type", "application/octet-stream")
	// Since we have a reader, we can't use ServeContent easily for range requests without Seek,
	// but io.Copy is fine for simple downloads.
	io.Copy(w, reader)
}

// Admin: Review
func (h *ResourceHandler) Review(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	status := domain.ResourceStatus(req.Status)
	if status != domain.ResourceStatusApproved && status != domain.ResourceStatusRejected {
		http.Error(w, "invalid status", http.StatusBadRequest)
		return
	}

	if err := h.svc.Review(r.Context(), id, status); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// Admin: Check Duplicate
func (h *ResourceHandler) CheckDuplicate(w http.ResponseWriter, r *http.Request) {
	hash := r.URL.Query().Get("hash")
	if hash == "" {
		http.Error(w, "hash required", http.StatusBadRequest)
		return
	}
	list, err := h.svc.CheckDuplicate(r.Context(), hash)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(list)
}
