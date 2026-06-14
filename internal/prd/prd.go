// Package prd defines the durable PRD Issue protocol that the interactive
// Planning Stage uses to publish, revise, and track approval of one PRD
// Issue per ADR 0014.
package prd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
)

const (
	// LabelPRD marks an Issue as a Heracles PRD Issue.
	LabelPRD = "heracles:prd"
	// LabelReview marks a PRD Issue as awaiting the Planning Approval Gate.
	LabelReview = "heracles:review"
	// LabelApproved marks a PRD Issue whose Planning Approval Gate is approved.
	LabelApproved = "heracles:approved"
)

const (
	revisionPrefix = "<!-- heracles:prd-revision:"
	revisionSuffix = " -->"
)

// Approved reports whether labels mark the PRD Issue's Planning Approval
// Gate as approved.
func Approved(labels []string) bool {
	return slices.Contains(labels, LabelApproved)
}

// AwaitingReview reports whether labels mark the PRD Issue as awaiting the
// Planning Approval Gate.
func AwaitingReview(labels []string) bool {
	return slices.Contains(labels, LabelReview)
}

// Revision returns the stable revision marker for PRD Issue body content.
func Revision(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

// EmbedRevision returns body with its revision marker set to revision,
// replacing any existing marker so the same durable PRD Issue can detect
// drift across revisions.
func EmbedRevision(body, revision string) string {
	stripped := strings.TrimRight(stripRevision(body), "\n")
	return stripped + "\n\n" + revisionPrefix + revision + revisionSuffix + "\n"
}

// ExtractRevision returns the revision marker embedded in body, if present.
func ExtractRevision(body string) (string, bool) {
	start := strings.Index(body, revisionPrefix)
	if start == -1 {
		return "", false
	}
	rest := body[start+len(revisionPrefix):]
	end := strings.Index(rest, revisionSuffix)
	if end == -1 {
		return "", false
	}
	return strings.TrimSpace(rest[:end]), true
}

func stripRevision(body string) string {
	start := strings.Index(body, revisionPrefix)
	if start == -1 {
		return body
	}
	rest := body[start+len(revisionPrefix):]
	end := strings.Index(rest, revisionSuffix)
	if end == -1 {
		return body
	}
	return body[:start] + rest[end+len(revisionSuffix):]
}

// Brief is the bounded context for one interactive Grilling Session.
type Brief struct {
	ID             string
	Problem        string
	Repositories   []string
	Documents      map[string]string
	QuestionBudget int
}

// SessionBrief returns the initial prompt for the provider-owned interactive
// Planning session, combining the Grilling Session protocol (ADR 0013) and
// the durable PRD Issue protocol (ADR 0014).
func SessionBrief(brief Brief) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are the configured Heracles Planner running an interactive Grilling Session for Planning Stage %q.\n\n", brief.ID)
	fmt.Fprintf(&b, "Problem:\n%s\n\n", brief.Problem)
	if len(brief.Repositories) > 0 {
		fmt.Fprintf(&b, "Target Repositories: %s\n\n", strings.Join(brief.Repositories, ", "))
	}
	for _, path := range sortedKeys(brief.Documents) {
		fmt.Fprintf(&b, "Existing documentation %s:\n%s\n\n", path, brief.Documents[path])
	}
	fmt.Fprintf(&b, `Grilling Session protocol (ADR 0013):
- Use the grill-with-docs skill. Explore the Target Repositories and any existing documentation above.
- Ask exactly one clarifying question at a time and wait for the answer before asking the next.
- Your Question Budget is %d questions. Track how many you have asked.
- When you reach the Question Budget, stop and ask the user whether you may exceed it or should proceed directly to drafting the PRD with what you already know.

PRD Issue protocol (ADR 0014):
- Once clarification is complete, use the to-prd-for-heracles skill to draft the PRD.
- Publish the PRD as one Issue in the configured Issue Tracker, labeled %q and %q.
- Embed a SHA-256 revision marker in the Issue body so future revisions of the same Issue can detect drift.
- After publishing or revising the PRD Issue, run:
    heracles plan --id %s --prd-issue <issue-url> --prd <local-path-to-prd.md>
  using this Planning Stage ID and a local Markdown file containing the exact PRD body you published, so Heracles can hand the approved PRD to the Issue Author.
- The PRD Issue keeps one durable URL across revisions: edit the same Issue rather than creating a new one.

Approval protocol (ADR 0015):
- The user may approve inside this session. When they do, edit the PRD Issue's labels to remove %q and add %q, then run:
    heracles approve planning %s
  This records the Planning Approval Gate decision and starts background issue generation. Planning ends once this command succeeds.
- The user may instead approve through another Control Surface and tell you to continue; once you observe %q on the PRD Issue, run the same heracles approve command and finish.
- If the user requests changes instead, revise the same PRD Issue and continue the session.
`, brief.QuestionBudget, LabelPRD, LabelReview, brief.ID, LabelReview, LabelApproved, brief.ID, LabelApproved)
	return b.String()
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
