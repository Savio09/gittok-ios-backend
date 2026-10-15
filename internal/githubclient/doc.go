// Package githubclient retrieves information from the GitHub API: repository
// metadata, file trees, and file contents. It owns authentication, rate-limit
// handling, retries/backoff, and typed errors for unavailable data.
//
// Nothing else in the backend talks to GitHub directly; callers depend on this
// package's interfaces so the rest of the system is insulated from the API.
package githubclient
