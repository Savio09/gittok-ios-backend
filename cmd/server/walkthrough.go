package main

import (
	"context"
	"errors"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/config"
	"github.com/Savio09/gittok-ios-backend/internal/generation"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
	"github.com/Savio09/gittok-ios-backend/internal/walkthrough"
)

func walkthroughService(ctx context.Context, cfg config.Config) (*walkthrough.Service, func() error, error) {
	noop := func() error { return nil }
	c := cfg.Walkthrough
	if !c.Enabled {
		return nil, noop, nil
	}
	ledger, err := generation.OpenLedger(c.LedgerPath, generation.LedgerLimits{MaxWindowMicros: c.WindowBudgetMicros, WindowID: c.WindowID, MaxReservations: 1024}, time.Now)
	if err != nil {
		return nil, noop, errors.New("spending ledger unavailable; explicit initialization or reconciliation is required")
	}
	fail := func(message string) (*walkthrough.Service, func() error, error) {
		ledger.Close()
		return nil, noop, errors.New(message)
	}
	engine, err := walkthrough.NewEngine(c.Engine, ledger)
	if err != nil {
		return fail("model generation settings are invalid")
	}
	client, err := githubclient.NewPublic(cfg.PublicToken, githubclient.WithUserAgent("gittok-walkthrough"))
	if err != nil {
		return fail("walkthrough public GitHub credential rejected")
	}
	verify, cancel := context.WithTimeout(ctx, 10*time.Second)
	err = client.VerifyCredential(verify)
	cancel()
	if err != nil {
		return fail("walkthrough public GitHub credential verification failed")
	}
	limits := cfg.AnalysisLimits
	limits.Timeout = 2 * time.Minute
	service, err := walkthrough.NewService(client, engine, walkthrough.ServiceConfig{Limits: limits, JournalPath: c.JournalPath, MaxJobs: 64, TTL: 30 * time.Minute})
	if err != nil {
		return fail("walkthrough journal unavailable")
	}
	cleanup := func() error {
		first := service.Close()
		second := ledger.Close()
		if first != nil {
			return first
		}
		return second
	}
	return service, cleanup, nil
}
