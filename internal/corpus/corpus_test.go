package corpus

import (
	"path/filepath"
	"reflect"
	"testing"
)

// The secrets guard withholds credential-like files and content, while ordinary
// manifests and example env files pass.
func TestSecretish(t *testing.T) {
	secret := map[string]string{
		".env":                    "PORT=8080\n",
		"config/.env.production":  "X=1\n",
		"deploy/server.pem":       "x",
		"keys/id_ed25519":         "x",
		".npmrc":                  "//registry:_authToken=x\n",
		"main.go (token content)": "const t = \"ghp_" + "abcdefghijklmnopqrstuvwxyz0123456789\"\n",
		"cfg.yml (aws key)":       "key: AKIA" + "ABCDEFGHIJKLMNOP\n",
		"k.txt (private key)":     "-----BEGIN RSA PRIVATE KEY-----\n",
	}
	for p, c := range secret {
		if !Secretish(p, c) {
			t.Errorf("%s was not withheld", p)
		}
	}
	safe := map[string]string{
		"package.json": `{"name":"x"}`,
		".env.example": "PORT=8080\n",
		"go.mod":       "module x\n",
		"README.md":    "Set GITHUB_TOKEN in your environment.\n",
	}
	for p, c := range safe {
		if Secretish(p, c) {
			t.Errorf("%s was wrongly withheld", p)
		}
	}
}

// Fetched reports what the capture saw — including paths it never requested —
// and a snapshot survives a save/load round trip unchanged.
func TestSnapshotFetchedAndRoundTrip(t *testing.T) {
	s := Snapshot{
		Repo: "o/r", Selected: []string{"go.mod", "main.go"},
		Extras:  map[string]string{"cmd/x.go": "decisive", "go.mod": "already selected"},
		Files:   map[string]File{"go.mod": {Content: "module x\n", Size: 9}, "cmd/x.go": {Content: "package x\n", Size: 10}},
		Skipped: map[string]string{"main.go": "not found"},
	}
	if got, want := s.AllRequested(), []string{"go.mod", "main.go", "cmd/x.go"}; !reflect.DeepEqual(got, want) {
		t.Errorf("AllRequested = %v, want %v", got, want)
	}
	f := s.Fetched([]string{"go.mod", "main.go", "never.go"})
	if len(f.Files) != 1 || len(f.Skipped) != 2 || f.Skipped[0].Reason != "not found" || f.Skipped[1].Reason != "not captured" {
		t.Errorf("Fetched = %+v", f)
	}

	file := filepath.Join(t.TempDir(), "s.json")
	if err := s.Save(file); err != nil {
		t.Fatal(err)
	}
	back, err := Load(file)
	if err != nil || !reflect.DeepEqual(back, s) {
		t.Errorf("round trip changed the snapshot: %+v, %v", back, err)
	}
}

// A captured listing names the GetTree request cmd/corpuscapture made at the
// recorded commit; without a valid recorded commit no request is claimed.
func TestSnapshotTreeNamesItsCapturedRequest(t *testing.T) {
	s := Snapshot{Repo: "o/r", ResolvedSHA: "0123456789abcdef0123456789abcdef01234567"}
	if got := s.FileTree().Request; got != "/repos/o/r/git/trees/0123456789abcdef0123456789abcdef01234567?recursive=1" {
		t.Errorf("Request = %q", got)
	}
	s.ResolvedSHA = "main"
	if got := s.FileTree().Request; got != "" {
		t.Errorf("an unpinned capture claimed request %q", got)
	}
}
