package rooms

import (
	"testing"
	"time"
)

func TestTheLedgerForgetsRoomsNobodyJoins(t *testing.T) {
	clock := time.Now()
	l := newLedger(pendingTTL, func() time.Time { return clock })
	allow := func(int64) error { return nil }
	for _, slug := range []string{"junk-1", "junk-2", "junk-3"} {
		if err := l.admit(slug, joiner{identity: "guest-x"}, nil, "", allow); err != nil {
			t.Fatalf("admit: %v", err)
		}
	}
	clock = clock.Add(2 * pendingTTL)
	if err := l.admit("kept", joiner{identity: "guest-y"}, nil, "", allow); err != nil {
		t.Fatalf("admit: %v", err)
	}
	if len(l.pending) != 1 {
		t.Fatalf("ledger holds %d rooms after a sweep, want only the fresh one", len(l.pending))
	}
}
