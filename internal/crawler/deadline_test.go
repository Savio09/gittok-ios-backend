package crawler

import (
	"context"
	"fmt"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

// Cancellation becomes visible at a controlled checkpoint, without wall time.
type checkpointContext struct {
	context.Context
	checks, stop int
}

func (c *checkpointContext) Err() error {
	c.checks++
	if c.checks >= c.stop {
		return context.DeadlineExceeded
	}
	return nil
}

func TestClassificationAndFactsStopDuringLargeInventory(t *testing.T) {
	insp := Inspection{Language: "JavaScript"}
	for i := 0; i < 10000; i++ {
		insp.Paths = append(insp.Paths, fmt.Sprintf("packages/%06d/package.json", i))
	}
	ctx := &checkpointContext{Context: context.Background(), stop: 50}
	cls := ClassifyContext(ctx, insp)
	if cls.Kind != apimodel.KindOther || cls.Status != VerdictInconclusive || ctx.checks > 55 {
		t.Fatalf("classification=%+v checkpoints=%d", cls, ctx.checks)
	}
	ctx = &checkpointContext{Context: context.Background(), stop: 50}
	if facts := CollectFactsContext(ctx, insp); len(facts) != 0 || ctx.checks > 55 {
		t.Fatalf("facts=%d checkpoints=%d", len(facts), ctx.checks)
	}
	ctx = &checkpointContext{Context: context.Background(), stop: 50}
	if imports := CollectImportsContext(ctx, insp); len(imports) != 0 || ctx.checks > 55 {
		t.Fatalf("imports=%d checkpoints=%d", len(imports), ctx.checks)
	}
}

func TestComponentDeadlineStopsLaterManifestAndGoParsers(t *testing.T) {
	insp := InspectionFromSources("Go", []string{"package.json", "go.mod", "a.go", "b.go"}, map[string]string{
		"package.json": `{"name":"x"}`, "go.mod": "module example.com/x\n\ngo 1.25\n", "a.go": "package main\nfunc main(){}", "b.go": "package main\nfunc helper(){}",
	})
	st := indexInspection(insp)
	comp := buildInventory(st)[0]
	st.ctx = &checkpointContext{Context: context.Background(), stop: 2}
	ix := newRepoIndex(st, comp, "Go")
	if ix.pkg == nil || ix.modPath != "" || len(ix.goFiles) != 0 {
		t.Fatalf("parsers started after deadline: npm=%v module=%q Go=%d", ix.pkg != nil, ix.modPath, len(ix.goFiles))
	}
	st.ctx = &checkpointContext{Context: context.Background(), stop: 6}
	ix = newRepoIndex(st, comp, "Go")
	if len(ix.goFiles) != 1 {
		t.Fatalf("Go parsers started=%d, want only the first", len(ix.goFiles))
	}
}
