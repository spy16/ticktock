package gobwasv2

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gobwas/ws"
	"github.com/rs/zerolog/log"
	"github.com/smallnest/epoller"
	"github.com/spy16/ticktock/ticker"
	"github.com/spy16/ticktock/utils"
)

func New() *Broker {
	p, err := epoller.NewPoller()
	if err != nil {
		panic(err)
	}

	return &Broker{
		poller:  p,
		clients: make(map[net.Conn]*wsClient),
		topics:  make(map[int32]map[*wsClient]ticker.Mode),

		ioEvents: make(chan net.Conn, 200000),
		messages: make(chan []ticker.Tick, 200000),
		requests: make(chan brokerRequest, 200000),
	}
}

type Broker struct {
	mu      sync.RWMutex
	clients map[net.Conn]*wsClient

	poller epoller.Poller
	topics map[int32]map[*wsClient]ticker.Mode

	ioEvents chan net.Conn
	requests chan brokerRequest
	messages chan []ticker.Tick
}

type brokerRequest struct {
	ticker.Request

	Client *wsClient
	Remove bool
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
	go br.runPoller(ctx, cancel)

	return utils.ServeCtx(ctx, addr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, rw, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			log.Error().Err(err).Msg("failed to upgrade connection")
			return
		}

		if tc, ok := conn.(*net.TCPConn); ok {
			tc.SetNoDelay(true)
		}

		if err := br.poller.Add(conn); err != nil {
			log.Error().Err(err).Msg("failed to add connection to poller")
			return
		}

		wc := &wsClient{
			br:     br,
			rw:     rw,
			conn:   conn,
			done:   make(chan struct{}),
			reads:  make(chan struct{}, 10000),
			writes: make(chan []byte, 10000),
		}

		br.mu.Lock()
		br.clients[wc.conn] = wc
		br.mu.Unlock()

		go func() {
			wc.Run(ctx)
			_ = br.poller.Remove(wc.conn)
		}()
	}))
}

func (br *Broker) runManagement(ctx context.Context, cancel context.CancelFunc) {
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return

		case conn, ok := <-br.ioEvents:
			if !ok {
				return
			}

			br.mu.RLock()
			client, found := br.clients[conn]
			br.mu.RUnlock()

			if found {
				select {
				case <-ctx.Done():

				case client.reads <- struct{}{}:
				}
			}

		case ticks := <-br.messages:
			for _, tick := range ticks {
				subs := br.topics[tick.Instrument]

				for wc, mode := range subs {
					wc.EnqueuWrite(tick.Compute(mode))
				}
			}

		case cmd := <-br.requests:
			if cmd.Remove {
				for _, instr := range cmd.Instruments {
					delete(br.topics[instr], cmd.Client)
				}
			} else {
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
}

func (br *Broker) runPoller(ctx context.Context, cancel context.CancelFunc) {
	defer cancel()
	defer close(br.ioEvents)
	defer br.poller.Close(true)

	for {
		select {
		case <-ctx.Done():
			return

		default:
			conns, err := br.poller.Wait(1024)
			if err != nil {
				log.Error().Err(err).Msg("failed to poll")
				continue
			} else {
				log.Debug().Int("count", len(conns)).Msg("polled")
			}

			for _, conn := range conns {
				if pConn, ok := conn.(epoller.ConnImpl); ok {
					conn = pConn.Conn
				}

				select {
				case br.ioEvents <- conn:
				case <-ctx.Done():
					return
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

func (br *Broker) removeSub(ctx context.Context, wc *wsClient) {
	select {
	case br.requests <- brokerRequest{Client: wc, Remove: true}:
	case <-ctx.Done():
	}
}
