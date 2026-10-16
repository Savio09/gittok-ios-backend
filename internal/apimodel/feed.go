package apimodel

// Feed is a page of the vertical feed: an ordered list of repository previews
// plus an opaque cursor for the next page. The cursor is empty on the last
// page. Ordering is the backend's job (curated now, personalized later); the
// app renders items in the order given.
type Feed struct {
	Items      []RepoPreview `json:"items"`
	NextCursor string        `json:"next_cursor,omitempty"`
}
