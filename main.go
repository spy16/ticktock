package main

import (
	"context"
	"fmt"
	"io"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/diode"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	rootCmd := &cobra.Command{
		Use:  "ticktock",
		Long: "ticktock a websocket server for streaming market data",
	}
	rootCmd.AddCommand(
		cmdServe(),
		cmdClient(),
	)

	var closeLogger func()

	var logLevel, logFormat string
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "text", "log form")
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		closeLogger = setupLogger(cmd.Context(), logLevel, logFormat)
	}

	rootCmd.PersistentPostRun = func(cmd *cobra.Command, args []string) {
		closeLogger()
	}

	rootCmd.SetContext(ctx)
	_ = rootCmd.Execute()
}

func setupLogger(ctx context.Context, level, format string) (closer func()) {
	logLevel, err := zerolog.ParseLevel(level)
	if err != nil {
		logLevel = zerolog.InfoLevel
	}

	var wr io.Writer
	if format == "text" {
		wr = &zerolog.ConsoleWriter{Out: os.Stderr}
	} else {
		wr = diode.NewWriter(os.Stderr, 1000, 10*time.Millisecond, func(missed int) {
			fmt.Printf("logger dropped %d messages", missed)
		})
	}

	log.Logger = zerolog.New(wr).With().Caller().Timestamp().Logger().Level(logLevel)

	return func() {
		// shutdown logger on context cancellation.
		if closer, ok := wr.(io.Closer); ok {
			_ = closer.Close()
		}
	}
}
