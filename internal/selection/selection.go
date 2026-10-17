package selection

// Slug identifies a repository by its owner and name.
type Slug struct {
	Owner string
	Name  string
}

// Curated is the Sprint-1 feed: a fixed, hand-picked set of real repositories
// spanning a frontend app, a backend, and CLI tools — all active and
// contributor-friendly. Personalized ranking from the user's profile replaces
// this later; for now the feed is exactly this list, in this order.
func Curated() []Slug {
	return []Slug{
		{Owner: "excalidraw", Name: "excalidraw"}, // TypeScript frontend
		{Owner: "pocketbase", Name: "pocketbase"}, // Go backend, local-first
		{Owner: "cli", Name: "cli"},               // Go dev tool / CLI
		{Owner: "starship", Name: "starship"},     // Rust CLI
	}
}
