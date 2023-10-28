package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/spf13/cobra"
)

func cmdClient() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client",
		Short: "Starts a client to connect to the socket server",
	}

	var addr string
	var count, instruments int
	cmd.Flags().StringVarP(&addr, "addr", "a", "ws://localhost:8080", "Address to connect to")
	cmd.Flags().IntVarP(&count, "count", "c", 100, "Number of clients to create")
	cmd.Flags().IntVarP(&instruments, "instruments", "i", 10, "Number of instruments to stream")

	cmd.Run = func(cmd *cobra.Command, args []string) {
		wg := &sync.WaitGroup{}
		log.Printf("creating %d clients", count)
		for i := 0; i < count; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				if err := runClient(cmd.Context(), int32(id), instruments, addr); err != nil {
					log.Printf("client %d failed: %v", id, err)
				}
			}(i)
		}
		wg.Wait()
		log.Println("all clients exited")
	}

	return cmd
}

func runClient(ctx context.Context, id int32, instruments int, addr string) error {
	conn, _, _, err := ws.Dial(ctx, addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	done := make(chan struct{})

	go func() {
		t := time.NewTicker(1 * time.Second)
		defer t.Stop()
		defer close(done)

		curLatency := 0 * time.Microsecond

		for {
			select {
			case <-t.C:
				log.Printf("latency (%d): %s", id, curLatency)

			default:
				msg, op, err := wsutil.ReadServerData(conn)
				if err != nil {
					return
				} else if op == ws.OpClose {
					return
				} else if op != ws.OpBinary {
					continue
				}

				l := time.Since(time.UnixMicro(int64(binary.BigEndian.Uint64(msg))))
				curLatency = (curLatency + l) / 2
			}
		}
	}()

	req := jsonStr(map[string]any{
		"m": 1,
		"i": []int32{
			rand.Int31n(int32(instruments)),
			rand.Int31n(int32(instruments)),
			rand.Int31n(int32(instruments)),
		},
	})

	if err := wsutil.WriteClientMessage(conn, ws.OpText, req); err != nil {
		return err
	}

	for {
		select {
		case <-done:
			return nil

		case <-ctx.Done():
			_ = wsutil.WriteClientMessage(conn, ws.OpClose, nil)
			_ = conn.Close()
			return nil
		}
	}
}

func jsonStr(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
