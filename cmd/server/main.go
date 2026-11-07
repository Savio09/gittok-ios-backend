// Command server is the entry point for the GitTok backend.
//
// It wires together the independent packages that each own one concern
// (GitHub ingestion, crawling, preview building, selection, storage, and
// serving the feed) and exposes them over HTTP to the SwiftUI app.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/analysis"
	"github.com/Savio09/gittok-ios-backend/internal/config"
	"github.com/Savio09/gittok-ios-backend/internal/feed"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
	"github.com/Savio09/gittok-ios-backend/internal/walkthrough"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("gittok: invalid configuration: %v", err)
	}

	previewSvc, mode, err := previewService(context.Background(), cfg)
	if err != nil {
		log.Fatalf("gittok: preview service: %v", err)
	}
	log.Printf("gittok: previews: %s", mode)
	// previewService has verified this same public-only credential at startup.
	// Discovery also checks scopes on every response; the broad legacy token
	// must never be used for public discovery.
	discoveryClient, err := githubclient.NewPublic(cfg.PublicToken, githubclient.WithUserAgent("gittok-discovery"))
	if err != nil {
		log.Fatal("gittok: public discovery credential rejected")
	}
	feedSvc := feed.NewPublicDiscoveryService(discoveryClient, 10*time.Minute)
	walkthroughSvc, closeWalkthrough, err := walkthroughService(context.Background(), cfg)
	if err != nil {
		log.Fatalf("gittok: walkthrough service: %v", err)
	}
	defer func() {
		if err := closeWalkthrough(); err != nil {
			log.Print("gittok: walkthrough control state could not be closed cleanly")
		}
	}()
	if walkthroughSvc == nil {
		log.Print("gittok: walkthrough generation disabled")
	} else {
		log.Print("gittok: source walkthrough generation enabled; single bounded worker")
	}

	srv := &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.Port),
		Handler:           newRouter(feedSvc, previewSvc, walkthroughSvc),
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

// newRouter builds the HTTP handler for the backend: the operational health
// check, v1 feed and on-demand preview. Further v1 endpoints (profile, events) are
// added in later tickets.
func newRouter(feedSvc interface{ Handler() http.HandlerFunc }, previewSvc *analysis.Service, storyServices ...*walkthrough.Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	if feedSvc != nil {
		mux.Handle("GET /v1/feed", feedSvc.Handler())
	}
	mux.Handle("GET /v1/repos/{id}/preview", previewSvc.Handler())
	mux.Handle("GET /v1/repos/{owner}/{name}/preview", previewSvc.Handler())
	var stories *walkthrough.Service
	if len(storyServices) > 0 {
		stories = storyServices[0]
	}
	mux.Handle("POST /v1/repos/{owner}/{name}/walkthrough", stories.Handler())
	mux.Handle("GET /v1/walkthroughs/{id}", stories.Handler())
	return mux
}

// previewService builds the public preview service from its own credential
// and cache settings, independently of the feed's token. A configured public
// credential is checked for family and verified once at startup; an unsafe or
// rejected one is an error, never a fallback to anonymous access. The returned
// description of the operating mode never contains the credential.
func previewService(ctx context.Context, cfg config.Config, opts ...githubclient.Option) (*analysis.Service, string, error) {
	opts = append([]githubclient.Option{githubclient.WithUserAgent("gittok-preview")}, opts...)
	client, err := githubclient.NewPublic(cfg.PublicToken, opts...)
	if err != nil {
		return nil, "", fmt.Errorf("GITHUB_PUBLIC_TOKEN rejected: %w", err)
	}
	vctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := client.VerifyCredential(vctx); err != nil {
		return nil, "", fmt.Errorf("GITHUB_PUBLIC_TOKEN could not be verified as a no-scope public credential: %w", err)
	}
	svc, err := analysis.NewPublicServiceFromClient(client)
	if err != nil {
		return nil, "", err
	}
	analysisLimits := cfg.AnalysisLimits
	if analysisLimits == (analysis.Limits{}) {
		analysisLimits = analysis.DefaultLimits()
	}
	if err := svc.UseLimits(analysisLimits); err != nil {
		return nil, "", fmt.Errorf("analysis limits: %w", err)
	}
	limits := analysis.DefaultCacheLimits()
	limits.TTL = cfg.PreviewCacheTTL
	dir := cfg.PreviewCacheDir
	if dir == "" {
		dir = analysis.DefaultCacheDir()
	}
	cache, err := analysis.OpenPreviewCache(dir, limits, time.Now)
	if err != nil {
		return nil, "", fmt.Errorf("preview cache: %w", err)
	}
	svc.UseCache(cache)
	access := "public-only token (classic PAT, no OAuth scopes) verified"
	if client.Mode() == "anonymous" {
		access = "anonymous GitHub access (60 requests/hour per outbound IP, shared on hosted platforms; set GITHUB_PUBLIC_TOKEN before a demo)"
	}
	return svc, fmt.Sprintf("%s; analysis %s, %d files / %d API attempts; cache %s, TTL %s, %d entries / %d MiB", access, analysisLimits.Timeout, analysisLimits.MaxFiles, analysisLimits.MaxAPICalls, dir, limits.TTL, limits.MaxEntries, limits.MaxBytes>>20), nil
}

// handleHealthz reports that the process is up and able to serve requests.
// It is intended for load balancers and deployment platforms, not the app.
func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
