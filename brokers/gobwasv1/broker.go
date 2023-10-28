package gobwasv1

import (
	"context"
	"net/http"
	"time"

	"github.com/gobwas/ws"
	"github.com/rs/zerolog/log"
	"github.com/spy16/ticktock/ticker"
	"github.com/spy16/ticktock/utils"
)

func New() *Broker {
	return &Broker{
		topics:   make(map[int32]map[*wsClient]ticker.Mode),
		messages: make(chan []ticker.Tick, 200000),
		requests: make(chan brokerRequest, 200000),
	}
}

type Broker struct {
	topics   map[int32]map[*wsClient]ticker.Mode
	requests chan brokerRequest
	messages chan []ticker.Tick
}

type brokerRequest struct {
	ticker.Request

	Client *wsClient
}

// Publish publishes the given ticks to all subscribers.
func (br *Broker) Publish(timeout time.Duration, ticks []ticker.Tick) error {
	select {
	case br.messages <- ticks:
		return nil

	case <-time.After(timeout):
		return ticker.ErrTimeout
	}
}

// Serve starts the broker server.
func (br *Broker) Serve(ctx context.Context, addr string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go br.runManagement(ctx, cancel)

	return utils.ServeCtx(ctx, addr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, _, err := ws.UpgradeHTTP(r, w)
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

func (br *Broker) runManagement(ctx context.Context, cancel context.CancelFunc) {
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return

		case ticks := <-br.messages:
			for _, tick := range ticks {
				subs := br.topics[tick.Instrument]

				for wc, mode := range subs {
					wc.EnqueuWrite(tick.Compute(mode))
				}
			}

		case cmd := <-br.requests:
			for _, instr := range cmd.Instruments {
				if cmd.Mode == ticker.ModeNone {
					if br.topics[instr] != nil {
						delete(br.topics[instr], cmd.Client)
					}
				} else {
					if br.topics[instr] == nil {
						br.topics[instr] = make(map[*wsClient]ticker.Mode)
					}
					br.topics[instr][cmd.Client] = cmd.Mode
				}
			}
		}
	}
}

func (br *Broker) updateSubs(ctx context.Context, wc *wsClient, req ticker.Request) {
	select {
	case br.requests <- brokerRequest{Request: req, Client: wc}:
	case <-ctx.Done():
	}
}
