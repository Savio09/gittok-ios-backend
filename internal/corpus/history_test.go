package corpus

import (
	"crypto/sha256"
	"fmt"
	"os"
	"testing"
)

// Historical captures are immutable evidence, including their original ordered
// selections. New selector replays must not silently rewrite this history.
func TestHistoricalAcquisitionCapturesRemainUnchanged(t *testing.T) {
	captured := map[string]string{
		"../../internal/crawler/testdata/corpus/Savio09__plantly.json":          "85f67eb09d894cb6de0ede0771c4a535c92d332ef7be4964f47b51159d4f5d97",
		"../../internal/crawler/testdata/corpus/Savio09__face-recognition.json": "f6c94860c9b81f2f74752cdcaada12ac5db224e031080ad73140707423fad2d5",
		"../../internal/crawler/testdata/corpus/cli__cli.json":                  "1a69d26f33edd0eb9cb95c5ae4d767f443a52abfe9d0c26ec091a2a0c38d7ca7",
		"../../internal/crawler/testdata/corpus/starship__starship.json":        "ac4e3739f96bcf9b3325c852a4d8cb1c4d6bd59b35206d5974185ab64e0e4e20",
		"../../internal/crawler/testdata/corpus/excalidraw__excalidraw.json":    "a1ac438d4f33eec36358988a43637c6c3e1ea529f0f9edf8da7e6bbe12a0771f",
		"../../internal/crawler/testdata/corpus/pocketbase__pocketbase.json":    "3956162534ee5dc600f5cd4155eb871e558fc396dffd0d240dd969c1b9df6cfb",
		"../../internal/analysis/testdata/g17/charmbracelet__glow.json":         "3658fe099090dc3ca646e0f7ec265375047e838bc518b42a10577e5f556541ee",
		"../../internal/analysis/testdata/g17/rack__rack.json":                  "0696f776aafb85a846d479a1dfc5dcde2fbff8ed6183ee824a77d25d38cfca76",
	}
	for path, want := range captured {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != want {
			t.Errorf("historical capture changed: %s; preserve it and add a separately provenance-labelled capture", path)
		}
	}
}
