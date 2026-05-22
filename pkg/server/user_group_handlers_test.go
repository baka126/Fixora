package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fixora/pkg/auth"
	"fixora/pkg/db"
	"fixora/pkg/models"
)

func adminRequest(t *testing.T, method, path, body string) *http.Request {
	t.Helper()
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	token, err := auth.GenerateToken(&models.User{
		ID:       "admin-id",
		Username: "admin",
		Role:     models.RoleAdmin,
	})
	if err != nil {
		t.Fatalf("GenerateToken returned error: %v", err)
	}
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestHandleUserByIDRejectsShortPathBeforeDB(t *testing.T) {
	originalPool := db.Pool
	db.Pool = nil
	defer func() { db.Pool = originalPool }()

	rec := httptest.NewRecorder()
	(&Server{}).handleUserByID(rec, adminRequest(t, http.MethodDelete, "/api/v1/auth/users", ""))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateUserRejectsInvalidRoleBeforeDB(t *testing.T) {
	originalPool := db.Pool
	db.Pool = nil
	defer func() { db.Pool = originalPool }()

	rec := httptest.NewRecorder()
	(&Server{}).handleUsers(rec, adminRequest(t, http.MethodPost, "/api/v1/auth/users", `{"username":"alice","password":"secret","role":"owner"}`))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleGroupsRejectsUnsupportedMethodBeforeDB(t *testing.T) {
	originalPool := db.Pool
	db.Pool = nil
	defer func() { db.Pool = originalPool }()

	rec := httptest.NewRecorder()
	(&Server{}).handleGroups(rec, adminRequest(t, http.MethodPatch, "/api/v1/auth/groups", "{}"))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestValidRole(t *testing.T) {
	for _, role := range []models.Role{models.RoleAdmin, models.RoleOperator, models.RoleViewer} {
		if !validRole(role) {
			t.Fatalf("validRole(%q) = false, want true", role)
		}
	}
	if validRole(models.Role("owner")) {
		t.Fatal("validRole(owner) = true, want false")
	}
}
