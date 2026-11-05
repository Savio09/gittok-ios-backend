package preview

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/crawler"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

var ownerPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`)
var repoPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}$`)

// ValidIdentity bounds a GitHub lookup to exactly one owner and repository.
func ValidIdentity(owner, name string) bool {
	return ownerPattern.MatchString(owner) && repoPattern.MatchString(name) && name != "." && name != ".."
}

func repoURL(r githubclient.Repo) string {
	return "https://github.com/" + url.PathEscape(r.Owner) + "/" + url.PathEscape(r.Name)
}
func metadataURL(r githubclient.Repo) string {
	return "https://api.github.com/repos/" + url.PathEscape(r.Owner) + "/" + url.PathEscape(r.Name)
}

func fileSource(in Input, w crawler.Witness) (apimodel.Source, string, error) {
	if !in.Inspection.Revision.Resolved || !githubclient.ValidCommitID(in.Inspection.Revision.Ref) {
		return apimodel.Source{}, "", fmt.Errorf("unresolved revision")
	}
	if w.Path == "" || path.Clean(w.Path) != w.Path || strings.HasPrefix(w.Path, "/") || !utf8.ValidString(w.Path) {
		return apimodel.Source{}, "", fmt.Errorf("invalid source path")
	}
	segments := strings.Split(w.Path, "/")
	for i, p := range segments {
		if p == "." || p == ".." || p == "" {
			return apimodel.Source{}, "", fmt.Errorf("invalid path segment")
		}
		segments[i] = url.PathEscape(p)
	}
	source := apimodel.Source{Kind: apimodel.SourceFile, Ref: w.Path, URL: repoURL(in.Repo) + "/blob/" + in.Inspection.Revision.Ref + "/" + strings.Join(segments, "/")}
	var content string
	found := false
	for _, f := range in.Inspection.Files {
		if f.Path == w.Path {
			content = f.Content
			found = true
			break
		}
	}
	if !found {
		return apimodel.Source{}, "", fmt.Errorf("unread file")
	}
	// Span witnesses and field witnesses carrying a crawler-verified declaration
	// location are rendered with lines; other fields cite the whole file.
	spanned := w.Kind == crawler.WitnessSpan || (w.Kind == crawler.WitnessField && (w.Start != 0 || w.End != 0))
	if !spanned {
		return source, "", nil
	}
	if w.Start < 0 || w.End <= w.Start || w.End > len(content) || content[w.Start:w.End] != w.Quote {
		return apimodel.Source{}, "", fmt.Errorf("invalid source span")
	}
	// An invalid UTF-8 prefix also makes line-oriented text presentation unsafe.
	if !utf8.ValidString(content[:w.End]) {
		return source, "", nil
	}
	start := 1 + strings.Count(content[:w.Start], "\n")
	end := 1 + strings.Count(content[:w.End-1], "\n")
	source.Lines = strconv.Itoa(start) + "-" + strconv.Itoa(end)
	source.URL += "#L" + strconv.Itoa(start) + "-L" + strconv.Itoa(end)
	quote := content[w.Start:w.End]
	if len(quote) > 1024 {
		quote = ""
	}
	return source, quote, nil
}
