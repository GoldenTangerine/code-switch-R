/**
 * @name: CLI 更新查询测试
 * @Descripttion: 验证本机 CLI 探测、发布源响应与更新状态。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-20 14:40:19
 * @LastEditTime: 2026-11-20 14:40:19
 * @FilePath: services/cliupdateservice_test.go
 */
package services

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type cliReleaseTransport func(*http.Request) (*http.Response, error)

func (transport cliReleaseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return transport(req)
}

func TestCheckCLIVersion(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	source := cliReleaseSources[0]
	tests := []struct {
		name, output, release, wantStatus, wantInstalled, wantLatest string
		httpStatus                                                   int
	}{
		{"update", "2.1.9 (Claude Code)", `{"latest":"2.1.10"}`, "updateAvailable", "2.1.9", "2.1.10", 200},
		{"equal", "2.1.10 (Claude Code)", `{"latest":"2.1.10"}`, "upToDate", "2.1.10", "2.1.10", 200},
		{"ahead", "2.1.11 (Claude Code)", `{"latest":"2.1.10"}`, "upToDate", "2.1.11", "2.1.10", 200},
		{"stable", "2.1.10-rc.1", `{"latest":"2.1.10","next":"2.1.11-rc.1"}`, "updateAvailable", "2.1.10-rc.1", "2.1.10", 200},
		{"preview", "2.1.11-rc.2", `{"latest":"2.1.10","next":"2.1.11-rc.10"}`, "updateAvailable", "2.1.11-rc.2", "2.1.11-rc.10", 200},
		{"missing command", "", `{"latest":"2.1.10"}`, "notDetected", "", "2.1.10", 200},
		{"bad command output", "version unknown", `{"latest":"2.1.10"}`, "localUnavailable", "", "2.1.10", 200},
		{"rate limit", "2.1.9", `{}`, "latestUnavailable", "2.1.9", "", 429},
		{"short remote version", "2.1.9", `{"latest":"2.1"}`, "latestUnavailable", "2.1.9", "", 200},
		{"dev build", "2.1.9+238.g456def", `{"latest":"2.1.10"}`, "uncomparable", "2.1.9+238.g456def", "2.1.10", 200},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "CLI tools")
			if err := os.Mkdir(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if tt.output != "" {
				filename := source.command
				script := "#!/bin/sh\ntest \"$1\" = \"--version\" || exit 1\nprintf '%s\\n' '" + tt.output + "'\n"
				if runtime.GOOS == "windows" {
					filename += ".cmd"
					script = "@echo off\r\nif not \"%1\"==\"--version\" exit /b 1\r\necho " + tt.output + "\r\n"
				}
				if err := os.WriteFile(filepath.Join(dir, filename), []byte(script), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			client := &http.Client{Transport: cliReleaseTransport(func(req *http.Request) (*http.Response, error) {
				if req.URL.String() != "https://registry.npmjs.org/-/package/@anthropic-ai%2fclaude-code/dist-tags" {
					t.Fatalf("unexpected URL: %s", req.URL)
				}
				return &http.Response{StatusCode: tt.httpStatus, Body: io.NopCloser(strings.NewReader(tt.release))}, nil
			})}
			got := checkCLIVersion(t.Context(), source, cliPaths{inherited: []string{dir}}, client, "")
			if got.Status != tt.wantStatus || got.InstalledVersion != tt.wantInstalled || got.LatestVersion != tt.wantLatest {
				t.Fatalf("got %+v, want status=%s, installed=%s, latest=%s", got, tt.wantStatus, tt.wantInstalled, tt.wantLatest)
			}
		})
	}
}

func TestCLIInstallPaths(t *testing.T) {
	home := t.TempDir()
	pnpm := filepath.Join(home, "pnpm global")
	grok := filepath.Join(home, ".grok", "bin")
	t.Setenv("PATH", "")
	t.Setenv("PNPM_HOME", pnpm)
	t.Setenv("GROK_BIN_DIR", "")
	paths := cliSearchPaths(home)
	codexSource, grokSource := cliReleaseSources[1], cliReleaseSources[5]
	writeCLIVersionFixture(t, pnpm, codexSource.command)
	writeCLIVersionFixture(t, grok, grokSource.command)
	if version, found := probeCLIVersion(t.Context(), codexSource, paths); !found || version != "1.2.3" {
		t.Fatalf("pnpm install: found=%t version=%q", found, version)
	}
	paths.fallback = append(grokCLIFallbackPaths(home), paths.fallback...)
	if version, found := probeCLIVersion(t.Context(), grokSource, paths); !found || version != "1.2.3" {
		t.Fatalf("Grok native install: found=%t version=%q", found, version)
	}
}

