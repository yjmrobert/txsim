package engine

import (
	"crypto/rand"
	"encoding/hex"
)

// idLen is the number of random hex characters appended to each ID.
const idLen = 16

// NewID returns a provider-style ID with the given prefix, e.g.
// NewID("ch") → "ch_7f3a...". Panics only if crypto/rand fails, which is
// effectively fatal anyway.
func NewID(prefix string) string {
	buf := make([]byte, idLen/2)
	if _, err := rand.Read(buf); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return prefix + "_" + hex.EncodeToString(buf)
}
