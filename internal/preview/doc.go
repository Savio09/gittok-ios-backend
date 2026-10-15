// Package preview builds the evidence-grounded preview from crawled evidence.
// It produces structured claims about a repository, links each claim to the
// file or metadata it came from, and marks claims whose evidence is
// insufficient or inferred (calibrated uncertainty). It never implies a
// repository was executed when it was not.
package preview
