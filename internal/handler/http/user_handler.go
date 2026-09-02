package http

import (
	"encoding/json"
	"net/http"

	"github.com/zuquanzhi/Chirp/backend/internal/domain"
	"github.com/zuquanzhi/Chirp/backend/internal/service"
)

type AuthHandler struct {
	svc *service.AuthService
}

func NewAuthHandler(svc *service.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

func (h *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.Email == "" || req.Password == "" {
		http.Error(w, "email and password required", http.StatusBadRequest)
		return
	}

	u, err := h.svc.Signup(r.Context(), req.Name, req.Email, req.Password)
	if err != nil {
		if err.Error() == "email already used" {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{"id": u.ID, "email": u.Email})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct{ Email, Password string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	token, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	json.NewEncoder(w).Encode(map[string]any{"token": token})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	u := GetUserFromContext(r.Context())
	json.NewEncoder(w).Encode(u)
}

func (h *AuthHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userCtx := GetUserFromContext(r.Context())
	if userCtx == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Name      string `json:"name"`
		School    string `json:"school"`
		StudentID string `json:"student_id"`
		Birthdate string `json:"birthdate"`
		Address   string `json:"address"`
		Gender    string `json:"gender"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	updated, err := h.svc.UpdateProfile(r.Context(), &domain.User{
		ID:        userCtx.ID,
		Name:      req.Name,
		School:    req.School,
		StudentID: req.StudentID,
		Birthdate: req.Birthdate,
		Address:   req.Address,
		Gender:    req.Gender,
	})
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(updated)
}
