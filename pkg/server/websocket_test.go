package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"fixora/pkg/auth"
	"fixora/pkg/models"
)

const serverTestJWTSecret = "0123456789abcdef0123456789abcdef"

func websocketUpgradeRequest(path string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	return req
}

func TestHandleWebSocketRejectsMissingJWT(t *testing.T) {
	t.Setenv("JWT_SECRET", serverTestJWTSecret)
	req := websocketUpgradeRequest("/api/v1/ws")
	rec := httptest.NewRecorder()

	(&Server{}).handleWebSocket(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestWebSocketTokenAcceptsQueryToken(t *testing.T) {
	t.Setenv("JWT_SECRET", serverTestJWTSecret)
	token, err := auth.GenerateToken(&models.User{ID: "u1", Username: "viewer", Role: models.RoleViewer})
	if err != nil {
		t.Fatal(err)
	}
	req := websocketUpgradeRequest("/api/v1/ws?token=" + token)

	claims, ok := (&Server{}).authenticateWebSocket(httptest.NewRecorder(), req)
	if !ok || claims == nil || claims.UserID != "u1" {
		t.Fatalf("claims = %#v, ok = %v", claims, ok)
	}
}

func TestCheckWebSocketOriginRequiresSameHost(t *testing.T) {
	server := &Server{}
	req := websocketUpgradeRequest("http://fixora.example.com/api/v1/ws")
	req.Host = "fixora.example.com"
	req.Header.Set("Origin", "https://fixora.example.com")
	if !server.checkWebSocketOrigin(req) {
		t.Fatal("same host origin was rejected")
	}

	req.Header.Set("Origin", "https://evil.example.com")
	if server.checkWebSocketOrigin(req) {
		t.Fatal("cross-origin websocket request was accepted")
	}
}
