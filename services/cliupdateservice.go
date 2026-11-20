/**
 * @name: CLI 更新查询服务
 * @Descripttion: 读取本机 CLI 版本并与公开发布源比较。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-20 14:40:19
 * @LastEditTime: 2026-11-20 14:40:19
 * @FilePath: services/cliupdateservice.go
 */
package services

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/mod/semver"
)

type CLIUpdateService struct{}

type CLIVersionInfo struct {
	Command          string `json:"command"`
	Name             string `json:"name"`
	InstalledVersion string `json:"installedVersion"`
	LatestVersion    string `json:"latestVersion"`
	Status           string `json:"status"`
}

type cliReleaseSource struct {
	command        string
	name           string
	npmPackage     string
	prereleaseTags []string
}

type cliPaths struct {
	inherited []string
	fallback  []string
}

var cliReleaseSources = []cliReleaseSource{
	{command: "claude", name: "Claude Code", npmPackage: "@anthropic-ai/claude-code"},
	{command: "codex", name: "Codex", npmPackage: "@openai/codex", prereleaseTags: []string{"alpha", "beta", "rc"}},
	{command: "agy", name: "Antigravity CLI"},
	{command: "gemini", name: "Gemini CLI", npmPackage: "@google/gemini-cli", prereleaseTags: []string{"preview", "nightly"}},
	{command: "opencode", name: "OpenCode", npmPackage: "opencode-ai", prereleaseTags: []string{"next", "beta"}},
	{command: "grok", name: "Grok Build", npmPackage: "@xai-official/grok"},
	{command: "openclaw", name: "OpenClaw", npmPackage: "openclaw"},
	{command: "hermes", name: "Hermes"},
	{command: "pi", name: "Pi", npmPackage: "@earendil-works/pi-coding-agent"},
}

const cliVersionNumber = `[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?`

var (
	cliVersionPattern = regexp.MustCompile(`(?:^|[^\w.])v?(` + cliVersionNumber + `)(?:$|[^\w.+-])`)
	cliVersionLine    = regexp.MustCompile(`^(?:(.*?)[\t :]+)?v?(` + cliVersionNumber + `)(?:[\t ]+\(.*\).*)?$`)
	cliGitBuild       = regexp.MustCompile(`^\d+\.g[0-9a-f]{4,}$`)
	cliReleaseClient  = &http.Client{Timeout: 8 * time.Second}
)

// CheckVersions 在每次调用时重新探测，单个工具失败时保留其他工具的结果。
func (*CLIUpdateService) CheckVersions(ctx context.Context) []CLIVersionInfo {
	home, _ := getUserHomeDir()
	paths := cliSearchPaths(home)
	results := make([]CLIVersionInfo, len(cliReleaseSources))
	var checks sync.WaitGroup
	for index, source := range cliReleaseSources {
		checks.Go(func() {
			toolPaths := paths
			if source.command == "grok" {
				toolPaths.fallback = slices.Concat(grokCLIFallbackPaths(home), paths.fallback)
			}
			results[index] = checkCLIVersion(ctx, source, toolPaths, cliReleaseClient, home)
		})
	}
	checks.Wait()
	return results
}

func checkCLIVersion(ctx context.Context, source cliReleaseSource, paths cliPaths, client *http.Client, home string) CLIVersionInfo {
	var installed string
	var found bool
	var probe sync.WaitGroup
	probe.Go(func() {
		installed, found = probeCLIVersion(ctx, source, paths)
	})
	release := source.fetchRelease(ctx, client)
	probe.Wait()

	stable := found && source.command == "claude" && claudeUsesStableChannel(home)
	latest := source.selectLatest(release, installed, stable)

	status := "upToDate"
	switch {
	case !found:
		status = "notDetected"
	case installed == "":
		status = "localUnavailable"
	case latest == "":
		status = "latestUnavailable"
	case cliGitBuild.MatchString(strings.TrimPrefix(semver.Build("v"+installed), "+")):
		status = "uncomparable"
	case semver.Compare("v"+installed, "v"+latest) < 0:
		status = "updateAvailable"
	}
	return CLIVersionInfo{
		Command: source.command, Name: source.name,
		InstalledVersion: installed, LatestVersion: latest, Status: status,
	}
}

