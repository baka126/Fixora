package notifications

import (
	"encoding/json"
	"strings"
	"testing"

	"fixora/pkg/config"
)

func TestBuildAuditPayloadScrubsEvidenceAndActionDetails(t *testing.T) {
	cfg := &config.Config{
		CustomLogScrubbingPatterns: []string{`ticket-[0-9]+`},
	}
	evidence := EvidenceChain{
		Namespace:         "orders",
		PodName:           "api-10.1.2.3",
		MetricProof:       "owner alice@example.com saw ticket-123",
		ClusterContext:    "bearer=supersecrettoken",
		HistoricalPattern: "prior ticket-999",
		EventTimeline:     "node 192.168.1.10 restarted",
		RootCause:         "password: ultrasecretpassword",
		FinOpsImpact:      "email bob@example.com",
		StackTrace:        "token: abcdefghijklmnopqrstuvwxyz",
		FinOpsDetails:     "ticket-456",
		ValidatedClaims:   []string{"owner alice@example.com proved ticket-321"},
		UnvalidatedClaims: []string{"token: zyxwvutsrqponmlkjihgfedcba needs review"},
	}

	payload := buildAuditPayload(cfg, evidence, "fix", "created", "approved by carol@example.com for ticket-777")

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	body := string(encoded)
	for _, leaked := range []string{
		"alice@example.com",
		"bob@example.com",
		"carol@example.com",
		"10.1.2.3",
		"192.168.1.10",
		"supersecrettoken",
		"ultrasecretpassword",
		"abcdefghijklmnopqrstuvwxyz",
		"ticket-123",
		"ticket-456",
		"ticket-777",
		"ticket-999",
		"ticket-321",
		"zyxwvutsrqponmlkjihgfedcba",
	} {
		if strings.Contains(body, leaked) {
			t.Fatalf("audit payload leaked %q: %s", leaked, body)
		}
	}
}
