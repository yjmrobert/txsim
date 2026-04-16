package engine_test

import (
	"testing"

	"github.com/yjmrobert/txsim/internal/engine"
)

// FuzzParseOutcome feeds arbitrary strings to ParseOutcome and asserts two
// invariants:
//  1. It never panics.
//  2. A string that round-trips an Outcome always parses back to that Outcome.
//
// Go's native fuzzer runs for a short budget by default and replays seeded
// corpora on every `go test` run, so this doubles as a regression suite for
// any bug found during fuzzing.
func FuzzParseOutcome(f *testing.F) {
	// Seed with every known outcome plus a few interesting edges.
	for _, o := range engine.AllOutcomes {
		f.Add(string(o))
	}
	f.Add("")
	f.Add("SUCCEEDED") // case sensitive — should NOT match
	f.Add("card_declined ")
	f.Add("\x00null byte")

	f.Fuzz(func(t *testing.T, s string) {
		got, err := engine.ParseOutcome(s)
		if err != nil {
			// Must not silently produce a zero Outcome when returning an error.
			if got != "" {
				t.Fatalf("ParseOutcome(%q) returned both value %q and error %v", s, got, err)
			}
			return
		}
		// If parsing succeeded, the returned Outcome must round-trip.
		back, err := engine.ParseOutcome(string(got))
		if err != nil {
			t.Fatalf("round-trip: ParseOutcome(%q) -> %v", string(got), err)
		}
		if back != got {
			t.Fatalf("round-trip mismatch: %q -> %q -> %q", s, got, back)
		}
	})
}
