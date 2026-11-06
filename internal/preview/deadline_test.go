package preview

import (
	"context"
	"testing"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

func TestExpiredAnalysisRendersUnknownWithNoFileClaims(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(1, 0))
	defer cancel()
	in := inputForTest()
	fileClaims := 0
	for _, c := range claimsIn(Build(in)) {
		if c.Source.Kind == apimodel.SourceFile {
			fileClaims++
		}
	}
	if fileClaims == 0 {
		t.Fatal("test evidence must normally support sourced file claims")
	}
	p := BuildContext(ctx, in)
	if p.Kind != apimodel.KindOther || p.KindEvidenceStatus != apimodel.EvidenceUnknown {
		t.Fatalf("unjustified role: %+v", p)
	}
	for _, s := range p.Sections {
		for _, c := range s.Claims {
			if c.Source.Kind != apimodel.SourceMetadata {
				t.Fatalf("file claim after deadline: %+v", c)
			}
		}
	}
}
