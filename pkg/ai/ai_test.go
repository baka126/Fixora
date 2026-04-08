package ai

import (
	"bytes"
	"testing"
)

func TestCleanPatch(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expected []byte
	}{
		{
			name:     "no markdown",
			raw:      "key: value",
			expected: []byte("key: value"),
		},
		{
			name:     "yaml markdown",
			raw:      "```yaml\nkey: value\n```",
			expected: []byte("key: value"),
		},
		{
			name:     "json markdown",
			raw:      "```json\n{\"key\": \"value\"}\n```",
			expected: []byte("{\"key\": \"value\"}"),
		},
		{
			name:     "plain markdown",
			raw:      "```\nkey: value\n```",
			expected: []byte("key: value"),
		},
		{
			name:     "with whitespace",
			raw:      "  ```yaml\nkey: value\n```  ",
			expected: []byte("key: value"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CleanPatch(tt.raw)
			if !bytes.Equal(got, tt.expected) {
				t.Errorf("CleanPatch() = %v, want %v", string(got), string(tt.expected))
			}
		})
	}
}
