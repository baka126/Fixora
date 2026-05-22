package auth

import (
	"errors"
	"os"
	"strings"
	"time"

	"fixora/pkg/models"
	"github.com/golang-jwt/jwt/v5"
)

const minJWTSecretLength = 32

var (
	ErrJWTSecretNotConfigured = errors.New("JWT_SECRET must be configured with at least 32 characters")
	ErrJWTSecretInsecure      = errors.New("JWT_SECRET uses a known insecure default")
)

var insecureJWTSecrets = map[string]bool{
	"default-development-secret-do-not-use-in-prod": true,
	"changeme":  true,
	"change-me": true,
	"secret":    true,
	"password":  true,
}

type Claims struct {
	UserID   string      `json:"user_id"`
	Username string      `json:"username"`
	Role     models.Role `json:"role"`
	jwt.RegisteredClaims
}

func signingSecret() ([]byte, error) {
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secret == "" || len(secret) < minJWTSecretLength {
		return nil, ErrJWTSecretNotConfigured
	}
	if insecureJWTSecrets[strings.ToLower(secret)] {
		return nil, ErrJWTSecretInsecure
	}
	return []byte(secret), nil
}

func RequireConfiguredSecret() error {
	_, err := signingSecret()
	return err
}

func GenerateToken(user *models.User) (string, error) {
	secret, err := signingSecret()
	if err != nil {
		return "", err
	}
	claims := Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

func ValidateToken(tokenString string) (*Claims, error) {
	secret, err := signingSecret()
	if err != nil {
		return nil, err
	}
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return secret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}