func cliSearchPaths(home string) cliPaths {
	inherited := filepath.SplitList(os.Getenv("PATH"))
	var fallback []string
	if pnpm := os.Getenv("PNPM_HOME"); filepath.IsAbs(pnpm) {
		fallback = append(fallback, pnpm)
	}
	if home != "" {
		fallback = append(fallback, filepath.Join(home, ".local", "bin"), filepath.Join(home, ".volta", "bin"),
			filepath.Join(home, ".npm-global", "bin"), filepath.Join(home, ".bun", "bin"),
			filepath.Join(home, ".local", "share", "pnpm"))
		if runtime.GOOS == "darwin" {
			fallback = append(fallback, filepath.Join(home, "Library", "pnpm"))
		}
	}
	if runtime.GOOS == "windows" {
		if local := os.Getenv("LOCALAPPDATA"); filepath.IsAbs(local) {
			fallback = append(fallback, filepath.Join(local, "Programs", "OpenAI", "Codex", "bin"),
				filepath.Join(local, "Programs", "claude"), filepath.Join(local, "agy", "bin"),
				filepath.Join(local, "Microsoft", "WinGet", "Links"), filepath.Join(local, "pnpm"))
		}
		if roaming := os.Getenv("APPDATA"); filepath.IsAbs(roaming) {
			fallback = append(fallback, filepath.Join(roaming, "npm"))
		}
	} else {
		fallback = append(fallback, "/opt/homebrew/bin", "/usr/local/bin", "/usr/bin")
		nvmDir := os.Getenv("NVM_DIR")
		if nvmDir == "" && home != "" {
			nvmDir = filepath.Join(home, ".nvm")
		}
		if filepath.IsAbs(nvmDir) {
			nodeDir := filepath.Join(nvmDir, "versions", "node")
			entries, _ := os.ReadDir(nodeDir)
			slices.SortFunc(entries, func(a, b os.DirEntry) int { return semver.Compare(b.Name(), a.Name()) })
			for _, entry := range entries {
				if entry.IsDir() && semver.IsValid(entry.Name()) {
					fallback = append(fallback, filepath.Join(nodeDir, entry.Name(), "bin"))
				}
			}
		}
	}
	return cliPaths{inherited: inherited, fallback: fallback}
}

func grokCLIFallbackPaths(home string) []string {
	var paths []string
	if customDir := os.Getenv("GROK_BIN_DIR"); filepath.IsAbs(customDir) {
		paths = append(paths, customDir)
	}
	if home != "" {
		paths = append(paths, filepath.Join(home, ".grok", "bin"))
	}
	return paths
}

func probeCLIVersion(ctx context.Context, source cliReleaseSource, paths cliPaths) (version string, found bool) {
	searchPaths := slices.Concat(paths.inherited, paths.fallback)
	for index, dir := range searchPaths {
		// WindowsApps 里的应用执行别名可能打开商店或桌面应用。
		if !filepath.IsAbs(dir) || (runtime.GOOS == "windows" && strings.EqualFold(filepath.Base(dir), "WindowsApps")) {
			continue
		}
		path, err := exec.LookPath(filepath.Join(dir, source.command))
		if err != nil {
			continue
		}

		probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		cmd := cliVersionCommand(probeCtx, path)
		cmd.WaitDelay = 250 * time.Millisecond
		versionPath := searchPaths
		if index >= len(paths.inherited) {
			// 优先使用启动器同目录的 Node，再查找桌面进程和常见安装目录。
			versionPath = append([]string{dir}, searchPaths...)
		}
		cmd.Env = append(cmd.Environ(), "PATH="+strings.Join(versionPath, string(os.PathListSeparator)))
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		output, err := cmd.Output()
		if err != nil {
			return "", true
		}
		if version := source.versionFromOutput(string(output)); version != "" {
			return version, true
		}
		return source.versionFromOutput(stderr.String()), true
	}
	return "", false
}

