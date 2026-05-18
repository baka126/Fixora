package security

import (
	"strings"
	"testing"
)

func TestTokenizerRedactsAndRestoresCloudAndPrivateKeys(t *testing.T) {
	input := strings.Join([]string{
		"access_key: AKIA_EXAMPLE_DUMMY_KEY",
		"aws_secret_access_key=EXAMPLE_DUMMY_SECRET_KEY_WITH_LONG_STRING",
		"-----BEGIN PRIVATE KEY-----",
		"abc123",
		"-----END PRIVATE KEY-----",
	}, "\n")

	tokenizer := NewTokenizer()
	tokenized := tokenizer.Tokenize(input)

	for _, sensitive := range []string{"AKIA_EXAMPLE_DUMMY_KEY", "EXAMPLE_DUMMY_SECRET_KEY_WITH_LONG_STRING", "abc123"} {
		if strings.Contains(tokenized, sensitive) {
			t.Fatalf("expected tokenized content to hide %q, got:\n%s", sensitive, tokenized)
		}
	}
	if !strings.Contains(tokenized, "FIXORA_AWS_") || !strings.Contains(tokenized, "FIXORA_PRIVATE_KEY_") {
		t.Fatalf("expected typed tokens in tokenized content, got:\n%s", tokenized)
	}

	if restored := tokenizer.Detokenize(tokenized); restored != input {
		t.Fatalf("detokenize mismatch\nwant:\n%s\ngot:\n%s", input, restored)
	}
}
