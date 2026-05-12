package entities

import "time"

// User represents a system user
type User struct {
	ID           int       `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // Never expose in JSON
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Roles        []Role    `json:"roles,omitempty"`
}

// Role represents a user role with permissions
type Role struct {
	ID          int          `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	CreatedAt   time.Time    `json:"created_at"`
	Permissions []Permission `json:"permissions,omitempty"`
}

// Permission represents a specific permission
type Permission struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// DecryptionRequest represents a request to decrypt sensitive log data
type DecryptionRequest struct {
	ID          int       `json:"id"`
	UserID      int       `json:"user_id"`
	LogID       string    `json:"log_id"`
	RequestedAt time.Time `json:"requested_at"`
	Granted     bool      `json:"granted"`
	Reason      string    `json:"reason,omitempty"`
}

// CreateUserRequest represents a request to create a new user
type CreateUserRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	RoleIDs  []int  `json:"role_ids"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse represents a successful login response
type LoginResponse struct {
	Token string `json:"token"`
	User  User  `json:"user"`
}
