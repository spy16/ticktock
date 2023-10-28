package gobwasv1

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"syscall"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/rs/zerolog/log"
	"github.com/spy16/ticktock/ticker"
)

type wsClient struct {
	br     *Broker
	done   chan struct{}
	conn   net.Conn
	writes chan []byte
}

func (wc *wsClient) EnqueuWrite(msg []byte) {
	select {
	case <-wc.done:
		return // client is closed

	case wc.writes <- msg:
		return
	}
}

func (wc *wsClient) Run(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go wc.runReader(ctx, cancel)

	for {
		select {
		case <-ctx.Done():
			return

		case msg, ok := <-wc.writes:
			if !ok {
				return
			}

			if err := wsutil.WriteServerMessage(wc.conn, ws.OpBinary, msg); err != nil {
				if errors.Is(err, syscall.EPIPE) {
					return
				}
				log.Error().Err(err).Msg("failed to write message")
				return
			}
		}
	}
}

func (wc *wsClient) runReader(ctx context.Context, cancel context.CancelFunc) {
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return

		default:
			msg, op, err := wsutil.ReadClientData(wc.conn)
			if err != nil {
				var closeErr wsutil.ClosedError
				if errors.As(err, &closeErr) {
					return
				} else if errors.Is(err, syscall.EPIPE) {
					return
				}

				log.Error().Err(err).Msg("failed to read message")
				return
			} else if op == ws.OpClose {
				return
			} else if op != ws.OpText {
				continue
			}

			var req ticker.Request
			if err := json.Unmarshal(msg, &req); err != nil {
				log.Warn().Err(err).Msg("failed to unmarshal request")
				continue // ignore invalid requests
			}
			wc.br.updateSubs(ctx, wc, req)
		}
	}
}
