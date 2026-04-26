package controller

import (
	"testing"

	"fixora/pkg/config"
)

func TestTruncateForPreview(t *testing.T) {
	long := "abcdefghijklmnopqrstuvwxyz"
	got := truncateForPreview(long, 10)
	want := "abcdefghij\n... [truncated]"
	if got != want {
		t.Fatalf("truncateForPreview() = %q, want %q", got, want)
	}

	short := "abc"
	if got := truncateForPreview(short, 10); got != short {
		t.Fatalf("truncateForPreview short = %q, want %q", got, short)
	}
}

func TestAllowAutoFixPR(t *testing.T) {
	c := &Controller{
		config: &config.Config{
			ModeAutoFixMaxPRPerHour: 2,
		},
	}

	if !c.allowAutoFixPR() {
		t.Fatal("first PR should be allowed")
	}
	if !c.allowAutoFixPR() {
		t.Fatal("second PR should be allowed")
	}
	if c.allowAutoFixPR() {
		t.Fatal("third PR should be rate limited")
	}
}
