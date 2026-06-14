// Package concurrency derives Scheduler serialization rules for Ready Issues
// from their dependencies, Target Repositories, conflict keys, and certainty.
package concurrency

import "github.com/davidtobonm/heracles/internal/scheduler"

// Issue is one Ready Issue's concurrency-relevant attributes.
type Issue struct {
	// Key uniquely identifies the issue within the backlog.
	Key string
	// Profile is the Agent Profile that will execute the issue, used for
	// per-profile capacity limits.
	Profile string
	// Dependencies lists the Keys of issues that must complete first.
	Dependencies []string
	// Repositories lists the Target Repositories the issue is expected to touch.
	Repositories []string
	// ConflictKeys lists project-defined keys that mark issues unsafe to run
	// concurrently (e.g. issues editing the same shared resource).
	ConflictKeys []string
	// Uncertain marks an issue whose scope could not be determined, so it
	// must run alone: serialized against every other issue.
	Uncertain bool
}

// Candidates converts issues into Scheduler Candidates whose Scopes
// serialize dependency-linked, repository-conflicting, conflict-key-sharing,
// and Uncertain issues, while leaving independent issues free to run
// concurrently.
func Candidates(issues []Issue) []scheduler.Candidate {
	candidates := make([]scheduler.Candidate, len(issues))
	for index, issue := range issues {
		issueScopes := scopes(issue)
		if issue.Uncertain {
			for _, other := range issues {
				issueScopes = append(issueScopes, issueScope(other.Key))
			}
		}
		candidates[index] = scheduler.Candidate{
			Key:          issue.Key,
			Dependencies: issue.Dependencies,
			Scopes:       dedupe(issueScopes),
			Profile:      issue.Profile,
		}
	}
	return candidates
}

// scopes returns the scopes that mark issue as conflicting with any other
// issue sharing them, including its own identity scope so that Uncertain
// issues can serialize against issues with no Repositories or ConflictKeys.
func scopes(issue Issue) []string {
	issueScopes := make([]string, 0, 1+len(issue.Repositories)+len(issue.ConflictKeys))
	issueScopes = append(issueScopes, issueScope(issue.Key))
	for _, repository := range issue.Repositories {
		issueScopes = append(issueScopes, "repository:"+repository)
	}
	for _, key := range issue.ConflictKeys {
		issueScopes = append(issueScopes, "conflict:"+key)
	}
	return issueScopes
}

func issueScope(key string) string {
	return "issue:" + key
}

func dedupe(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}
