package gorillav1

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
	"github.com/spy16/ticktock/ticker"
	"github.com/spy16/ticktock/utils"
)

func New() (*Broker, error) {
	return &Broker{
		topics: make(map[int32]map[*wsClient]ticker.Mode),
		upgrader: &websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
	}, nil
}

// Broker is a broker implementation using gorilla websocket.
type Broker struct {
	mu     sync.RWMutex
	topics map[int32]map[*wsClient]ticker.Mode

	upgrader *websocket.Upgrader
}

// Publish publishes the given ticks to all subscribers.
func (br *Broker) Publish(_ time.Duration, ticks []ticker.Tick) error {
	br.mu.RLock()
	defer br.mu.RUnlock()
	for _, tick := range ticks {
		subs := br.topics[tick.Instrument]

		for wc, mode := range subs {
			wc.EnqueuWrite(tick.Compute(mode))
		}
	}
	return nil
}

// Serve starts the broker server.
func (br *Broker) Serve(ctx context.Context, addr string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	return utils.ServeCtx(ctx, addr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := br.upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Error().Err(err).Msg("failed to upgrade connection")
			return
		}

		wc := &wsClient{
			br:     br,
			conn:   conn,
			done:   make(chan struct{}),
			writes: make(chan []byte, 1024),
		}
		go wc.Run(ctx)
	}))
}

func (br *Broker) updateSubs(ctx context.Context, wc *wsClient, req ticker.Request) {
	br.mu.Lock()
	defer br.mu.Unlock()

	for _, instr := range req.Instruments {
		if req.Mode == ticker.ModeNone {
			if br.topics[instr] != nil {
				delete(br.topics[instr], wc)
			}
		} else {
			if br.topics[instr] == nil {
				br.topics[instr] = make(map[*wsClient]ticker.Mode)
			}
			br.topics[instr][wc] = req.Mode
		}
	}
}