func writeCLIVersionFixture(t *testing.T, dir, command string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\necho 1.2.3\n"
	if runtime.GOOS == "windows" {
		command += ".cmd"
		script = "@echo off\r\necho 1.2.3\r\n"
	}
	if err := os.WriteFile(filepath.Join(dir, command), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestCLIProbePreservesInheritedPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses a POSIX Node launcher")
	}
	nodeDir, launcherDir := t.TempDir(), t.TempDir()
	for path, script := range map[string]string{
		filepath.Join(nodeDir, "node"):       "#!/bin/sh\necho codex-cli 0.170.0\n",
		filepath.Join(launcherDir, "node"):   "#!/bin/sh\nexit 1\n",
		filepath.Join(launcherDir, "codex"):  "#!/usr/bin/env node\n",
		filepath.Join(launcherDir, "claude"): "#!/bin/sh\necho 'Using Node 22.20.0' >&2\n",
	} {
		if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	paths := cliPaths{inherited: []string{nodeDir, launcherDir}}
	if version, found := probeCLIVersion(t.Context(), cliReleaseSources[1], paths); !found || version != "0.170.0" {
		t.Fatalf("PATH precedence: found=%t version=%q", found, version)
	}
	if version, found := probeCLIVersion(t.Context(), cliReleaseSources[0], paths); !found || version != "" {
		t.Fatalf("diagnostic output: found=%t version=%q", found, version)
	}
}

func TestCLIRecognizesVersionOutput(t *testing.T) {
	for _, tt := range []struct {
		source    cliReleaseSource
		out, want string
	}{
		{cliReleaseSources[1], "Using Node 22.20.0\ncodex-cli 0.172.0-alpha.2", "0.172.0-alpha.2"},
		{cliReleaseSources[2], "Antigravity CLI version: 1.5.13", "1.5.13"},
		{cliReleaseSources[3], "v0.71.0-preview.0", "0.71.0-preview.0"},
		{cliReleaseSources[4], "[warning] npm 11.0.0\n1.19.6", "1.19.6"},
		{cliReleaseSources[5], "grok 1.2.3", "1.2.3"},
		{cliReleaseSources[6], "OpenClaw 2026.11.6 (760d8f2)", "2026.11.6"},
		{cliReleaseSources[7], "Hermes Agent v0.22.1+238.g8211bb9 (2026.9.24) · upstream 8211bb96", "0.22.1+238.g8211bb9"},
		{cliReleaseSources[1], "Node.js version 22.20.0", ""},
		{cliReleaseSources[1], "codex-cli requires Node 22.20.0", ""},
	} {
		if got := tt.source.versionFromOutput(tt.out); got != tt.want {
			t.Errorf("%s output %q: got %q, want %q", tt.source.command, tt.out, got, tt.want)
		}
	}
}

func TestCLICheckUsesClaudeStableChannel(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, claudeSettingsFileName), []byte(`{"autoUpdatesChannel":"stable"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	claude := cliReleaseSources[0]
	writeCLIVersionFixture(t, dir, claude.command)
	client := &http.Client{Transport: cliReleaseTransport(func(req *http.Request) (*http.Response, error) {
		body := `{"stable":"1.2.3","latest":"1.3.0","next":"1.4.0-rc.1"}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	got := checkCLIVersion(t.Context(), claude, cliPaths{inherited: []string{dir}}, client, "")
	if got.Status != "upToDate" || got.LatestVersion != "1.2.3" {
		t.Fatalf("stable channel: %+v", got)
	}
}

func TestCLILatestUsesPrereleaseChannel(t *testing.T) {
	for _, tt := range []struct {
		source          cliReleaseSource
		release         map[string]any
		installed, want string
	}{
		{cliReleaseSources[1], map[string]any{"latest": "0.171.0", "alpha": "0.173.0-alpha.1"}, "0.172.0-alpha.2", "0.173.0-alpha.1"},
		{cliReleaseSources[1], map[string]any{"latest": "0.171.0", "alpha": "0.173.0-alpha.1"}, "0.171.0", "0.171.0"},
		{cliReleaseSources[3], map[string]any{"latest": "0.70.0", "preview": "0.71.0-preview.0", "nightly": "0.72.0-nightly.1"}, "0.70.0-preview.0", "0.71.0-preview.0"},
		{cliReleaseSources[3], map[string]any{"latest": "0.70.0", "preview": "0.71.0-preview.0", "nightly": "0.72.0-nightly.1"}, "0.71.0-nightly.1", "0.72.0-nightly.1"},
	} {
		if got := tt.source.selectLatest(tt.release, tt.installed, false); got != tt.want {
			t.Errorf("%s at %s: got %q, want %q", tt.source.command, tt.installed, got, tt.want)
		}
	}
}

func TestCLINativeReleaseSources(t *testing.T) {
	if got := antigravityManifestURL("darwin", "arm64"); !strings.HasSuffix(got, "/manifests/darwin_arm64.json") {
		t.Fatalf("unexpected Antigravity manifest URL: %q", got)
	}
	client := &http.Client{Transport: cliReleaseTransport(func(req *http.Request) (*http.Response, error) {
		var body string
		switch req.URL.Host {
		case "antigravity-cli-auto-updater-974169037036.us-central1.run.app":
			body = `{"version":"1.5.13"}`
		case "api.github.com":
			body = `{"tag_name":"v2026.8.31","name":"Hermes Agent v0.21.0 (v2026.8.31)"}`
		default:
			t.Fatalf("unexpected release URL: %s", req.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	for _, tt := range []struct{ command, want string }{{"agy", "1.5.13"}, {"hermes", "0.21.0"}} {
		source := cliReleaseSource{command: tt.command}
		if got := source.selectLatest(source.fetchRelease(t.Context(), client), "", false); got != tt.want {
			t.Errorf("%s returned %q, want %q", tt.command, got, tt.want)
		}
	}
}
