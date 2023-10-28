package ticker

import (
	"errors"
	"time"
)

// Request types.
const (
	ModeNone  Mode = iota // Unsubscribe.
	ModeLTP               // LTP only.
	ModeQuote             // LTP + Quote
	ModeFull              // LTP + Quote + Market Depth, etc.
)

// ErrTimeout should be returned when a publish operation times out.
var ErrTimeout = errors.New("timeout")

// Mode indicates the subscription mode.
type Mode int32

// Publisher is a broker that publishes ticks to subscribers.
type Publisher interface {
	Publish(timeout time.Duration, ticks []Tick) error
}

// Tick represents a single tick data for an instrument.
type Tick struct {
	Data       []byte
	Instrument int32
}

// Compute computes the message data based on the subscription mode.
func (tic *Tick) Compute(mode Mode) []byte {
	// TODO: slice the data based on the mode.
	return tic.Data
}

// Request is a request from client. It is used to subscribe/unsubscribe to
// instruments.
type Request struct {
	Mode        Mode    `json:"m"`
	Instruments []int32 `json:"i"`
}
