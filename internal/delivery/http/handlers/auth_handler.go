package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/Baldyozh/log-processor/internal/domain/entities"
	"github.com/Baldyozh/log-processor/internal/infrastructure/auth"
	"github.com/Baldyozh/log-processor/internal/infrastructure/postgres"
	"golang.org/x/crypto/bcrypt"
)

// AuthHandler handles HTTP requests for authentication
type AuthHandler struct {
	authRepo   *postgres.AuthRepository
	jwtService *auth.JWTService
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(authRepo *postgres.AuthRepository, jwtService *auth.JWTService) *AuthHandler {
	return &AuthHandler{
		authRepo:   authRepo,
		jwtService: jwtService,
	}
}

// Login handles user login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req entities.LoginRequest
	if err := jsonDecode(r, &req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get user by username
	user, err := h.authRepo.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Generate JWT token
	token, err := h.jwtService.GenerateToken(user.ID, user.Username)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Clear password hash from response
	user.PasswordHash = ""

	response := entities.LoginResponse{
		Token: token,
		User:  *user,
	}

	respondJSON(w, http.StatusOK, response)
}

// CreateUser handles user creation (admin only)
func (h *AuthHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req entities.CreateUserRequest
	if err := jsonDecode(r, &req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	// Create user
	user, err := h.authRepo.CreateUser(r.Context(), req, string(hashedPassword))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Clear password hash from response
	user.PasswordHash = ""

	respondJSON(w, http.StatusCreated, user)
}

// GetCurrentUser returns the current authenticated user
func (h *AuthHandler) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserIDFromContext(r)
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	user, err := h.authRepo.GetUserByID(r.Context(), userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Clear password hash from response
	user.PasswordHash = ""

	respondJSON(w, http.StatusOK, user)
}

// GetAllRoles returns all available roles
func (h *AuthHandler) GetAllRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := h.authRepo.GetAllRoles(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, roles)
}

// jsonDecode is a helper function to decode JSON from request body
func jsonDecode(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}
