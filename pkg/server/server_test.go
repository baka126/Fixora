package server

import "testing"

func TestMatchesBearerToken(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string
		expected   string
		want       bool
	}{
		{
			name:       "exact token matches",
			authHeader: "Bearer secret-token-123",
			expected:   "secret-token-123",
			want:       true,
		},
		{
			name:       "prefix token mismatch",
			authHeader: "Bearer secret-token-123-extra",
			expected:   "secret-token-123",
			want:       false,
		},
		{
			name:       "wrong scheme",
			authHeader: "Basic abc123",
			expected:   "abc123",
			want:       false,
		},
		{
			name:       "empty expected token",
			authHeader: "Bearer anything",
			expected:   "",
			want:       false,
		},
		{
			name:       "trailing whitespace is allowed",
			authHeader: "Bearer secret-token-123  ",
			expected:   "secret-token-123",
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesBearerToken(tt.authHeader, tt.expected)
			if got != tt.want {
				t.Fatalf("matchesBearerToken(%q, %q) = %v, want %v", tt.authHeader, tt.expected, got, tt.want)
			}
		})
	}
}

func TestIsPendingFixCallback(t *testing.T) {
	tests := []struct {
		name       string
		callbackID string
		want       bool
	}{
		{name: "fix prefix", callbackID: "fix-123", want: true},
		{name: "legacy patch prefix", callbackID: "patch-123", want: true},
		{name: "rollout callback", callbackID: "rollout-restart-ns-app", want: false},
		{name: "empty callback", callbackID: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPendingFixCallback(tt.callbackID); got != tt.want {
				t.Fatalf("isPendingFixCallback(%q) = %v, want %v", tt.callbackID, got, tt.want)
			}
		})
	}
}
