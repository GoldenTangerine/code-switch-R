// @name: 技能库回归测试
// @Descripttion: 验证多应用安装、导入、停用和失败回滚。
// @version: 1.0.0
// @Author: sm
// @Date: 2026-11-20 15:44:00
// @LastEditTime: 2026-11-20 15:44:00
// @FilePath: services/skilllibrary_test.go

package services

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixtureSkillMarkdown = "---\nname: Demo\ndescription: Sample skill\n---\nInstructions.\n"

type skillTestTransport []byte

func (transport skillTestTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(transport)),
	}, nil
}

func skillArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for path, content := range files {
		file, err := archive.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(file, content); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func newTestSkillLibrary(t *testing.T) *SkillService {
	t.Helper()
	home := t.TempDir()
	t.Chdir(t.TempDir())
	service := &SkillService{
		storePath:  filepath.Join(home, skillStoreDir, skillStoreFile),
		installDir: filepath.Join(home, ".claude", "skills"),
	}
	if err := os.MkdirAll(filepath.Dir(service.storePath), 0o755); err != nil {
		t.Fatal(err)
	}
	// Fixtures use the old skill.json shape to exercise upgrades without changing existing placements.
	if err := os.WriteFile(service.storePath, []byte(`{"skills":{"demo":{"installed":true}},"repos":[{"owner":"test","name":"repo","branch":"main","enabled":true}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	return service
}

func writeFixtureSkill(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func testSkillPath(t *testing.T, service *SkillService, app, location string) string {
	t.Helper()
	root, err := service.getInstallPath(app, location)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(root, "demo")
}

func assertSkillMarkdown(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(path, "SKILL.md"))
	if err != nil || string(got) != want {
		t.Fatalf("read %s = %q, %v; want %q", path, got, err, want)
	}
}

func assertMissingSkillPath(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be absent: %v", path, err)
	}
}

func TestSkillLibraryInstallToggleAndUninstall(t *testing.T) {
	service := newTestSkillLibrary(t)
	service.httpClient = &http.Client{Transport: skillTestTransport(skillArchive(t, map[string]string{
		"repo-main/skills/demo/SKILL.md":         fixtureSkillMarkdown,
		"repo-main/skills/demo/scripts/setup.sh": "echo ready\n",
	}))}
	discovered, err := service.DiscoverSkills()
	if err != nil || len(discovered) != 1 || discovered[0].RepoPath != "skills/demo" {
		t.Fatalf("discover nested skill = %+v, %v", discovered, err)
	}
	repoSkill := discovered[0]
	request := installRequest{
		Directory: repoSkill.Directory, RepoOwner: repoSkill.RepoOwner, RepoName: repoSkill.RepoName,
		Branch: repoSkill.RepoBranch, RepoPath: repoSkill.RepoPath, Apps: []string{skillPlatformCodex, skillPlatformGemini},
	}
	if err := service.InstallSkill(request); err != nil {
		t.Fatal(err)
	}
	// Reopen the store to verify the application state survives a restart.
	service = &SkillService{storePath: service.storePath}
	codex := testSkillPath(t, service, skillPlatformCodex, skillLocationUser)
	gemini := testSkillPath(t, service, skillPlatformGemini, skillLocationUser)
	master := filepath.Join(service.managedSkillRoot(), "demo")
	for _, path := range []string{codex, gemini, master} {
		assertSkillMarkdown(t, path, fixtureSkillMarkdown)
		script, err := os.ReadFile(filepath.Join(path, "scripts", "setup.sh"))
		if err != nil || string(script) != "echo ready\n" {
			t.Fatalf("skill asset missing from %s: %v", path, err)
		}
	}
	library, err := service.ListSkillLibrary()
	if err != nil || len(library.Local) != 0 || len(library.Managed) != 1 ||
		!library.Managed[0].Apps[skillPlatformCodex] || !library.Managed[0].Apps[skillPlatformGemini] ||
		library.Managed[0].Apps[skillPlatformClaude] {
		t.Fatalf("list library = %+v, %v", library, err)
	}
	for _, app := range request.Apps {
		if err := service.SetManagedSkillEnabled("demo", app, false); err != nil {
			t.Fatal(err)
		}
	}
	assertMissingSkillPath(t, codex)
	assertMissingSkillPath(t, gemini)
	content, err := service.GetSkillContent("demo", "", skillLocationLibrary)
	if err != nil || content != fixtureSkillMarkdown {
		t.Fatalf("read disabled skill = %q, %v", content, err)
	}
	if err := service.SetManagedSkillEnabled("demo", skillPlatformGemini, true); err != nil {
		t.Fatal(err)
	}
	assertSkillMarkdown(t, gemini, fixtureSkillMarkdown)
	if err := service.UninstallManagedSkill("demo"); err != nil {
		t.Fatal(err)
	}
	assertMissingSkillPath(t, master)
	assertMissingSkillPath(t, gemini)
}

func TestSkillLibraryImportProtectsLocalVariants(t *testing.T) {
	service := newTestSkillLibrary(t)
	claude := testSkillPath(t, service, skillPlatformClaude, skillLocationUser)
	codex := testSkillPath(t, service, skillPlatformCodex, skillLocationUser)
	gemini := testSkillPath(t, service, skillPlatformGemini, skillLocationUser)
	project := testSkillPath(t, service, skillPlatformCodex, skillLocationProject)
	writeFixtureSkill(t, claude, fixtureSkillMarkdown)
	writeFixtureSkill(t, gemini, fixtureSkillMarkdown)
	writeFixtureSkill(t, codex, fixtureSkillMarkdown+"User edits.\n")
	writeFixtureSkill(t, project, fixtureSkillMarkdown+"Project instructions.\n")

	if err := service.ImportLocalSkill("demo", skillPlatformClaude); err != nil {
		t.Fatal(err)
	}
	if err := service.ImportLocalSkill("demo", skillPlatformGemini); err != nil {
		t.Fatal(err)
	}
	if err := service.ImportLocalSkill("demo", skillPlatformCodex); err == nil {
		t.Fatal("adopted a different same-name skill")
	}
	if err := service.SetManagedSkillEnabled("demo", skillPlatformCodex, true); err == nil {
		t.Fatal("enabled over an existing local skill")
	}
	library, err := service.ListSkillLibrary()
	if err != nil || len(library.Managed) != 1 || len(library.Local) != 2 ||
		!library.Managed[0].Apps[skillPlatformClaude] || !library.Managed[0].Apps[skillPlatformGemini] ||
		len(library.Managed[0].Conflicts) != 1 || library.Managed[0].Conflicts[0] != skillPlatformCodex {
		t.Fatalf("list imported skill = %+v, %v", library, err)
	}
	if err := service.UninstallManagedSkill("demo"); err != nil {
		t.Fatal(err)
	}
	assertMissingSkillPath(t, claude)
	assertMissingSkillPath(t, gemini)
	assertSkillMarkdown(t, codex, fixtureSkillMarkdown+"User edits.\n")
	assertSkillMarkdown(t, project, fixtureSkillMarkdown+"Project instructions.\n")
}

func TestSkillLibraryRollsBackFailedMultiAppInstall(t *testing.T) {
	service := newTestSkillLibrary(t)
	service.httpClient = &http.Client{Transport: skillTestTransport(skillArchive(t, map[string]string{
		"repo-main/demo/SKILL.md": fixtureSkillMarkdown,
	}))}
	codex := testSkillPath(t, service, skillPlatformCodex, skillLocationUser)
	gemini := testSkillPath(t, service, skillPlatformGemini, skillLocationUser)
	writeFixtureSkill(t, gemini, fixtureSkillMarkdown+"Original.\n")
	if err := service.InstallSkill(installRequest{
		Directory: "demo", Apps: []string{skillPlatformCodex, skillPlatformGemini},
	}); err == nil {
		t.Fatal("install should fail on existing Gemini directory")
	}
	assertMissingSkillPath(t, codex)
	assertMissingSkillPath(t, filepath.Join(service.managedSkillRoot(), "demo"))
	assertSkillMarkdown(t, gemini, fixtureSkillMarkdown+"Original.\n")
	library, err := service.ListSkillLibrary()
	if err != nil || len(library.Managed) != 0 || len(library.Local) != 1 {
		t.Fatalf("list after failed install = %+v, %v", library, err)
	}
}

func TestSkillLibraryPreservesEditedCopiesAndRollsBackUninstall(t *testing.T) {
	service := newTestSkillLibrary(t)
	master := filepath.Join(service.managedSkillRoot(), "demo")
	writeFixtureSkill(t, master, fixtureSkillMarkdown)
	store, err := service.loadStore()
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, err := hashSkillDirectory(master)
	if err != nil {
		t.Fatal(err)
	}
	store.Managed["demo"] = managedSkillState{Directory: "demo", Deployments: map[string]string{
		skillPlatformClaude: fingerprint, skillPlatformGemini: fingerprint,
	}}
	if err := service.saveStoreLocked(store); err != nil {
		t.Fatal(err)
	}
	claude := testSkillPath(t, service, skillPlatformClaude, skillLocationUser)
	gemini := testSkillPath(t, service, skillPlatformGemini, skillLocationUser)
	writeFixtureSkill(t, claude, fixtureSkillMarkdown)
	writeFixtureSkill(t, gemini, fixtureSkillMarkdown+"Edited copy.\n")
	if err := service.UninstallManagedSkill("demo"); err == nil || !strings.Contains(err.Error(), "gemini") {
		t.Fatalf("uninstall edited copy = %v", err)
	}
	assertSkillMarkdown(t, master, fixtureSkillMarkdown)
	assertSkillMarkdown(t, claude, fixtureSkillMarkdown)
	assertSkillMarkdown(t, gemini, fixtureSkillMarkdown+"Edited copy.\n")
}

func TestSkillRepositoryRejectsArchiveTraversal(t *testing.T) {
	temp := t.TempDir()
	archive := filepath.Join(temp, "archive.zip")
	if err := os.WriteFile(archive, skillArchive(t, map[string]string{
		"repo-main/../../escape.txt": "content",
	}), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := unzipArchive(archive, filepath.Join(temp, "extract")); err == nil {
		t.Fatal("accepted an archive entry outside the snapshot")
	}
	assertMissingSkillPath(t, filepath.Join(temp, "escape.txt"))
}

func TestSkillLibraryProtectsMasterThroughSharedAppRoot(t *testing.T) {
	service := newTestSkillLibrary(t)
	claude := testSkillPath(t, service, skillPlatformClaude, skillLocationUser)
	writeFixtureSkill(t, claude, fixtureSkillMarkdown)
	if err := service.ImportLocalSkill("demo", skillPlatformClaude); err != nil {
		t.Fatal(err)
	}
	codexRoot := filepath.Dir(testSkillPath(t, service, skillPlatformCodex, skillLocationUser))
	if err := os.MkdirAll(filepath.Dir(codexRoot), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(service.managedSkillRoot(), codexRoot); err != nil {
		t.Skipf("directory links unavailable: %v", err)
	}
	library, err := service.ListSkillLibrary()
	if err != nil || len(library.Managed) != 1 || len(library.Local) != 0 {
		t.Fatalf("list through shared root = %+v, %v", library, err)
	}
	if err := service.UninstallSkillEx("demo", skillPlatformCodex, skillLocationUser); err == nil {
		t.Fatal("uninstalled the master through the app alias")
	}
	assertSkillMarkdown(t, filepath.Join(service.managedSkillRoot(), "demo"), fixtureSkillMarkdown)
	assertSkillMarkdown(t, claude, fixtureSkillMarkdown)
}

func TestSkillLibraryListsHealthyAppsWithBrokenRoot(t *testing.T) {
	service := newTestSkillLibrary(t)
	writeFixtureSkill(t, testSkillPath(t, service, skillPlatformClaude, skillLocationUser), fixtureSkillMarkdown)
	if err := service.ImportLocalSkill("demo", skillPlatformClaude); err != nil {
		t.Fatal(err)
	}
	broken := filepath.Dir(testSkillPath(t, service, skillPlatformGemini, skillLocationUser))
	if err := os.MkdirAll(filepath.Dir(broken), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(broken, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	library, err := service.ListSkillLibrary()
	if err != nil || len(library.Managed) != 1 || len(library.Warnings) != 1 ||
		library.Managed[0].AppStatuses[skillPlatformGemini] != "unavailable" ||
		library.Managed[0].AppStatuses[skillPlatformClaude] != "synced" {
		t.Fatalf("partial skill library = %+v, %v", library, err)
	}
}

func TestSkillLibraryAdoptsUnregisteredMasterAndRepairsBrokenLinks(t *testing.T) {
	service := newTestSkillLibrary(t)
	master := filepath.Join(service.managedSkillRoot(), "demo")
	writeFixtureSkill(t, master, fixtureSkillMarkdown)
	library, err := service.ListSkillLibrary()
	if err != nil || len(library.Managed) != 1 || library.Managed[0].MasterStatus != "unregistered" {
		t.Fatalf("unregistered skill = %+v, %v", library, err)
	}
	if err := service.RecoverManagedSkill("demo"); err != nil {
		t.Fatal(err)
	}
	if err := service.SetManagedSkillEnabled("demo", skillPlatformCodex, true); err != nil {
		t.Fatal(err)
	}
	codex := testSkillPath(t, service, skillPlatformCodex, skillLocationUser)
	if _, err := os.Readlink(codex); err != nil {
		t.Skipf("directory links unavailable: %v", err)
	}
	if err := os.Rename(master, filepath.Join(t.TempDir(), "backup")); err != nil {
		t.Fatal(err)
	}
	library, err = service.ListSkillLibrary()
	if err != nil || len(library.Managed) != 1 || library.Managed[0].MasterStatus != "missing" ||
		library.Managed[0].AppStatuses[skillPlatformCodex] != "broken" || library.Managed[0].Apps[skillPlatformCodex] {
		t.Fatalf("broken link = %+v, %v", library, err)
	}
	store, err := service.loadStore()
	if err != nil {
		t.Fatal(err)
	}
	state := store.Managed["demo"]
	state.RepoOwner, state.RepoName, state.RepoBranch = "test", "repo", "main"
	store.Managed["demo"] = state
	if err := service.saveStoreLocked(store); err != nil {
		t.Fatal(err)
	}
	service.httpClient = &http.Client{Transport: skillTestTransport(skillArchive(t, map[string]string{
		"repo-main/demo/SKILL.md": fixtureSkillMarkdown,
	}))}
	if err := service.RecoverManagedSkill("demo"); err != nil {
		t.Fatal(err)
	}
	assertSkillMarkdown(t, codex, fixtureSkillMarkdown)
}

func TestSkillLibraryUsesNativeProjectPathsAndImportsNestedPiSkills(t *testing.T) {
	service := newTestSkillLibrary(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for app, want := range map[string]string{
		skillPlatformPi:       filepath.Join(cwd, ".pi", "skills", "demo"),
		skillPlatformOpenClaw: filepath.Join(cwd, "skills", "demo"),
	} {
		if got := testSkillPath(t, service, app, skillLocationProject); got != want {
			t.Fatalf("%s project path = %s, want %s", app, got, want)
		}
	}
	custom := filepath.Join(t.TempDir(), "pi-agent")
	service.lookupEnv = func(key string) string {
		if key == "PI_CODING_AGENT_DIR" {
			return custom
		}
		return ""
	}
	path := filepath.Join(custom, "skills", "collection", "demo")
	writeFixtureSkill(t, path, fixtureSkillMarkdown)
	library, err := service.ListSkillLibrary()
	if err != nil || len(library.Local) != 1 || library.Local[0].Directory != "collection/demo" {
		t.Fatalf("nested Pi skill = %+v, %v", library, err)
	}
	if err := service.ImportLocalSkill("collection/demo", skillPlatformPi); err != nil {
		t.Fatal(err)
	}
	assertMissingSkillPath(t, path)
	assertSkillMarkdown(t, filepath.Join(custom, "skills", "demo"), fixtureSkillMarkdown)
}

func TestSkillLibraryDisablesInheritedClaudeSkillInNativeConfigs(t *testing.T) {
	for _, format := range []string{"v1", "v2"} {
		t.Run(format, func(t *testing.T) {
			service := newTestSkillLibrary(t)
			base := filepath.Dir(filepath.Dir(testSkillPath(t, service, skillPlatformOpenCode, skillLocationUser)))
			if err := os.MkdirAll(base, 0o755); err != nil {
				t.Fatal(err)
			}
			openCode := filepath.Join(base, "opencode.jsonc")
			openCodeConfig := "{\n  // keep my comment\n  \"permission\": {\"skill\": {\"unrelated\": \"deny\"}},\n}\n"
			if format == "v2" {
				openCodeConfig = "{\n  // keep my comment\n  \"policies\": [{\"action\":\"skill.use\",\"resource\":\"unrelated\",\"effect\":\"deny\"}],\n}\n"
			}
			if err := os.WriteFile(openCode, []byte(openCodeConfig), 0o600); err != nil {
				t.Fatal(err)
			}
			grokRoot := filepath.Dir(filepath.Dir(testSkillPath(t, service, skillPlatformGrokBuild, skillLocationUser)))
			if err := os.MkdirAll(grokRoot, 0o755); err != nil {
				t.Fatal(err)
			}
			grok := filepath.Join(grokRoot, "config.toml")
			if err := os.WriteFile(grok, []byte("# keep my comment\n[skills]\npaths = [\"/sample\"]\ndisabled = [\"unrelated\"]\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			claude := testSkillPath(t, service, skillPlatformClaude, skillLocationUser)
			writeFixtureSkill(t, claude, fixtureSkillMarkdown)
			if err := service.ImportLocalSkill("demo", skillPlatformClaude); err != nil {
				t.Fatal(err)
			}
			library, err := service.ListSkillLibrary()
			if err != nil || len(library.Managed) != 1 || library.Managed[0].AppStatuses[skillPlatformOpenCode] != "blocked" ||
				library.Managed[0].AppStatuses[skillPlatformGrokBuild] != "blocked" {
				t.Fatalf("native state = %+v, %v", library, err)
			}
			for _, app := range skillNativeApps {
				if err := service.SetManagedSkillEnabled("demo", app, true); err != nil {
					t.Fatal(err)
				}
				if config := service.readSkillNativeConfig(app); config.err != nil || config.blocks("Demo") || !config.blocks("unrelated") ||
					!strings.Contains(string(config.data), "keep my comment") {
					t.Fatalf("re-enable changed the wrong settings for %s: %s (%v)", app, config.data, config.err)
				}
				if err := service.SetManagedSkillEnabled("demo", app, false); err != nil {
					t.Fatal(err)
				}
			}
			assertSkillMarkdown(t, claude, fixtureSkillMarkdown)
			if err := service.UninstallManagedSkill("demo"); err != nil {
				t.Fatal(err)
			}
			for _, app := range skillNativeApps {
				if config := service.readSkillNativeConfig(app); config.err != nil || config.blocks("Demo") || !config.blocks("unrelated") {
					t.Fatalf("uninstall left a native rule or removed a user rule for %s", app)
				}
			}
		})
	}
}
