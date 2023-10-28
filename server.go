package main

import (
	"context"
	_ "embed"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spy16/ticktock/brokers/gorillav1"
	"github.com/spy16/ticktock/brokers/gorillav2"
	"github.com/spy16/ticktock/ticker"
	"github.com/spy16/ticktock/utils"
)

//go:embed index.html
var indexFile string

// Server is a websocket server.
type Server interface {
	Serve(ctx context.Context, addr string) error
}

func cmdServe() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Starts the socket server",
	}

	var addr, serverType, brokerType string
	var count, tradeCount int
	var tickRate time.Duration
	cmd.Flags().StringVarP(&addr, "addr", "a", ":8080", "server address")
	cmd.Flags().IntVarP(&count, "instruments", "i", 100, "Number of instruments to stream")
	cmd.Flags().StringVarP(&serverType, "server", "s", "gorillav1", "Server Model to be Used")
	cmd.Flags().StringVarP(&brokerType, "broker", "b", "lockbased", "Broker Model to be Used")
	cmd.Flags().DurationVarP(&tickRate, "tickrate", "t", 100*time.Millisecond, "Tick Rate")
	cmd.Flags().IntVarP(&tradeCount, "trade-count", "c", 5000, "Number of trades to generate per tick")

	cmd.Run = func(cmd *cobra.Command, args []string) {
		if v, err := utils.SetMaxFdLimit(); err != nil {
			log.Warn().Err(err).Msg("failed to set max open files")
		} else {
			log.Info().Uint64("max_open_files", v).Msg("max open files set")
		}

		go func() {
			http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(indexFile))
			})
			addr := ":6060"
			log.Info().Str("addr", addr).Msg("starting pprof server")
			if err := http.ListenAndServe(addr, nil); err != nil {
				log.Fatal().Err(err).Msg("pprof server exited")
			}
		}()

		srv, publisher := setupServerAndPublisher(cmd.Context(), serverType, brokerType)

		// start a tick source that publishes random data to the broker.
		ts := &ticker.Ticker{
			TickRate:   tickRate,
			Publisher:  publisher,
			TradeCount: tradeCount,
		}
		go ts.Run(cmd.Context())

		log.Info().Str("addr", addr).Msg("starting server")
		if err := srv.Serve(cmd.Context(), addr); err != nil {
			log.Fatal().Err(err).Msg("server exited")
		}
	}

	return cmd
}

func setupServerAndPublisher(ctx context.Context, serverType, brokerType string) (Server, ticker.Publisher) {
	switch serverType {

	case "gorillav1":
		srv, err := gorillav1.New()
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create server")
		}
		return srv, srv

	case "gorillav2":
		srv, err := gorillav2.New()
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create server")
		}
		return srv, srv

	default:
		log.Fatal().Str("server", serverType).Msg("unknown server type")
		return nil, nil
	}
}
