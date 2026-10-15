// Command server is the entry point for the GitTok backend.
//
// It wires together the independent packages that each own one concern
// (GitHub ingestion, crawling, preview building, selection, storage, and
// serving the feed) and exposes them over HTTP to the SwiftUI app.
package main

import "fmt"

func main() {
	// The HTTP server is introduced in G2. For now this is a placeholder
	// so the module builds and the layout is in place.
	fmt.Println("gittok backend — scaffold")
}
