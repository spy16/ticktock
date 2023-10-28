package ticker

import (
	"context"
	"encoding/binary"
	"math/rand"
	"time"

	"github.com/paulbellamy/ratecounter"
	"github.com/rs/zerolog/log"
)

// Ticker is a source of ticks.
type Ticker struct {
	Publisher Publisher

	TickRate   time.Duration
	TradeCount int
}

// Run starts the ticker.
func (ts *Ticker) Run(ctx context.Context) {
	tick := time.NewTicker(ts.TickRate)
	defer tick.Stop()

	rateTick := time.NewTicker(10 * time.Second)
	defer rateTick.Stop()

	counter := ratecounter.NewRateCounter(1 * time.Second)

	b := make([]byte, 8)
	for {
		select {
		case <-ctx.Done():
			return

		case <-rateTick.C:
			log.Debug().Int64("rate", counter.Rate()).Msg("tick rate")

		case t := <-tick.C:
			binary.BigEndian.PutUint64(b, uint64(t.UnixMicro()))

			updateCount := rand.Intn(ts.TradeCount)
			instrs := make([]Tick, updateCount)
			for i := 0; i < updateCount; i++ {
				instrs[i] = Tick{Instrument: int32(i), Data: b}
			}
			if err := ts.Publisher.Publish(5*time.Millisecond, instrs); err != nil {
				log.Warn().Err(err).Msg("failed to publish")
			}
			counter.Incr(int64(updateCount))
		}
	}
}
