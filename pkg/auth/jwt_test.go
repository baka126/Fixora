package auth

import (
	"errors"
	"testing"

	"fixora/pkg/models"
)

const testJWTSecret = "0123456789abcdef0123456789abcdef"

func TestGenerateTokenRequiresConfiguredSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "")

	_, err := GenerateToken(&models.User{ID: "u1", Username: "admin", Role: models.RoleAdmin})
	if !errors.Is(err, ErrJWTSecretNotConfigured) {
		t.Fatalf("GenerateToken error = %v, want ErrJWTSecretNotConfigured", err)
	}
}

func TestGenerateTokenRejectsKnownInsecureSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "default-development-secret-do-not-use-in-prod")

	_, err := GenerateToken(&models.User{ID: "u1", Username: "admin", Role: models.RoleAdmin})
	if !errors.Is(err, ErrJWTSecretInsecure) {
		t.Fatalf("GenerateToken error = %v, want ErrJWTSecretInsecure", err)
	}
}

func TestGenerateAndValidateToken(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)

	token, err := GenerateToken(&models.User{ID: "u1", Username: "admin", Role: models.RoleAdmin})
	if err != nil {
		t.Fatalf("GenerateToken returned error: %v", err)
	}
	claims, err := ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken returned error: %v", err)
	}
	if claims.UserID != "u1" || claims.Username != "admin" || claims.Role != models.RoleAdmin {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}