func (source cliReleaseSource) versionFromOutput(output string) string {
	name := strings.TrimSuffix(strings.ToLower(source.name), " cli")
	for line := range strings.SplitSeq(output, "\n") {
		match := cliVersionLine.FindStringSubmatch(strings.TrimSpace(line))
		if match == nil || !semver.IsValid("v"+match[2]) {
			continue
		}
		label := strings.ToLower(match[1])
		label = strings.TrimSuffix(strings.TrimSuffix(label, " version"), " cli")
		if label == "" || label == name || label == source.command || label == source.command+"-cli" ||
			(source.command == "hermes" && label == "hermes agent") {
			return match[2]
		}
	}
	return ""
}

func extractCLIVersion(output string) string {
	for _, match := range cliVersionPattern.FindAllStringSubmatch(output, -1) {
		if semver.IsValid("v" + match[1]) {
			return match[1]
		}
	}
	return ""
}

func (source cliReleaseSource) fetchRelease(ctx context.Context, client *http.Client) map[string]any {
	var url string
	switch source.command {
	case "agy":
		url = antigravityManifestURL(runtime.GOOS, runtime.GOARCH)
	case "hermes":
		url = "https://api.github.com/repos/NousResearch/hermes-agent/releases/latest"
	default:
		url = "https://registry.npmjs.org/-/package/" + strings.ReplaceAll(source.npmPackage, "/", "%2f") + "/dist-tags"
	}
	if url == "" {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "CodeSwitchR")
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var release map[string]any
	if json.NewDecoder(io.LimitReader(resp.Body, 256<<10)).Decode(&release) != nil {
		return nil
	}
	return release
}

func (source cliReleaseSource) selectLatest(release map[string]any, installed string, stable bool) string {
	value := func(key string) string {
		result, _ := release[key].(string)
		return result
	}
	switch source.command {
	case "agy":
		return normalizeCLIVersion(value("version"))
	case "hermes":
		// 发布名称含 CLI 版本，部分历史 tag 使用日期。
		for _, candidate := range []string{extractCLIVersion(value("name")), normalizeCLIVersion(value("tag_name"))} {
			if candidate != "" && len(semver.Major("v"+candidate)) < 5 {
				return candidate
			}
		}
		return ""
	default:
		if source.command == "claude" && stable {
			return normalizeCLIVersion(value("stable"))
		}
		latest := normalizeCLIVersion(value("latest"))
		channel := source.prereleaseChannel(installed)
		if source.command == "claude" && latest != "" && semver.Compare("v"+installed, "v"+latest) > 0 {
			channel = "next"
		}
		if candidate := normalizeCLIVersion(value(channel)); channel != "" && semver.Compare("v"+candidate, "v"+latest) > 0 {
			return candidate
		}
		return latest
	}
}

func (source cliReleaseSource) prereleaseChannel(installed string) string {
	pre := strings.TrimPrefix(semver.Prerelease("v"+installed), "-")
	channel, _, _ := strings.Cut(pre, ".")
	channel, _, _ = strings.Cut(channel, "-")
	if slices.Contains(source.prereleaseTags, channel) {
		return channel
	}
	return ""
}

func claudeUsesStableChannel(home string) bool {
	dir := os.Getenv("CLAUDE_CONFIG_DIR")
	if !filepath.IsAbs(dir) {
		if home == "" {
			return false
		}
		dir = filepath.Join(home, claudeSettingsDir)
	}
	var settings struct {
		AutoUpdatesChannel string `json:"autoUpdatesChannel"`
	}
	err := ReadJSONFile(filepath.Join(dir, claudeSettingsFileName), &settings)
	return err == nil && settings.AutoUpdatesChannel == "stable"
}

func normalizeCLIVersion(value string) string {
	value = strings.TrimPrefix(strings.TrimSpace(value), "v")
	if extractCLIVersion(value) == value {
		return value
	}
	return ""
}

func antigravityManifestURL(osName, arch string) string {
	switch osName {
	case "darwin", "linux", "windows":
	default:
		return ""
	}
	if arch != "amd64" && arch != "arm64" {
		return ""
	}
	return "https://antigravity-cli-auto-updater-974169037036.us-central1.run.app/manifests/" + osName + "_" + arch + ".json"
}
