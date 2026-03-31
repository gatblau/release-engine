// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package bootstrap

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// discoverRepoRoot searches upward from dir for a directory containing go.mod.
// It returns the repository root (the directory containing go.mod).
func discoverRepoRoot(dir string) (string, error) {
	// If REPO_ROOT is set explicitly, use it (useful for CI/debugging).
	if repoRoot := os.Getenv("REPO_ROOT"); repoRoot != "" {
		return filepath.Abs(repoRoot)
	}

	// Walk upward looking for go.mod.
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("failed to make dir absolute: %w", err)
	}
	for {
		gomod := filepath.Join(absDir, "go.mod")
		if _, err := os.Stat(gomod); err == nil {
			return absDir, nil
		}
		parent := filepath.Dir(absDir)
		if parent == absDir {
			break
		}
		absDir = parent
	}
	return "", fmt.Errorf("repository root not found: no go.mod found by walking up from %s", dir)
}

// GiteaCLIBootstrap runs gitea-init.sh to create the admin user via Gitea CLI,
// then calls BootstrapGitea to perform all remaining setup (PAT, org, repo) in Go.
// This is the only entry point for Gitea bootstrap in the e2e test.
func GiteaCLIBootstrap(ctx context.Context, giteaURL, containerName, adminUser, adminPassword, adminEmail string) (string, error) {
	// Discover the repo root once; all paths are derived from it.
	repoRoot, err := discoverRepoRoot(".")
	if err != nil {
		return "", fmt.Errorf("failed to discover repository root: %w", err)
	}

	scriptPath := filepath.Join(repoRoot, "e2e", "scripts", "gitea-init.sh")
	scriptDir := filepath.Dir(scriptPath)
	e2eDir := scriptDir

	// Check if script exists.
	if _, err = os.Stat(scriptPath); err != nil {
		return "", fmt.Errorf("gitea-init.sh not found at %s: %w", scriptPath, err)
	}

	// Validate script path is within the project directory for security.
	relPath, err := filepath.Rel(repoRoot, scriptPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return "", fmt.Errorf("script path %s is outside project directory", scriptPath)
	}
	resolvedPath, err := filepath.EvalSymlinks(scriptPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve symlinks for %s: %w", scriptPath, err)
	}
	resolvedRel, err := filepath.Rel(repoRoot, resolvedPath)
	if err != nil || strings.HasPrefix(resolvedRel, "..") {
		return "", fmt.Errorf("resolved script path %s is outside project directory", resolvedPath)
	}
	scriptPath = resolvedPath

	// Set environment variables for the script.
	env := os.Environ()
	env = append(env, fmt.Sprintf("GITEA_URL=%s", giteaURL))
	env = append(env, fmt.Sprintf("GITEA_CONTAINER_NAME=%s", containerName))
	env = append(env, fmt.Sprintf("GITEA_ADMIN_USER=%s", adminUser))
	env = append(env, fmt.Sprintf("GITTA_ADMIN_PASSWORD=%s", adminPassword))
	env = append(env, fmt.Sprintf("GITEA_ADMIN_EMAIL=%s", adminEmail))

	// Verify the script is readable (path was already validated above).
	if _, err := os.Stat(scriptPath); err != nil {
		return "", fmt.Errorf("script not accessible at %s: %w", scriptPath, err)
	}

	// Run the script: it creates the admin user via Gitea CLI and waits for readiness.
	// All other setup (PAT, org, repo) is handled by BootstrapGitea below.
	cmd := exec.CommandContext(ctx, "bash", scriptPath) // #nosec G204 -- fixed path, no variable args
	cmd.Dir = e2eDir
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("Running Gitea CLI bootstrap script: %s\n", scriptPath)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gitea CLI bootstrap script failed: %w", err)
	}

	// Script succeeded: admin user exists and is reachable via HTTP.
	// Now BootstrapGitea creates the PAT (with RequiredScopes), org, and repo.
	fmt.Println("[gitea-cli] Shell script succeeded; handing off to Go bootstrap for PAT/org/repo setup...")
	pat, err := BootstrapGitea(ctx, giteaURL, adminUser, adminPassword)
	if err != nil {
		return "", fmt.Errorf("go bootstrap failed: %w", err)
	}

	// Write the PAT to gitea_pat.txt under the e2e directory.
	patFilePath := filepath.Join(e2eDir, "gitea_pat.txt")
	if err := os.WriteFile(patFilePath, []byte("GITEA_PAT="+pat), 0600); err != nil {
		fmt.Printf("Warning: failed to save PAT to %s: %v\n", patFilePath, err)
	}

	return pat, nil
}

// BootstrapGiteaWithCLIFallback is the top-level bootstrap entry point used by RunE2E.
// It runs GiteaCLIBootstrap (shell script + Go setup) as the primary path.
// If the shell script fails (e.g., Docker not available), it falls back to
// calling BootstrapGitea directly via the API (only works if admin user already exists).
func BootstrapGiteaWithCLIFallback(ctx context.Context, baseURL, adminUser, adminPassword string) (string, error) {
	containerName := "e2e-gitea-1"
	adminEmail := fmt.Sprintf("%s@local.dev", adminUser)

	pat, err := GiteaCLIBootstrap(ctx, baseURL, containerName, adminUser, adminPassword, adminEmail)
	if err == nil {
		return pat, nil
	}

	// CLI bootstrap failed — try API-only fallback (only works if admin already exists).
	fmt.Printf("CLI bootstrap failed: %v, falling back to API-only bootstrap\n", err)
	fmt.Println("Note: API-only bootstrap requires the admin user to already exist in Gitea.")
	fmt.Println("      This fallback is only reliable when the Docker-based CLI bootstrap succeeds.")

	return BootstrapGitea(ctx, baseURL, adminUser, adminPassword)
}
