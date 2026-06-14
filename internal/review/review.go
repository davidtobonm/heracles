// Package review reconciles Change Sets that are awaiting manual pull
// request review when automatic merging is disabled, per ADR 0003.
package review

import (
	"context"
	"errors"

	"github.com/davidtobonm/heracles/internal/changeset"
)

// Client reports the current merge state of a pull request.
type Client interface {
	Status(context.Context, changeset.PullRequest) (changeset.PullRequestStatus, error)
}

// Outcome is the result of reconciling one Change Set's pull requests.
type Outcome struct {
	// Merged is true once every pull request in the Change Set has merged.
	Merged bool
	// PullRequests reflects each pull request's latest merged state.
	PullRequests []changeset.PullRequest
}

// Service reconciles Change Sets awaiting manual review.
type Service struct {
	Client Client
}

// Reconcile reports whether every pull request in a Change Set has merged.
func (service Service) Reconcile(ctx context.Context, set changeset.ChangeSet) (Outcome, error) {
	if service.Client == nil {
		return Outcome{}, errors.New("review reconciliation requires a Client")
	}
	pullRequests := append([]changeset.PullRequest(nil), set.PullRequests...)
	merged := true
	for index, pullRequest := range pullRequests {
		if pullRequest.Merged {
			continue
		}
		status, err := service.Client.Status(ctx, pullRequest)
		if err != nil {
			return Outcome{}, err
		}
		pullRequests[index].Merged = status.Merged
		if !status.Merged {
			merged = false
		}
	}
	return Outcome{Merged: merged, PullRequests: pullRequests}, nil
}
