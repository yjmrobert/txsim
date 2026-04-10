package engine

import "fmt"

// Outcome represents the deterministic result of a mock payment operation.
type Outcome string

const (
	OutcomeSucceeded       Outcome = "succeeded"
	OutcomeCardDeclined    Outcome = "card_declined"
	OutcomeInsufficient    Outcome = "insufficient_funds"
	OutcomeExpiredCard     Outcome = "expired_card"
	OutcomeProcessingError Outcome = "processing_error"
	OutcomeRateLimited     Outcome = "rate_limited"
)

// AllOutcomes is the set of Outcome values MockPay understands. Order is
// preserved for help text.
var AllOutcomes = []Outcome{
	OutcomeSucceeded,
	OutcomeCardDeclined,
	OutcomeInsufficient,
	OutcomeExpiredCard,
	OutcomeProcessingError,
	OutcomeRateLimited,
}

// ParseOutcome converts a user-supplied status string into an Outcome. An
// empty string is treated as OutcomeSucceeded.
func ParseOutcome(s string) (Outcome, error) {
	if s == "" {
		return OutcomeSucceeded, nil
	}
	for _, o := range AllOutcomes {
		if string(o) == s {
			return o, nil
		}
	}
	return "", fmt.Errorf("unknown status %q (valid: %v)", s, AllOutcomes)
}

// IsFailure reports whether the outcome represents a failed payment.
func (o Outcome) IsFailure() bool { return o != OutcomeSucceeded }

// Message returns a human-readable description of the outcome, suitable for
// embedding in provider-shaped error envelopes.
func (o Outcome) Message() string {
	switch o {
	case OutcomeSucceeded:
		return "Payment succeeded."
	case OutcomeCardDeclined:
		return "Your card was declined."
	case OutcomeInsufficient:
		return "Your card has insufficient funds."
	case OutcomeExpiredCard:
		return "Your card has expired."
	case OutcomeProcessingError:
		return "An error occurred while processing your card. Try again in a little bit."
	case OutcomeRateLimited:
		return "Too many requests made to the API too quickly."
	}
	return string(o)
}
