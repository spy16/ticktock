package utils

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/rs/zerolog/log"
)

// ServeCtx starts an HTTP server and blocks until the context is canceled.
// Context cancellation triggers a graceful shutdown of the server.
func ServeCtx(ctx context.Context, addr string, handler http.Handler) error {
	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err

	case <-ctx.Done():
		log.Debug().Msg("shutting down server")
		return srv.Shutdown(ctx)
	}
}

// WriteJSON writes v as JSON to w.
func WriteJSON(w io.Writer, v any) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Warn().Err(err).Msg("failed to write json")
	}
}
