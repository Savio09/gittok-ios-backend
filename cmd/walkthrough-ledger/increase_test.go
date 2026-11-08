package main

import (
	"flag"
	"github.com/Savio09/gittok-ios-backend/internal/generation"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestExplicitIncreaseDoesNotInitializeOrResetWindow(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "spend.json")
	if err := increase(path, "window", 1_000_000, 2_000_000); err == nil {
		t.Fatal("increase initialized missing accounting")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("failed increase created accounting")
	}
	if err := initialize(path, "window", 1_000_000); err != nil {
		t.Fatal(err)
	}
	old := generation.LedgerLimits{WindowID: "window", MaxWindowMicros: 1_000_000}
	l, err := generation.OpenLedger(path, old, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = l.Reserve("unknown", "call", 600_000); err != nil {
		t.Fatal(err)
	}
	if err = l.Close(); err != nil {
		t.Fatal(err)
	}
	if err = increase(path, "window", 1_000_000, 2_000_000); err != nil {
		t.Fatal(err)
	}
	old.MaxWindowMicros = 2_000_000
	reopened, err := generation.OpenLedger(path, old, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err = reopened.Reserve("new", "call", 1_400_001); err == nil {
		t.Fatal("increase discarded the unknown charge")
	}
}

func TestExplicitZeroIncreaseFlagCannotInitializeMissingLedger(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(os.Args[0], "-test.run=^TestLedgerIncreaseCLIHelper$", "--", "-path", filepath.Join(dir, "spend.json"), "-window", "window", "-budget-micros", "2000000", "-increase-from-micros", "0")
	command.Env = append(os.Environ(), "GITTOK_LEDGER_CLI_HELPER=1")
	if err := command.Run(); err == nil {
		t.Fatal("explicit invalid increase fell through to initialization")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatal("invalid increase created accounting or a marker", err)
	}
}

func TestLedgerIncreaseCLIHelper(t *testing.T) {
	if os.Getenv("GITTOK_LEDGER_CLI_HELPER") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"walkthrough-ledger"}, os.Args[i+1:]...)
			break
		}
	}
	flag.CommandLine = flag.NewFlagSet("walkthrough-ledger", flag.ExitOnError)
	main()
}
