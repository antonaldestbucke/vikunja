// Vikunja is a to-do list application to facilitate your life.
// Copyright 2018-present Vikunja and contributors. All rights reserved.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Package e2e is the integration suite for veans. It assumes a running
// Vikunja API at VEANS_E2E_API_URL and admin/seed credentials in
// VEANS_E2E_ADMIN_TOKEN (or VEANS_E2E_ADMIN_USER + VEANS_E2E_ADMIN_PASS).
//
// The suite never provisions Vikunja itself — locally, point it at a dev
// instance; in CI, the workflow spins one up the same way the frontend
// Playwright suite does.
package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"code.vikunja.io/veans/internal/client"
)

// Harness bundles a built veans binary and an authenticated admin client
// for verifying side effects on the server.
type Harness struct {
	Binary      string
	APIURL      string
	AdminToken  string
	AdminClient *client.Client
}

// SkipIfNotConfigured calls t.Skip if the suite hasn't been pointed at a
// Vikunja instance. Intended for the top of TestMain / TestXxx so plain
// `go test ./...` doesn't fail on contributors who haven't set up the env.
func SkipIfNotConfigured(t *testing.T) {
	t.Helper()
	if os.Getenv("VEANS_E2E_API_URL") == "" {
		t.Skip("VEANS_E2E_API_URL not set — skipping e2e")
	}
}

// New builds (or reuses) the veans binary, mints/loads an admin token via
// the env, and returns a Harness ready to drive tests.
func New(t *testing.T) *Harness {
	t.Helper()
	SkipIfNotConfigured(t)

	apiURL := strings.TrimRight(os.Getenv("VEANS_E2E_API_URL"), "/")
	binary, err := buildOrLocate()
	if err != nil {
		t.Fatalf("locate veans binary: %v", err)
	}

	tok := os.Getenv("VEANS_E2E_ADMIN_TOKEN")
	if tok == "" {
		user, pass := os.Getenv("VEANS_E2E_ADMIN_USER"), os.Getenv("VEANS_E2E_ADMIN_PASS")
		if user == "" || pass == "" {
			t.Fatal("set VEANS_E2E_ADMIN_TOKEN or VEANS_E2E_ADMIN_USER + VEANS_E2E_ADMIN_PASS")
		}
		c := client.New(apiURL, "")
		resp, err := c.Login(t.Context(), &client.LoginRequest{
			Username: user, Password: pass, LongToken: true,
		})
		if err != nil {
			t.Fatalf("admin login: %v", err)
		}
		tok = resp.Token
	}

	return &Harness{
		Binary:      binary,
		APIURL:      apiURL,
		AdminToken:  tok,
		AdminClient: client.New(apiURL, tok),
	}
}

// Workspace creates a per-test git repo in a TempDir with HOME pointed at
// a TempDir so the credential store writes under the test's own directory
// rather than touching the developer's keychain.
type Workspace struct {
	Dir          string
	Home         string
	ConfigPath   string
	BotUsername  string
	envOverrides map[string]string
}

// NewWorkspace initializes a fresh repo with `git init` + a single commit so
// `git rev-parse --abbrev-ref HEAD` returns a real branch name.
func (h *Harness) NewWorkspace(t *testing.T) *Workspace {
	t.Helper()
	dir := t.TempDir()
	home := t.TempDir()

	for _, c := range [][]string{
		{"git", "init", "-q", "-b", "main"},
		{"git", "config", "user.email", "veans-e2e@example.com"},
		{"git", "config", "user.name", "veans-e2e"},
		// Disable any inherited commit signing; the test commit doesn't
		// need provenance and signing brokers can fail in dev containers.
		{"git", "config", "commit.gpgsign", "false"},
		{"git", "config", "tag.gpgsign", "false"},
	} {
		cmd := exec.CommandContext(t.Context(), c[0], c[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s: %v\n%s", strings.Join(c, " "), err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range [][]string{
		{"git", "add", "."},
		{"git", "commit", "-q", "-m", "initial"},
	} {
		cmd := exec.CommandContext(t.Context(), c[0], c[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s: %v\n%s", strings.Join(c, " "), err, out)
		}
	}

	return &Workspace{
		Dir:        dir,
		Home:       home,
		ConfigPath: filepath.Join(dir, ".veans.yml"),
		envOverrides: map[string]string{
			"HOME": home,
		},
	}
}

// Run executes the veans binary against this workspace, returning stdout,
// stderr, and exit code.
func (h *Harness) Run(t *testing.T, ws *Workspace, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), h.Binary, args...)
	cmd.Dir = ws.Dir
	// Filter VEANS_* out of the inherited env before applying our
	// overrides — a developer's VEANS_TOKEN would otherwise mask the
	// per-test bot token via the env backend.
	cmd.Env = append(filterEnv(os.Environ(), "VEANS_"), envSlice(ws.envOverrides)...)
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return so.String(), se.String(), ee.ExitCode()
		}
		t.Fatalf("run veans %v: %v", args, err)
	}
	return so.String(), se.String(), 0
}

// CreateProject creates a fresh project owned by the admin user and returns
// it. Tests use a unique title to keep results isolated across parallel runs.
func (h *Harness) CreateProject(t *testing.T, title, identifier string) *client.Project {
	t.Helper()
	out, err := h.AdminClient.CreateProject(t.Context(),
		&client.Project{Title: title, Identifier: identifier})
	if err != nil {
		t.Fatalf("create project %q: %v", title, err)
	}
	return out
}

// FindKanbanView returns the first Kanban view of the project (Vikunja
// auto-creates one).
func (h *Harness) FindKanbanView(t *testing.T, projectID int64) *client.ProjectView {
	t.Helper()
	views, err := h.AdminClient.ListProjectViews(t.Context(), projectID)
	if err != nil {
		t.Fatalf("list views: %v", err)
	}
	for _, v := range views {
		if v.ViewKind == client.ViewKindKanban {
			return v
		}
	}
	t.Fatalf("no Kanban view on project %d", projectID)
	return nil
}

// GetTask fetches a task by ID for verification.
func (h *Harness) GetTask(t *testing.T, id int64) *client.Task {
	t.Helper()
	task, err := h.AdminClient.GetTask(t.Context(), id)
	if err != nil {
		t.Fatalf("get task %d: %v", id, err)
	}
	return task
}

func buildOrLocate() (string, error) {
	if env := os.Getenv("VEANS_BINARY"); env != "" {
		if abs, err := filepath.Abs(env); err == nil {
			if _, err := os.Stat(abs); err == nil {
				return abs, nil
			}
		}
	}
	tmp, err := os.MkdirTemp("", "veans-bin-*")
	if err != nil {
		return "", err
	}
	bin := filepath.Join(tmp, "veans")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.CommandContext(context.Background(), "go", "build", "-o", bin, "./cmd/veans")
	cmd.Dir = repoRoot()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("build veans: %w (output: %s)", err, out)
	}
	return bin, nil
}

func repoRoot() string {
	// e2e/helpers.go lives at <repo>/veans/e2e/helpers.go, so go up two.
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
}

func envSlice(overrides map[string]string) []string {
	out := make([]string, 0, len(overrides))
	for k, v := range overrides {
		out = append(out, k+"="+v)
	}
	return out
}

// filterEnv returns env entries whose keys do NOT start with prefix.
func filterEnv(env []string, prefix string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if !strings.HasPrefix(kv, prefix) {
			out = append(out, kv)
		}
	}
	return out
}
