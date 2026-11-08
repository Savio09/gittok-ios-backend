// Command walkthrough-ledger explicitly initializes a bounded spending window.
// It never resets or reconciles existing/lost accounting automatically.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Savio09/gittok-ios-backend/internal/generation"
)

func main() {
	path := flag.String("path", "", "absolute ledger path")
	window := flag.String("window", "", "operator-selected spending window ID")
	budget := flag.Int64("budget-micros", 0, "maximum USD microdollars in the window (1000000 = $1)")
	flag.Parse()
	if err := initialize(*path, *window, *budget); err != nil {
		fmt.Fprintln(os.Stderr, "Ledger initialization refused; check bounds, ownership and existing accounting.")
		os.Exit(1)
	}
	fmt.Println("Initialized bounded walkthrough spending ledger. No model calls were made.")
}
func initialize(path, window string, budget int64) error {
	if path == "" || !filepath.IsAbs(path) || window == "" || budget <= 0 || budget > 1_000_000_000 {
		return errors.New("explicit ledger bounds required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return generation.InitializeLedger(path, generation.LedgerLimits{MaxWindowMicros: budget, WindowID: window, MaxReservations: 1024}, window)
}
