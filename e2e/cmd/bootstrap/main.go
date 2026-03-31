// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package main

import (
	"context"
	"log"
	"os"

	"github.com/gatblau/release-engine/e2e/bootstrap"
)

func main() {
	cfg, err := bootstrap.LoadE2EConfig()
	if err != nil {
		log.Fatalf("Failed to load E2E configuration: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.TestTimeout)
	defer cancel()

	result, err := bootstrap.RunE2E(ctx, *cfg)
	if err != nil {
		log.Fatalf("E2E bootstrap failed: %v", err)
	}

	log.Printf("E2E bootstrap completed successfully: job_id=%s commit_sha=%s", result.JobID, result.CommitSHA)
	os.Exit(0)
}
