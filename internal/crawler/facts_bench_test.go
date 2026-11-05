package crawler

import (
	"encoding/json"
	"fmt"
	"testing"
)

func BenchmarkCollectDenseManifestFacts(b *testing.B) {
	deps := make(map[string]string, 2000)
	for i := 0; i < 2000; i++ {
		deps[fmt.Sprintf("package-%04d", i)] = "^1.0.0"
	}
	content, err := json.Marshal(map[string]any{"dependencies": deps})
	if err != nil {
		b.Fatal(err)
	}
	if len(content) > 64*1024 {
		b.Fatal("benchmark escaped the production per-file cap")
	}
	insp := InspectionFromSources("", []string{"package.json"}, map[string]string{"package.json": string(content)})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if got := CollectFacts(insp); len(got) != 2000 {
			b.Fatalf("lost dense declarations: %d", len(got))
		}
	}
}
