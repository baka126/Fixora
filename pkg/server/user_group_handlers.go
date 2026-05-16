package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"fixora/pkg/auth"
	"fixora/pkg/db"
	"fixora/pkg/models"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

var errLastAdmin = errors.New("cannot remove the last admin")

func validateAdmin(r *http.Request) bool {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return false
	}
	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := auth.ValidateToken(tokenStr)
	if err != nil || claims.Role != models.RoleAdmin {
		return false
	}
	return true
}

func requireUserDB(w http.ResponseWriter) bool {
	if db.Pool == nil {
		http.Error(w, "database not connected", http.StatusInternalServerError)
		return false
	}
	return true
}

func validRole(role models.Role) bool {
	switch role {
	case models.RoleAdmin, models.RoleOperator, models.RoleViewer:
		return true
	default:
		return false
	}
}

func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	if !validateAdmin(r) {
		http.Error(w, "forbidden: admin role required", http.StatusForbidden)
		return
	}

	if r.Method == http.MethodGet {
		s.listUsers(w, r)
		return
	} else if r.Method == http.MethodPost {
		s.createUser(w, r)
		return
	}
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	if !requireUserDB(w) {
		return
	}

	// Fetch users and their groups
	rows, err := db.Pool.Query(r.Context(), `
		SELECT u.id, u.username, u.role, u.created_at, g.id, g.name 
		FROM users u 
		LEFT JOIN user_groups ug ON u.id = ug.user_id 
		LEFT JOIN groups g ON ug.group_id = g.id
	`)
	if err != nil {
		slog.Error("Failed to list users", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	userMap := make(map[string]*models.User)
	for rows.Next() {
		var u models.User
		var gID, gName *string
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt, &gID, &gName); err != nil {
			continue
		}

		if _, exists := userMap[u.ID]; !exists {
			userMap[u.ID] = &models.User{
				ID:        u.ID,
				Username:  u.Username,
				Role:      u.Role,
				CreatedAt: u.CreatedAt,
				Groups:    []models.Group{},
			}
		}

		if gID != nil && gName != nil {
			userMap[u.ID].Groups = append(userMap[u.ID].Groups, models.Group{
				ID:   *gID,
				Name: *gName,
			})
		}
	}

	users := make([]*models.User, 0, len(userMap))
	for _, u := range userMap {
		users = append(users, u)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	body := http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" || req.Role == "" {
		http.Error(w, "username, password, and role required", http.StatusBadRequest)
		return
	}
	if !validRole(req.Role) {
		http.Error(w, "invalid role", http.StatusBadRequest)
		return
	}
	if !requireUserDB(w) {
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	var user models.User
	err = db.Pool.QueryRow(
		r.Context(),
		"INSERT INTO users (username, password_hash, role) VALUES ($1, $2, $3) RETURNING id, username, role, created_at",
		req.Username, string(hashed), req.Role,
	).Scan(&user.ID, &user.Username, &user.Role, &user.CreatedAt)

	if err != nil {
		slog.Error("Failed to create user", "error", err)
		http.Error(w, "failed to create user", http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (s *Server) handleUserByID(w http.ResponseWriter, r *http.Request) {
	if !validateAdmin(r) {
		http.Error(w, "forbidden: admin role required", http.StatusForbidden)
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 5 || parts[4] == "" {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	id := parts[4]

	if r.Method == http.MethodDelete {
		if len(parts) != 5 {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if !requireUserDB(w) {
			return
		}
		if err := s.deleteUser(r, id); err != nil {
			if err == pgx.ErrNoRows {
				http.Error(w, "user not found", http.StatusNotFound)
				return
			}
			if errors.Is(err, errLastAdmin) {
				http.Error(w, err.Error(), http.StatusConflict)
				return
			}
			slog.Error("Failed to delete user", "user_id", id, "error", err)
			http.Error(w, "failed to delete user", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	} else if r.Method == http.MethodPut {
		if len(parts) != 5 && !(len(parts) == 6 && parts[5] == "role") {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		var req struct {
			Role models.Role `json:"role"`
		}
		body := http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		if err := json.NewDecoder(body).Decode(&req); err != nil || req.Role == "" {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		if !validRole(req.Role) {
			http.Error(w, "invalid role", http.StatusBadRequest)
			return
		}
		if !requireUserDB(w) {
			return
		}
		if err := s.updateUserRole(r, id, req.Role); err != nil {
			if err == pgx.ErrNoRows {
				http.Error(w, "user not found", http.StatusNotFound)
				return
			}
			if errors.Is(err, errLastAdmin) {
				http.Error(w, err.Error(), http.StatusConflict)
				return
			}
			slog.Error("Failed to update user role", "user_id", id, "role", req.Role, "error", err)
			http.Error(w, "failed to update user role", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleGroups(w http.ResponseWriter, r *http.Request) {
	if !validateAdmin(r) {
		http.Error(w, "forbidden: admin role required", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !requireUserDB(w) {
		return
	}

	if r.Method == http.MethodGet {
		rows, err := db.Pool.Query(r.Context(), "SELECT id, name, description, created_at FROM groups")
		if err != nil {
			http.Error(w, "failed to list groups", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var groups []models.Group
		for rows.Next() {
			var g models.Group
			var desc *string
			if err := rows.Scan(&g.ID, &g.Name, &desc, &g.CreatedAt); err == nil {
				if desc != nil {
					g.Description = *desc
				}
				groups = append(groups, g)
			}
		}
		if groups == nil {
			groups = []models.Group{} // return empty array instead of null
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(groups)
		return
	} else {
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		body := http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		if err := json.NewDecoder(body).Decode(&req); err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}

		var g models.Group
		err := db.Pool.QueryRow(r.Context(), "INSERT INTO groups (name, description) VALUES ($1, $2) RETURNING id, name, description, created_at", req.Name, req.Description).Scan(&g.ID, &g.Name, &g.Description, &g.CreatedAt)
		if err != nil {
			http.Error(w, "failed to create group", http.StatusConflict)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(g)
		return
	}

}

func (s *Server) handleGroupByID(w http.ResponseWriter, r *http.Request) {
	if !validateAdmin(r) {
		http.Error(w, "forbidden: admin role required", http.StatusForbidden)
		return
	}

	// Paths:
	// DELETE /api/v1/auth/groups/{id}
	// POST /api/v1/auth/groups/{id}/users/{userId}
	// DELETE /api/v1/auth/groups/{id}/users/{userId}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 5 || parts[4] == "" {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	groupID := parts[4]
	if !requireUserDB(w) {
		return
	}

	if len(parts) == 5 && r.Method == http.MethodDelete {
		_, err := db.Pool.Exec(r.Context(), "DELETE FROM groups WHERE id = $1", groupID)
		if err != nil {
			http.Error(w, "failed to delete group", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	if len(parts) == 7 && parts[5] == "users" {
		userID := parts[6]
		if r.Method == http.MethodPost {
			_, err := db.Pool.Exec(r.Context(), "INSERT INTO user_groups (user_id, group_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", userID, groupID)
			if err != nil {
				http.Error(w, "failed to add user to group", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		} else if r.Method == http.MethodDelete {
			_, err := db.Pool.Exec(r.Context(), "DELETE FROM user_groups WHERE user_id = $1 AND group_id = $2", userID, groupID)
			if err != nil {
				http.Error(w, "failed to remove user from group", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) deleteUser(r *http.Request, id string) error {
	tx, err := db.Pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())

	var role models.Role
	if err := tx.QueryRow(r.Context(), "SELECT role FROM users WHERE id = $1 FOR UPDATE", id).Scan(&role); err != nil {
		return err
	}
	if role == models.RoleAdmin {
		if err := lockAdminUsers(r, tx); err != nil {
			return err
		}
		var adminCount int
		if err := tx.QueryRow(r.Context(), "SELECT COUNT(*) FROM users WHERE role = $1", models.RoleAdmin).Scan(&adminCount); err != nil {
			return err
		}
		if adminCount <= 1 {
			return errLastAdmin
		}
	}

	if _, err := tx.Exec(r.Context(), "DELETE FROM users WHERE id = $1", id); err != nil {
		return err
	}
	return tx.Commit(r.Context())
}

func (s *Server) updateUserRole(r *http.Request, id string, nextRole models.Role) error {
	tx, err := db.Pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())

	var currentRole models.Role
	if err := tx.QueryRow(r.Context(), "SELECT role FROM users WHERE id = $1 FOR UPDATE", id).Scan(&currentRole); err != nil {
		return err
	}
	if currentRole == models.RoleAdmin && nextRole != models.RoleAdmin {
		if err := lockAdminUsers(r, tx); err != nil {
			return err
		}
		var adminCount int
		if err := tx.QueryRow(r.Context(), "SELECT COUNT(*) FROM users WHERE role = $1", models.RoleAdmin).Scan(&adminCount); err != nil {
			return err
		}
		if adminCount <= 1 {
			return errLastAdmin
		}
	}

	if _, err := tx.Exec(r.Context(), "UPDATE users SET role = $1 WHERE id = $2", nextRole, id); err != nil {
		return err
	}
	return tx.Commit(r.Context())
}

func lockAdminUsers(r *http.Request, tx pgx.Tx) error {
	rows, err := tx.Query(r.Context(), "SELECT id FROM users WHERE role = $1 FOR UPDATE", models.RoleAdmin)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
	}
	return rows.Err()
}
