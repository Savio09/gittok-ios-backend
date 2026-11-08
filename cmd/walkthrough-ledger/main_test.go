package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitializationRequiresExplicitBoundsAndCannotResetAccounting(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "spend.json")
	if err := initialize(path, "", 2000000); err == nil {
		t.Fatal("missing window accepted")
	}
	if err := initialize(path, "demo", 0); err == nil {
		t.Fatal("zero budget accepted")
	}
	if err := initialize(path, "demo", 2000000); err != nil {
		t.Fatal(err)
	}
	if err := initialize(path, "demo", 2000000); err == nil {
		t.Fatal("existing ledger reset")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := initialize(path, "demo", 2000000); err == nil {
		t.Fatal("lost accounting reset")
	}
}
