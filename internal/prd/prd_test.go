package prd_test

import (
	"strings"
	"testing"

	"github.com/davidtobonm/heracles/internal/prd"
)

func TestApprovedAndAwaitingReviewReadLabels(t *testing.T) {
	t.Parallel()

	if prd.Approved([]string{prd.LabelPRD, prd.LabelReview}) {
		t.Error("Approved() = true for an Issue still awaiting review")
	}
	if !prd.AwaitingReview([]string{prd.LabelPRD, prd.LabelReview}) {
		t.Error("AwaitingReview() = false, want true")
	}
	if !prd.Approved([]string{prd.LabelPRD, prd.LabelApproved}) {
		t.Error("Approved() = false, want true")
	}
	if prd.AwaitingReview([]string{prd.LabelPRD, prd.LabelApproved}) {
		t.Error("AwaitingReview() = true for an approved Issue")
	}
}

func TestEmbedAndExtractRevisionRoundTrip(t *testing.T) {
	t.Parallel()

	body := "# Product Requirements\n\nDetails."
	revision := prd.Revision(body)

	embedded := prd.EmbedRevision(body, revision)
	if !strings.HasPrefix(embedded, body) {
		t.Fatalf("EmbedRevision() = %q, want it to preserve the original body", embedded)
	}

	extracted, ok := prd.ExtractRevision(embedded)
	if !ok {
		t.Fatalf("ExtractRevision() ok = false, want true")
	}
	if extracted != revision {
		t.Errorf("ExtractRevision() = %q, want %q", extracted, revision)
	}
}

func TestEmbedRevisionReplacesExistingMarker(t *testing.T) {
	t.Parallel()

	body := "# Product Requirements"
	first := prd.EmbedRevision(body, prd.Revision(body))

	revised := "# Product Requirements\n\nRevised."
	second := prd.EmbedRevision(first, prd.Revision(revised))

	extracted, ok := prd.ExtractRevision(second)
	if !ok {
		t.Fatalf("ExtractRevision() ok = false, want true")
	}
	if extracted != prd.Revision(revised) {
		t.Errorf("ExtractRevision() = %q, want %q", extracted, prd.Revision(revised))
	}
	if strings.Count(second, "heracles:prd-revision:") != 1 {
		t.Errorf("EmbedRevision() left %d markers, want 1: %q", strings.Count(second, "heracles:prd-revision:"), second)
	}
}

func TestSessionBriefDescribesTheCompleteProtocol(t *testing.T) {
	t.Parallel()

	brief := prd.SessionBrief(prd.Brief{
		ID:             "labor-1",
		Problem:        "Build a water tracking app",
		Repositories:   []string{"app"},
		Documents:      map[string]string{"README.md": "Existing notes"},
		QuestionBudget: 20,
	})

	for _, want := range []string{
		"labor-1",
		"Build a water tracking app",
		"Target Repositories: app",
		"README.md",
		"Existing notes",
		"grill-with-docs",
		"to-prd",
		"one clarifying question at a time",
		"Question Budget is 20",
		"exceed it or should proceed",
		prd.LabelPRD,
		prd.LabelReview,
		prd.LabelApproved,
		"heracles plan --id labor-1 --prd-issue <issue-url> --prd <local-path-to-prd.md>",
		"heracles approve planning labor-1",
		"SHA-256 revision marker",
	} {
		if !strings.Contains(brief, want) {
			t.Errorf("SessionBrief() missing %q\n%s", want, brief)
		}
	}
}
