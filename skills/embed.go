// Package skills embeds the skills Heracles ships per ADR 0019: skill
// content lives alongside this file so it can be embedded into the
// `heracles` binary and installed into provider-compatible skill
// directories without depending on a checkout of this repository.
package skills

import "embed"

// FS holds the shipped skill directories, each containing a SKILL.md and any
// supporting files.
//
//go:embed grill-with-docs to-prd-for-heracles to-issues-for-heracles
var FS embed.FS
