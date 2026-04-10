package engine

import "time"

// Sleeper is the function used by the engine to simulate latency. Tests can
// replace it with a no-op to keep runs fast.
var Sleeper = time.Sleep

// Delay pauses for the given number of milliseconds when ms > 0.
func Delay(ms int) {
	if ms <= 0 {
		return
	}
	Sleeper(time.Duration(ms) * time.Millisecond)
}
