package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Baldyozh/log-processor/internal/domain/entities"
)

// AuthRepository handles authentication and authorization operations
type AuthRepository struct {
	db *sql.DB
}

// NewAuthRepository creates a new auth repository
func NewAuthRepository(db *sql.DB) *AuthRepository {
	return &AuthRepository{db: db}
}

// CreateUser creates a new user with roles
func (r *AuthRepository) CreateUser(ctx context.Context, req entities.CreateUserRequest, passwordHash string) (*entities.User, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert user
	var userID int
	query := `INSERT INTO users (username, email, password_hash) VALUES ($1, $2, $3) RETURNING id`
	err = tx.QueryRowContext(ctx, query, req.Username, req.Email, passwordHash).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Assign roles
	if len(req.RoleIDs) > 0 {
		for _, roleID := range req.RoleIDs {
			_, err = tx.ExecContext(ctx, `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, userID, roleID)
			if err != nil {
				return nil, fmt.Errorf("failed to assign role %d: %w", roleID, err)
			}
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return r.GetUserByID(ctx, userID)
}

// GetUserByUsername retrieves a user by username
func (r *AuthRepository) GetUserByUsername(ctx context.Context, username string) (*entities.User, error) {
	query := `SELECT id, username, email, password_hash, created_at, updated_at FROM users WHERE username = $1`
	user := &entities.User{}
	err := r.db.QueryRowContext(ctx, query, username).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Load roles
	user.Roles, err = r.getUserRoles(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// GetUserByID retrieves a user by ID
func (r *AuthRepository) GetUserByID(ctx context.Context, userID int) (*entities.User, error) {
	query := `SELECT id, username, email, password_hash, created_at, updated_at FROM users WHERE id = $1`
	user := &entities.User{}
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Load roles
	user.Roles, err = r.getUserRoles(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// getUserRoles retrieves roles for a user
func (r *AuthRepository) getUserRoles(ctx context.Context, userID int) ([]entities.Role, error) {
	query := `
		SELECT r.id, r.name, r.description, r.created_at
		FROM roles r
		JOIN user_roles ur ON r.id = ur.role_id
		WHERE ur.user_id = $1
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user roles: %w", err)
	}
	defer rows.Close()

	var roles []entities.Role
	for rows.Next() {
		var role entities.Role
		err := rows.Scan(&role.ID, &role.Name, &role.Description, &role.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan role: %w", err)
		}

		// Load permissions for each role
		role.Permissions, err = r.getRolePermissions(ctx, role.ID)
		if err != nil {
			return nil, err
		}

		roles = append(roles, role)
	}

	return roles, nil
}

// getRolePermissions retrieves permissions for a role
func (r *AuthRepository) getRolePermissions(ctx context.Context, roleID int) ([]entities.Permission, error) {
	query := `
		SELECT p.id, p.name, p.description, p.created_at
		FROM permissions p
		JOIN role_permissions rp ON p.id = rp.permission_id
		WHERE rp.role_id = $1
	`
	rows, err := r.db.QueryContext(ctx, query, roleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get role permissions: %w", err)
	}
	defer rows.Close()

	var permissions []entities.Permission
	for rows.Next() {
		var perm entities.Permission
		err := rows.Scan(&perm.ID, &perm.Name, &perm.Description, &perm.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan permission: %w", err)
		}
		permissions = append(permissions, perm)
	}

	return permissions, nil
}

// HasPermission checks if a user has a specific permission
func (r *AuthRepository) HasPermission(ctx context.Context, userID int, permissionName string) (bool, error) {
	query := `
		SELECT COUNT(*) > 0
		FROM permissions p
		JOIN role_permissions rp ON p.id = rp.permission_id
		JOIN user_roles ur ON rp.role_id = ur.role_id
		WHERE ur.user_id = $1 AND p.name = $2
	`
	var hasPermission bool
	err := r.db.QueryRowContext(ctx, query, userID, permissionName).Scan(&hasPermission)
	if err != nil {
		return false, fmt.Errorf("failed to check permission: %w", err)
	}
	return hasPermission, nil
}

// LogDecryptionRequest logs a decryption request
func (r *AuthRepository) LogDecryptionRequest(ctx context.Context, userID int, logID string, reason string) error {
	query := `INSERT INTO decryption_requests (user_id, log_id, reason) VALUES ($1, $2, $3)`
	_, err := r.db.ExecContext(ctx, query, userID, logID, reason)
	if err != nil {
		return fmt.Errorf("failed to log decryption request: %w", err)
	}
	return nil
}

// GetDecryptionRequests retrieves decryption requests for a user
func (r *AuthRepository) GetDecryptionRequests(ctx context.Context, userID int, limit int) ([]entities.DecryptionRequest, error) {
	query := `
		SELECT id, user_id, log_id, requested_at, granted, reason
		FROM decryption_requests
		WHERE user_id = $1
		ORDER BY requested_at DESC
		LIMIT $2
	`
	rows, err := r.db.QueryContext(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get decryption requests: %w", err)
	}
	defer rows.Close()

	var requests []entities.DecryptionRequest
	for rows.Next() {
		var req entities.DecryptionRequest
		err := rows.Scan(&req.ID, &req.UserID, &req.LogID, &req.RequestedAt, &req.Granted, &req.Reason)
		if err != nil {
			return nil, fmt.Errorf("failed to scan decryption request: %w", err)
		}
		requests = append(requests, req)
	}

	return requests, nil
}

// GetAllRoles retrieves all roles
func (r *AuthRepository) GetAllRoles(ctx context.Context) ([]entities.Role, error) {
	query := `SELECT id, name, description, created_at FROM roles ORDER BY name`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get roles: %w", err)
	}
	defer rows.Close()

	var roles []entities.Role
	for rows.Next() {
		var role entities.Role
		err := rows.Scan(&role.ID, &role.Name, &role.Description, &role.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan role: %w", err)
		}
		roles = append(roles, role)
	}

	return roles, nil
}

// AssignRole assigns a role to a user
func (r *AuthRepository) AssignRole(ctx context.Context, userID int, roleID int) error {
	query := `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	_, err := r.db.ExecContext(ctx, query, userID, roleID)
	if err != nil {
		return fmt.Errorf("failed to assign role: %w", err)
	}
	return nil
}

// RemoveRole removes a role from a user
func (r *AuthRepository) RemoveRole(ctx context.Context, userID int, roleID int) error {
	query := `DELETE FROM user_roles WHERE user_id = $1 AND role_id = $2`
	_, err := r.db.ExecContext(ctx, query, userID, roleID)
	if err != nil {
		return fmt.Errorf("failed to remove role: %w", err)
	}
	return nil
}
