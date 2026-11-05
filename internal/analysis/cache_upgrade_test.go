package analysis

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

func TestRepairInvalidatesPersistedVersionTwoPreviews(t *testing.T) {
	clock := &fakeClock{time.Now()}
	dir := t.TempDir()
	old := openTestCache(t, dir, testLimits(), clock)
	oldKey := CacheKey{RepoID: 7, Owner: "o", Name: "r", version: 2}
	stale := apimodel.RepoPreview{ID: "o/r", Owner: "o", Name: "r", Kind: apimodel.KindBackend}
	if !old.Put(oldKey, "0123456789abcdef0123456789abcdef01234567", clock.t, stale) {
		t.Fatal("could not seed the pre-repair cache")
	}
	gh := &countingGitHub{repo: publicMeta()}
	s := NewService(gh)
	s.publicOnly = true
	s.UseCache(openTestCache(t, dir, testLimits(), clock))
	fresh, err := s.Preview(context.Background(), "o", "r")
	if err != nil {
		t.Fatal(err)
	}
	if gh.refs != 1 || gh.trees != 1 || gh.contentsN != 1 || reflect.DeepEqual(fresh, stale) {
		t.Fatalf("served pre-repair output: refs=%d trees=%d contents=%d preview=%+v", gh.refs, gh.trees, gh.contentsN, fresh)
	}
	metadata := gh.metadata
	warm, err := s.Preview(context.Background(), "o", "r")
	if err != nil || !reflect.DeepEqual(warm, fresh) || gh.metadata != metadata+1 || gh.trees != 1 {
		t.Fatalf("fresh snapshot was not reused after checking visibility: err=%v metadata=%d trees=%d", err, gh.metadata, gh.trees)
	}
}
