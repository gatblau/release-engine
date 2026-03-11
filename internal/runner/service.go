package runner

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gatblau/release-engine/internal/db"
	"github.com/gatblau/release-engine/internal/registry"
)

// RunnerService executes claimed jobs, drives step lifecycle, and finalises jobs.
type RunnerService interface {
	RunJob(ctx context.Context, jobID string, runID string) error
}

type runnerService struct {
	pool     db.Pool
	stepAPI  StepAPI
	registry registry.ModuleRegistry
}

func NewRunnerService(pool db.Pool, stepAPI StepAPI, registry registry.ModuleRegistry) RunnerService {
	return &runnerService{
		pool:     pool,
		stepAPI:  stepAPI,
		registry: registry,
	}
}

func (s *runnerService) RunJob(ctx context.Context, jobID string, runID string) error {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire db connection: %w", err)
	}
	defer conn.Release()

	// 1. Fetch job definition
	var pathKey, version string
	var paramsRaw []byte
	err = conn.QueryRow(ctx, "SELECT path_key, params_json->>'version', params_json FROM jobs WHERE id = $1 AND run_id = $2", jobID, runID).Scan(&pathKey, &version, &paramsRaw)
	if err != nil {
		return fmt.Errorf("failed to fetch job: %w", err)
	}

	params := map[string]any{}
	if len(paramsRaw) > 0 {
		if err := json.Unmarshal(paramsRaw, &params); err != nil {
			return fmt.Errorf("failed to decode job params: %w", err)
		}
	}

	// 2. Resolve module from registry
	if version == "" {
		version = "latest"
	}
	module, ok := s.registry.Lookup(pathKey, version)
	if !ok {
		return fmt.Errorf("module %s:%s not found", pathKey, version)
	}

	// 3. Execute module workflow (modules orchestrate via StepAPI).
	if err := module.Execute(ctx, s.stepAPI, params); err != nil {
		return fmt.Errorf("module execution failed for %s:%s: %w", pathKey, version, err)
	}

	fmt.Printf("Job %s run %s processed successfully\n", jobID, runID)
	return nil
}
