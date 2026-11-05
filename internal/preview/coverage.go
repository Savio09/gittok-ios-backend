package preview

import "github.com/Savio09/gittok-ios-backend/internal/crawler"

// Coverage is the acquisition state behind a preview's reader-facing coverage
// notes. The analysis layer sets it from its own outcome at the point each gap
// arises; nothing is inferred from diagnostic prose. Detailed causes (paths,
// provider and parser reasons) stay in internal results, not in the preview.
type Coverage struct {
	// IdentityUnverified: the repository's identity could not be reconfirmed
	// after acquisition, so file evidence was withheld.
	IdentityUnverified bool
	// FilesNotAcquired: no file contents were acquired (no default branch,
	// unpinned revision, no tree, or the deadline arrived first).
	FilesNotAcquired bool
	// ContentsIncomplete: selected contents could not all be acquired.
	ContentsIncomplete bool
	// SymlinksExcluded: symlinked paths were listed but never read.
	SymlinksExcluded bool
}

// Reader-facing notes, most material first. Each states a gap in what was
// inspected; none claims that something is absent from the repository.
const (
	noteIdentity = "GitHub could not reconfirm this repository's identity after it was read, so file evidence was withheld."
	noteMetadata = "File contents could not be inspected, so this preview shows GitHub metadata only."
	noteNoneRead = "The selected files could not be read, so this preview shows GitHub metadata only."
	noteUnread   = "Some files could not be read in full, so details may be missing."
	notePartTree = "This preview covers part of a large repository."
	noteKind     = "The project type could not be confirmed from the inspected files."
	noteOmitted  = "Lower-priority declarations are not shown."
	maxNotes     = 3
)

// coverageNotes summarizes what the preview could not cover, at most three
// short statements, from typed acquisition, inspection and verdict state.
func coverageNotes(in Input, cls crawler.Classification, explained bool, omitted int) []string {
	insp := in.Inspection
	unread := in.Coverage.ContentsIncomplete || in.Coverage.SymlinksExcluded || len(insp.Unavailable) > 0
	for _, f := range insp.Files {
		unread = unread || f.Truncated
	}
	var notes []string
	add := func(cond bool, note string) {
		if cond && len(notes) < maxNotes {
			notes = append(notes, note)
		}
	}
	noFiles := len(insp.Files) == 0 || !insp.Revision.Resolved
	add(in.Coverage.IdentityUnverified, noteIdentity)
	noneRead := noFiles && insp.Revision.Resolved && !in.Coverage.FilesNotAcquired && len(insp.Unavailable) > 0
	add(!in.Coverage.IdentityUnverified && noneRead, noteNoneRead)
	add(!in.Coverage.IdentityUnverified && !noneRead && (in.Coverage.FilesNotAcquired || noFiles), noteMetadata)
	add(unread && !noFiles, noteUnread)
	add(insp.Scope.Truncated || insp.Scope.DepthLimited, notePartTree)
	add(!noFiles && (cls.Status != crawler.VerdictResolved || !explained), noteKind)
	add(omitted > 0, noteOmitted)
	return notes
}
