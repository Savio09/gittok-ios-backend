package analysis

import (
	"context"
	"fmt"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

// Limits bounds one analysis. Values must be positive and below the validation
// ceilings, so configuration cannot accidentally restore unbounded work.
// Configure the service before sharing it with callers.
type Limits struct {
	Timeout          time.Duration
	MaxTreeEntries   int
	MaxDepth         int
	MaxFiles         int
	MaxFileBytes     int
	MaxTotalBytes    int
	MaxAPICalls      int
	MaxResponseBytes int
}

func DefaultLimits() Limits {
	return Limits{Timeout: 30 * time.Second, MaxTreeEntries: 5000, MaxDepth: 12, MaxFiles: 16, MaxFileBytes: 64 << 10, MaxTotalBytes: 512 << 10, MaxAPICalls: 80, MaxResponseBytes: githubclient.DefaultMaxResponseBytes}
}

// Validate applies finite operating ceilings as well as requiring positive caps.
func (l Limits) Validate() error {
	if l.Timeout <= 0 || l.Timeout > 2*time.Minute {
		return fmt.Errorf("analysis timeout must be between 0 and 2m (exclusive of 0)")
	}
	for _, v := range []struct {
		name       string
		value, max int
	}{
		{"tree entries", l.MaxTreeEntries, 10000}, {"tree depth", l.MaxDepth, 32}, {"files", l.MaxFiles, 40},
		{"file bytes", l.MaxFileBytes, 256 << 10}, {"total bytes", l.MaxTotalBytes, 2 << 20},
		{"API calls", l.MaxAPICalls, 128}, {"response bytes", l.MaxResponseBytes, 32 << 20},
	} {
		if v.value <= 0 || v.value > v.max {
			return fmt.Errorf("analysis %s must be between 1 and %d", v.name, v.max)
		}
	}
	return nil
}

func (s *Service) UseLimits(l Limits) error {
	if err := l.Validate(); err != nil {
		return err
	}
	s.limits = l
	s.timeout = l.Timeout
	return nil
}

// workContext starts the deadline before metadata and owns exactly one quota.
// Its quota can move to a coalesced leader's independently cancelled context;
// the absolute deadline is preserved instead of renewing a full timeout.
func (s *Service) workContext(parent context.Context) (context.Context, context.CancelFunc, *githubclient.RequestBudget, time.Time) {
	deadline := time.Now().Add(s.timeout)
	ctx, cancel := context.WithDeadline(parent, deadline)
	b := githubclient.NewRequestBudget(s.limits.MaxAPICalls, s.limits.MaxResponseBytes)
	return b.Context(ctx), cancel, b, deadline
}
