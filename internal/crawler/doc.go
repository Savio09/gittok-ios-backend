// Package crawler examines a repository's evidence: it classifies the project
// type (frontend / backend / library / CLI), selects the most informative
// files to read (manifests, config, entry points, docs, tests), and enforces
// the limits (file count/size, API calls, time budget) that keep analysis
// bounded even for very large repositories.
package crawler
