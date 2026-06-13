package control

import (
	"context"
	"sync"

	"github.com/davidtobonm/heracles/internal/project"
)

// Dynamic lazily initializes or discovers a local project for long-lived Control Surfaces.
type Dynamic struct {
	workingDirectory string
	explicitConfig   string
	mu               sync.Mutex
	local            *Local
}

// NewDynamic creates a Control Surface that can start before project initialization.
func NewDynamic(workingDirectory, explicitConfig string) *Dynamic {
	return &Dynamic{workingDirectory: workingDirectory, explicitConfig: explicitConfig}
}

// Execute initializes a project or delegates to the discovered local application core.
func (dynamic *Dynamic) Execute(ctx context.Context, operation Operation) (Result, error) {
	dynamic.mu.Lock()
	defer dynamic.mu.Unlock()
	if operation.Name == "init" {
		initialized, err := project.Initialize(ctx, project.InitOptions{
			WorkingDirectory: dynamic.workingDirectory, ConfigPath: dynamic.explicitConfig,
			Tracker: operation.Tracker, Repositories: operation.Repositories,
		})
		if err != nil {
			return Result{}, err
		}
		if dynamic.local != nil {
			_ = dynamic.local.Close()
		}
		loaded, err := project.Load(initialized.Path)
		if err != nil {
			return Result{}, err
		}
		dynamic.local, err = NewLocal(ctx, loaded)
		return Result{Operation: operation.Name, ID: operation.ID, Status: "initialized", Data: initialized}, err
	}
	if dynamic.local == nil {
		path, err := project.Discover(dynamic.workingDirectory, dynamic.explicitConfig)
		if err != nil {
			return Result{}, err
		}
		loaded, err := project.Load(path)
		if err != nil {
			return Result{}, err
		}
		dynamic.local, err = NewLocal(ctx, loaded)
		if err != nil {
			return Result{}, err
		}
	}
	return dynamic.local.Execute(ctx, operation)
}

// Close closes any discovered local application core.
func (dynamic *Dynamic) Close() error {
	dynamic.mu.Lock()
	defer dynamic.mu.Unlock()
	if dynamic.local == nil {
		return nil
	}
	return dynamic.local.Close()
}
