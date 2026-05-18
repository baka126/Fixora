package controller

import "testing"

func TestClusterLogPatternsNormalizesVariableTokens(t *testing.T) {
	logs := "panic: user 123 failed at 2026-05-18T10:00:00Z\npanic: user 456 failed at 2026-05-18T10:01:00Z\ninfo: ready\n"
	got := ClusterLogPatterns(logs, 2)
	if len(got) != 2 {
		t.Fatalf("patterns = %d, want 2: %#v", len(got), got)
	}
	if got[0].Count != 2 || got[0].Pattern != "panic: user <var> failed at <var>" {
		t.Fatalf("top pattern = %#v, want normalized panic count 2", got[0])
	}
}
