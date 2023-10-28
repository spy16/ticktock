package gorillav3

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
	"github.com/spy16/ticktock/ticker"
)

const readTimeout = 60 * time.Second

type wsClient struct {
	br     *Broker
	done   chan struct{}
	conn   *websocket.Conn
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
	defer func() {
		close(wc.done)
		wc.br.removeSub(ctx, wc)
		cancel()
	}()

	if err := wc.conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
		log.Error().Err(err).Msg("failed to set read deadline")
		return
	}

	wc.conn.SetPongHandler(func(string) error {
		return wc.conn.SetReadDeadline(time.Now().Add(readTimeout))
	})

	go wc.runReader(ctx, cancel)

	writeTick := time.NewTicker(100 * time.Millisecond)
	defer writeTick.Stop()

	var buf bytes.Buffer

	for {
		select {
		case <-ctx.Done():
			return

		case <-writeTick.C:
			if buf.Len() == 0 {
				continue
			}

			if err := wc.conn.WriteMessage(websocket.BinaryMessage, buf.Bytes()); err != nil {
				if isClose(err) {
					return
				}
				log.Error().Err(err).Msg("failed to write message")
			}
			buf.Reset()

		case msg, ok := <-wc.writes:
			if !ok {
				return
			}
			_, _ = buf.Write(msg)
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
			msgType, msg, err := wc.conn.ReadMessage()
			if err != nil {
				if isClose(err) {
					return
				}
				log.Error().Err(err).Msg("failed to read message")
				return
			}

			switch msgType {
			case websocket.TextMessage:
				var req ticker.Request
				if err := json.Unmarshal(msg, &req); err != nil {
					log.Warn().Err(err).Msg("failed to unmarshal request")
					continue // ignore invalid requests
				}
				wc.br.updateSubs(ctx, wc, req)

			case websocket.CloseMessage:
				return

			default:
				log.Warn().Int("type", msgType).Msg("unexpected message type")
			}
		}
	}
}

func isClose(err error) bool {
	if errors.Is(err, websocket.ErrCloseSent) {
		return true
	} else if websocket.IsCloseError(err, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
		return true
	} else if errors.Is(err, io.EOF) {
		return true
	} else if errors.Is(err, syscall.EPIPE) {
		return true
	}

	return false
}
