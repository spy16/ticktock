package quickwsv1

import (
	"context"
	"encoding/json"

	"github.com/antlabs/quickws"
	"github.com/rs/zerolog/log"
	"github.com/spy16/ticktock/ticker"
)

type wsClient struct {
	br     *Broker
	ctx    context.Context
	conn   *quickws.Conn
	done   chan struct{}
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

func (e *wsClient) Run(ctx context.Context) {
	e.conn.StartReadLoop()

	log.Debug().Msg("started read loop")
	for {
		select {
		case <-ctx.Done():
			return

		case msg, ok := <-e.writes:
			if !ok {
				return
			}

			if err := e.conn.WriteMessage(quickws.Binary, msg); err != nil {
				log.Error().Err(err).Msg("failed to write message")
				return
			}
		}
	}
}

func (e *wsClient) OnOpen(c *quickws.Conn) {}

func (e *wsClient) OnMessage(c *quickws.Conn, op quickws.Opcode, msg []byte) {
	if op == quickws.Text {
		var req ticker.Request
		if err := json.Unmarshal(msg, &req); err != nil {
			log.Warn().Err(err).Msg("failed to unmarshal request")
			return // ignore invalid requests
		}
		log.Debug().Interface("req", req).Msg("got request")
		e.br.updateSubs(e.ctx, e, req)
	}
}

func (e *wsClient) OnClose(c *quickws.Conn, err error) {
	close(e.done)
	e.br.removeSub(e.ctx, e)
	close(e.writes)
}
