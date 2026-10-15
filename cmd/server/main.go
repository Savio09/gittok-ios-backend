// Command server is the entry point for the GitTok backend.
//
// It wires together the independent packages that each own one concern
// (GitHub ingestion, crawling, preview building, selection, storage, and
// serving the feed) and exposes them over HTTP to the SwiftUI app.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("gittok: invalid configuration: %v", err)
	}

	srv := &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.Port),
		Handler:           newRouter(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	if err := run(srv); err != nil {
		log.Fatalf("gittok: %v", err)
	}
}

// run starts srv and blocks until an interrupt or terminate signal arrives,
// then shuts it down gracefully so in-flight requests can finish. It returns
// a non-nil error only for an actual failure, not for a clean shutdown.
func run(srv *http.Server) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("gittok: listening on %s", srv.Addr)
		serverErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		log.Print("gittok: shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return errors.New("graceful shutdown failed: " + err.Error())
		}
		log.Print("gittok: stopped")
		return nil
	}
}

// newRouter builds the HTTP handler for the backend. Feed endpoints are added
// in later tickets; for now it exposes only the operational health check.
func newRouter() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	return mux
}

// handleHealthz reports that the process is up and able to serve requests.
// It is intended for load balancers and deployment platforms, not the app.
func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
