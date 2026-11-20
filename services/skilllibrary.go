// @name: 技能库
// @Descripttion: 保存技能主副本并按应用管理部署。
// @version: 1.0.0
// @Author: sm
// @Date: 2026-11-20 15:31:00
// @LastEditTime: 2026-11-20 15:31:00
// @FilePath: services/skilllibrary.go

package services

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

const (
	skillLocationLibrary = "library"
	skillDeploymentLink  = "link"
)

var managedSkillApps = []string{
	skillPlatformClaude, skillPlatformCodex, skillPlatformGemini, skillPlatformOpenCode, skillPlatformGrokBuild,
	skillPlatformOpenClaw, skillPlatformHermes, skillPlatformPi,
}

type managedSkillState struct {
	Directory    string                      `json:"directory"`
	Deployments  map[string]string           `json:"deployments"`
	RepoOwner    string                      `json:"repo_owner,omitempty"`
	RepoName     string                      `json:"repo_name,omitempty"`
	RepoBranch   string                      `json:"repo_branch,omitempty"`
	RepoPath     string                      `json:"repo_path,omitempty"`
	NativeBlocks map[string]skillNativeBlock `json:"native_blocks,omitempty"`
}

type ManagedSkill struct {
	Skill
	Apps         map[string]bool   `json:"apps"`
	Conflicts    []string          `json:"conflicts"`
	AppStatuses  map[string]string `json:"app_statuses"`
	MasterStatus string            `json:"master_status"`
}

type SkillLibrary struct {
	Managed  []ManagedSkill `json:"managed"`
	Local    []Skill        `json:"local"`
	Warnings []string       `json:"warnings"`
}

func validSkillDirectory(directory string) bool {
	return directory == strings.TrimSpace(directory) &&
		!strings.HasPrefix(directory, ".") && !strings.HasSuffix(directory, ".") &&
		!strings.Contains(directory, "/") && validSkillRepoPath(directory)
}

func validSkillRepoPath(path string) bool {
	return fs.ValidPath(path) && !strings.ContainsAny(path, "\\:<>|\"?*\x00") && filepath.IsLocal(filepath.FromSlash(path))
}

func validLocalSkillPath(path string) bool {
	if !validSkillRepoPath(path) {
		return false
	}
	for _, part := range strings.Split(path, "/") {
		if !validSkillDirectory(part) {
			return false
		}
	}
	return true
}

func validateSkillApps(apps []string) error {
	for _, app := range apps {
		if !slices.Contains(managedSkillApps, app) {
			return fmt.Errorf("未知的技能应用 %q", app)
		}
	}
	return nil
}

func (ss *SkillService) managedSkillRoot() string {
	return filepath.Join(filepath.Dir(ss.storePath), "skills")
}

func (ss *SkillService) ListSkillLibrary() (SkillLibrary, error) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	store, err := ss.loadStoreLocked()
	if err != nil {
		return SkillLibrary{}, err
	}
	library := SkillLibrary{Managed: []ManagedSkill{}, Local: []Skill{}, Warnings: []string{}}
	ss.addUnregisteredSkills(&store, &library)
	unavailable := make(map[string]bool)
	for _, app := range managedSkillApps {
		userRoot, err := ss.getInstallPath(app, skillLocationUser)
		if err != nil {
			library.Warnings = append(library.Warnings, fmt.Sprintf("%s: %v", app, err))
			unavailable[app] = true
			continue
		}
		if ss.checkLocalSkillMutation(app, userRoot, userRoot, true) != nil {
			unavailable[app] = true
		}
		for _, location := range skillLocations(app) {
			root, err := ss.getInstallPath(app, location)
			if err != nil {
				library.Warnings = append(library.Warnings, fmt.Sprintf("%s: %v", app, err))
				continue
			}
			if location != skillLocationUser && sameSkillRoot(root, userRoot) {
				continue
			}
			if sameSkillRoot(root, ss.managedSkillRoot()) {
				unavailable[app] = true
				continue
			}
			skills, err := ss.scanSkillsDirectory(root, app, location)
			if err != nil {
				library.Warnings = append(library.Warnings, fmt.Sprintf("%s: %v", root, err))
				if location == skillLocationUser && skills == nil {
					unavailable[app] = true
				}
			}
			for _, skill := range skills {
				target := filepath.Join(root, skill.Directory)
				if (location == skillLocationUser && ss.isManagedDeployment(store, app, skill.Directory, target)) ||
					ss.isSkillLibraryPath(filepath.Dir(target)) {
					continue
				}
				skill.ReadOnly = ss.checkLocalSkillMutation(app, root, target, false) != nil
				library.Local = append(library.Local, skill)
			}
		}
	}
	var nativeConfigs map[string]skillNativeConfig
	if len(store.Managed) > 0 {
		nativeConfigs = ss.readSkillNativeConfigs()
	}
	for app, config := range nativeConfigs {
		if config.err != nil {
			library.Warnings = append(library.Warnings, fmt.Sprintf("%s: %v", app, config.err))
		}
	}
	slices.Sort(library.Warnings)
	for _, state := range store.Managed {
		skill, err := ss.managedSkillSummary(state, unavailable, nativeConfigs)
		if err != nil {
			library.Warnings = append(library.Warnings, err.Error())
			continue
		}
		library.Managed = append(library.Managed, skill)
	}
	sortSkills(library.Local)
	sort.Slice(library.Managed, func(i, j int) bool {
		return skillLess(library.Managed[i].Skill, library.Managed[j].Skill)
	})
	return library, nil
}

func (ss *SkillService) managedSkillSummary(state managedSkillState, unavailable map[string]bool, configs map[string]skillNativeConfig) (ManagedSkill, error) {
	if !validSkillDirectory(state.Directory) {
		return ManagedSkill{}, fmt.Errorf("无效的技能目录 %q", state.Directory)
	}
	source := filepath.Join(ss.managedSkillRoot(), state.Directory)
	masterOK := hasSkillMaster(source)
	skill := Skill{Name: state.Directory, Directory: state.Directory, Installed: true, InstallLocation: skillLocationLibrary}
	if masterOK {
		skill = ss.localSkillSummary(source, state.Directory, "", skillLocationLibrary)
	}
	skill.Key = "library:" + state.Directory
	skill.RepoOwner, skill.RepoName, skill.RepoBranch, skill.RepoPath = state.RepoOwner, state.RepoName, state.RepoBranch, state.RepoPath
	if state.RepoOwner != "" && state.RepoName != "" {
		skill.ReadmeURL = buildRepoURL(skillRepoConfig{Owner: state.RepoOwner, Name: state.RepoName}, state.RepoBranch, state.RepoPath)
	}
	result := ManagedSkill{Skill: skill, Apps: make(map[string]bool), Conflicts: []string{}, AppStatuses: make(map[string]string)}
	switch {
	case state.Deployments == nil:
		result.MasterStatus = "unregistered"
	case !masterOK:
		result.MasterStatus = "missing"
	}
	for _, app := range managedSkillApps {
		root, err := ss.getInstallPath(app, skillLocationUser)
		target := filepath.Join(root, state.Directory)
		status := "disabled"
		switch {
		case err != nil || unavailable[app]:
			status = "unavailable"
		case isSkillDeployment(target, source, state.Deployments[app]):
			if hasSkillMarkdown(target) {
				status = "synced"
				result.Apps[app] = true
			} else {
				status = "broken"
			}
		case skillPathExists(target):
			status = "conflict"
			result.Conflicts = append(result.Conflicts, app)
		case ss.inheritsSkill(app, state.Directory, skill.Name, configs[app]):
			status = "inherited"
		}
		if status != "unavailable" && status != "conflict" && status != "broken" && configs[app].blocks(skill.Name) {
			status = "blocked"
		}
		result.AppStatuses[app] = status
	}
	return result, nil
}

func skillPathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func hasSkillMaster(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.IsDir() && hasSkillMarkdown(path)
}

func (ss *SkillService) inheritsSkill(app, directory, name string, config skillNativeConfig) bool {
	var roots []string
	defaultClaude := filepath.Join(ss.skillHome(), ".claude", "skills")
	shared := filepath.Join(ss.skillHome(), ".agents", "skills")
	switch app {
	case skillPlatformOpenCode:
		if !enabledSkillEnv(ss.skillEnv("OPENCODE_DISABLE_EXTERNAL_SKILLS")) {
			roots = append(roots, shared)
			if !enabledSkillEnv(ss.skillEnv("OPENCODE_DISABLE_CLAUDE_CODE_SKILLS")) {
				roots = append(roots, defaultClaude)
			}
		}
	case skillPlatformGrokBuild:
		roots = append(roots, shared)
		compat := config.grok.Compat.Claude.Skills
		env := ss.skillEnv("GROK_CLAUDE_SKILLS_ENABLED")
		if enabledSkillEnv(env) || (env == "" && (compat == nil || *compat)) {
			roots = append(roots, defaultClaude)
		}
	case skillPlatformCodex, skillPlatformGemini, skillPlatformPi, skillPlatformOpenClaw:
		roots = append(roots, shared)
	}
	for _, root := range roots {
		path := filepath.Join(root, directory)
		if hasSkillMarkdown(path) && ss.localSkillSummary(path, directory, "", "").Name == name {
			return true
		}
	}
	return false
}

func enabledSkillEnv(value string) bool {
	return value == "1" || strings.EqualFold(value, "true")
}

func (ss *SkillService) addUnregisteredSkills(store *skillStore, library *SkillLibrary) {
	root := ss.managedSkillRoot()
	entries, err := os.ReadDir(root)
	if err != nil && !os.IsNotExist(err) {
		library.Warnings = append(library.Warnings, fmt.Sprintf("%s: %v", root, err))
	}
	for _, entry := range entries {
		directory := entry.Name()
		if !entry.IsDir() || !validSkillDirectory(directory) || !hasSkillMarkdown(filepath.Join(root, directory)) {
			continue
		}
		key := normalizeDirectoryKey(directory)
		if _, exists := store.Managed[key]; !exists {
			store.Managed[key] = managedSkillState{Directory: directory}
		}
	}
}

func skillPathWithin(path, root string) bool {
	realPath, pathErr := filepath.EvalSymlinks(path)
	realRoot, rootErr := filepath.EvalSymlinks(root)
	if pathErr != nil || rootErr != nil {
		return false
	}
	relative, err := filepath.Rel(realRoot, realPath)
	return err == nil && filepath.IsLocal(relative)
}

func (ss *SkillService) isSkillLibraryPath(path string) bool {
	return skillPathWithin(path, ss.managedSkillRoot())
}

func (ss *SkillService) checkLocalSkillMutation(app, root, target string, followTarget bool) error {
	if app == "" {
		app = skillPlatformClaude
	}
	path := filepath.Dir(target)
	if followTarget {
		path = target
	}
	shared := filepath.Join(ss.skillHome(), ".agents", "skills")
	if ss.isSkillLibraryPath(path) || ss.isSkillLibraryPath(root) ||
		skillPathWithin(path, shared) || skillPathWithin(root, shared) {
		return errors.New("此技能位于共享目录，请从技能库或原始目录管理")
	}
	for _, other := range managedSkillApps {
		if other != app {
			otherRoot, err := ss.getInstallPath(other, skillLocationUser)
			if err == nil && (sameSkillRoot(root, otherRoot) || skillPathWithin(path, otherRoot)) {
				return errors.New("此技能位于其他应用共用的目录")
			}
		}
	}
	return nil
}

func (ss *SkillService) isManagedDeployment(store skillStore, app, directory, target string) bool {
	state, ok := store.Managed[normalizeDirectoryKey(directory)]
	return ok && isSkillDeployment(target, filepath.Join(ss.managedSkillRoot(), state.Directory), state.Deployments[app])
}

func sameSkillRoot(left, right string) bool {
	if filepath.Clean(left) == filepath.Clean(right) {
		return true
	}
	leftInfo, leftErr := os.Stat(left)
	rightInfo, rightErr := os.Stat(right)
	return leftErr == nil && rightErr == nil && os.SameFile(leftInfo, rightInfo)
}

func isSkillDeployment(target, source, deployment string) bool {
	info, err := os.Lstat(target)
	if err != nil {
		return false
	}
	if link, err := os.Readlink(target); err == nil {
		if !filepath.IsAbs(link) {
			link = filepath.Join(filepath.Dir(target), link)
		}
		return filepath.Clean(link) == source
	}
	return deployment != "" && deployment != skillDeploymentLink && info.IsDir() && hasSkillMarkdown(target)
}

func hasSkillMarkdown(directory string) bool {
	info, err := os.Lstat(filepath.Join(directory, "SKILL.md"))
	return err == nil && info.Mode().IsRegular()
}

func (ss *SkillService) installManagedSkill(directory, source string, apps []string, state managedSkillState) error {
	if !hasSkillMarkdown(source) {
		return fmt.Errorf("%s 缺少 SKILL.md", directory)
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()
	store, err := ss.loadStoreLocked()
	if err != nil {
		return err
	}
	key := normalizeDirectoryKey(directory)
	if _, exists := store.Managed[key]; exists {
		return fmt.Errorf("技能库中已存在 %s", directory)
	}
	state.Directory = directory
	state.Deployments = make(map[string]string)
	var changes skillFileChanges
	defer changes.finish()
	master := filepath.Join(ss.managedSkillRoot(), directory)
	if err := changes.publish(master, func(staged string) error {
		return copyDirectory(source, staged)
	}); err != nil {
		return err
	}
	for _, app := range apps {
		if err := ss.setManagedSkillApp(&changes, &state, app, true); err != nil {
			return err
		}
	}
	if err := ss.blockInheritedSkill(&changes, &state); err != nil {
		return err
	}
	store.Managed[key] = state
	return ss.commitSkillChanges(store, &changes)
}

// ImportLocalSkill adopts a user-level skill while keeping it available in its original app.
func (ss *SkillService) ImportLocalSkill(directory, app string) error {
	if !validLocalSkillPath(directory) {
		return fmt.Errorf("无效的技能目录 %q", directory)
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()
	source, err := ss.managedSkillAppTarget(app, directory)
	if err != nil {
		return err
	}
	store, err := ss.loadStoreLocked()
	if err != nil {
		return err
	}
	directory = filepath.Base(directory)
	key := normalizeDirectoryKey(directory)
	state, exists := store.Managed[key]
	if !exists && hasSkillMaster(filepath.Join(ss.managedSkillRoot(), directory)) {
		state = managedSkillState{Directory: directory, Deployments: make(map[string]string)}
		exists = true
	}
	if exists {
		if state.Directory != directory {
			return fmt.Errorf("技能库中已存在 %s", state.Directory)
		}
		if ss.isManagedDeployment(store, app, directory, source) {
			return nil
		}
	} else {
		state = managedSkillState{Directory: directory, Deployments: make(map[string]string)}
	}
	sourceDir, err := filepath.EvalSymlinks(source)
	if err != nil {
		return err
	}
	if !hasSkillMarkdown(sourceDir) {
		return fmt.Errorf("%s 缺少 SKILL.md", directory)
	}
	var changes skillFileChanges
	defer changes.finish()
	master := filepath.Join(ss.managedSkillRoot(), directory)
	if !exists {
		if err := changes.publish(master, func(staged string) error {
			return copyDirectory(sourceDir, staged)
		}); err != nil {
			return err
		}
	} else {
		sourceHash, err := hashSkillDirectory(sourceDir)
		if err != nil {
			return err
		}
		masterHash, err := hashSkillDirectory(master)
		if err != nil {
			return err
		}
		if sourceHash != masterHash {
			return fmt.Errorf("%s 中的 %s 与技能库中的同名技能内容不同", app, directory)
		}
	}
	if err := changes.remove(source); err != nil {
		return err
	}
	if err := ss.setManagedSkillApp(&changes, &state, app, true); err != nil {
		return err
	}
	if err := ss.blockInheritedSkill(&changes, &state); err != nil {
		return err
	}
	store.Managed[key] = state
	return ss.commitSkillChanges(store, &changes)
}

func (ss *SkillService) RecoverManagedSkill(directory string) error {
	if !validSkillDirectory(directory) {
		return fmt.Errorf("无效的技能目录 %q", directory)
	}
	state, complete, err := ss.recoverSkillFromDisk(directory)
	if complete || err != nil {
		return err
	}
	if state.RepoOwner == "" || state.RepoName == "" {
		return fmt.Errorf("找不到 %s 的可用副本，请把备份放回技能库目录或重新导入本地副本", directory)
	}
	path := state.RepoPath
	if path == "" {
		path = directory
	}
	if !validSkillRepoPath(path) {
		return fmt.Errorf("无效的仓库技能路径 %q", path)
	}
	return ss.withSkillRepoSource(installRequest{
		Directory: directory, RepoOwner: state.RepoOwner, RepoName: state.RepoName, Branch: state.RepoBranch, RepoPath: path,
	}, func(source string, _ managedSkillState) error {
		return ss.updateManagedSkill(directory, func(changes *skillFileChanges, current *managedSkillState) error {
			master := filepath.Join(ss.managedSkillRoot(), directory)
			if hasSkillMaster(master) {
				return nil
			}
			return publishRestoredSkill(changes, master, source)
		}, false)
	})
}

func (ss *SkillService) recoverSkillFromDisk(directory string) (managedSkillState, bool, error) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	store, err := ss.loadStoreLocked()
	if err != nil {
		return managedSkillState{}, false, err
	}
	key := normalizeDirectoryKey(directory)
	state, exists := store.Managed[key]
	master := filepath.Join(ss.managedSkillRoot(), directory)
	if exists && state.Directory != directory {
		return state, false, fmt.Errorf("技能库中已存在 %s", state.Directory)
	}
	var changes skillFileChanges
	defer changes.finish()
	if hasSkillMaster(master) {
		if exists {
			return state, true, nil
		}
		state = managedSkillState{Directory: directory, Deployments: make(map[string]string)}
		for _, app := range managedSkillApps {
			root, err := ss.getInstallPath(app, skillLocationUser)
			if err == nil && !ss.isSkillLibraryPath(root) && isSkillDeployment(filepath.Join(root, directory), master, "") {
				state.Deployments[app] = skillDeploymentLink
			}
		}
		store.Managed[key] = state
		return state, true, ss.commitSkillChanges(store, &changes)
	}
	if !exists {
		return state, false, fmt.Errorf("技能库中找不到 %s", directory)
	}
	for _, app := range managedSkillApps {
		fingerprint := state.Deployments[app]
		if fingerprint == "" || fingerprint == skillDeploymentLink {
			continue
		}
		root, err := ss.getInstallPath(app, skillLocationUser)
		if err != nil {
			continue
		}
		source := filepath.Join(root, directory)
		if !hasSkillMaster(source) {
			continue
		}
		if hash, err := hashSkillDirectory(source); err != nil || hash != fingerprint {
			continue
		}
		if err := publishRestoredSkill(&changes, master, source); err != nil {
			return state, false, err
		}
		return state, true, ss.commitSkillChanges(store, &changes)
	}
	return state, false, nil
}

func publishRestoredSkill(changes *skillFileChanges, master, source string) error {
	if !hasSkillMaster(source) {
		return errors.New("恢复来源缺少 SKILL.md")
	}
	return changes.publish(master, func(staged string) error {
		return copyDirectory(source, staged)
	})
}

func (ss *SkillService) SetManagedSkillEnabled(directory, app string, enabled bool) error {
	if err := validateSkillApps([]string{app}); err != nil {
		return err
	}
	return ss.updateManagedSkill(directory, func(changes *skillFileChanges, state *managedSkillState) error {
		if err := ss.setManagedSkillApp(changes, state, app, enabled); err != nil {
			return err
		}
		if enabled && app == skillPlatformClaude {
			return ss.blockInheritedSkill(changes, state)
		}
		return nil
	}, false)
}

func (ss *SkillService) UninstallManagedSkill(directory string) error {
	return ss.updateManagedSkill(directory, func(changes *skillFileChanges, state *managedSkillState) error {
		for _, app := range managedSkillApps {
			if err := ss.disableManagedSkill(changes, state, app); err != nil {
				return err
			}
		}
		for app, receipt := range state.NativeBlocks {
			if err := ss.setSkillNativeBlock(changes, state, app, receipt.Name, false); err != nil {
				return err
			}
		}
		return changes.remove(filepath.Join(ss.managedSkillRoot(), state.Directory))
	}, true)
}

func (ss *SkillService) updateManagedSkill(directory string, update func(*skillFileChanges, *managedSkillState) error, uninstall bool) error {
	if !validSkillDirectory(directory) {
		return fmt.Errorf("无效的技能目录 %q", directory)
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()
	store, err := ss.loadStoreLocked()
	if err != nil {
		return err
	}
	key := normalizeDirectoryKey(directory)
	state, ok := store.Managed[key]
	if !ok || state.Directory != directory {
		return fmt.Errorf("技能库中找不到 %s", directory)
	}
	var changes skillFileChanges
	defer changes.finish()
	if err := update(&changes, &state); err != nil {
		return err
	}
	if uninstall {
		delete(store.Managed, key)
	} else {
		store.Managed[key] = state
	}
	return ss.commitSkillChanges(store, &changes)
}

func (ss *SkillService) getManagedSkillContent(directory string) (string, error) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	store, err := ss.loadStoreLocked()
	if err != nil {
		return "", err
	}
	state, ok := store.Managed[normalizeDirectoryKey(directory)]
	master := filepath.Join(ss.managedSkillRoot(), directory)
	if (ok && state.Directory != directory) || !hasSkillMaster(master) {
		return "", fmt.Errorf("技能库中找不到 %s", directory)
	}
	data, err := os.ReadFile(filepath.Join(master, "SKILL.md"))
	return string(data), err
}

func (ss *SkillService) setManagedSkillApp(changes *skillFileChanges, state *managedSkillState, app string, enabled bool) error {
	master := filepath.Join(ss.managedSkillRoot(), state.Directory)
	name := ss.localSkillSummary(master, state.Directory, "", "").Name
	if slices.Contains(skillNativeApps, app) {
		if err := ss.setSkillNativeBlock(changes, state, app, name, !enabled); err != nil {
			return err
		}
		if enabled && ss.readSkillNativeConfig(app).blocks(name) {
			return fmt.Errorf("%s 原有配置已停用 %s", app, name)
		}
	}
	if enabled {
		return ss.enableManagedSkill(changes, state, app)
	}
	return ss.disableManagedSkill(changes, state, app)
}

func (ss *SkillService) blockInheritedSkill(changes *skillFileChanges, state *managedSkillState) error {
	master := filepath.Join(ss.managedSkillRoot(), state.Directory)
	name := ss.localSkillSummary(master, state.Directory, "", "").Name
	for _, app := range skillNativeApps {
		if state.Deployments[app] == "" && ss.inheritsSkill(app, state.Directory, name, ss.readSkillNativeConfig(app)) {
			root, err := ss.getInstallPath(app, skillLocationUser)
			if err != nil {
				return err
			}
			target := filepath.Join(root, state.Directory)
			if skillPathExists(target) && !isSkillDeployment(target, master, state.Deployments[app]) {
				continue
			}
			local, _ := ss.scanSkillsDirectory(root, app, skillLocationUser)
			if slices.ContainsFunc(local, func(skill Skill) bool { return skill.Name == name }) {
				continue
			}
			if err := ss.setSkillNativeBlock(changes, state, app, name, true); err != nil {
				return err
			}
		}
	}
	return nil
}

func (ss *SkillService) enableManagedSkill(changes *skillFileChanges, state *managedSkillState, app string) error {
	target, err := ss.managedSkillAppTarget(app, state.Directory)
	if err != nil {
		return err
	}
	master := filepath.Join(ss.managedSkillRoot(), state.Directory)
	if !hasSkillMaster(master) {
		return fmt.Errorf("技能库中的 %s 缺少 SKILL.md", state.Directory)
	}
	if isSkillDeployment(target, master, state.Deployments[app]) {
		if state.Deployments[app] == "" {
			state.Deployments[app] = skillDeploymentLink
		}
		return nil
	}
	deployment := skillDeploymentLink
	err = changes.publish(target, func(staged string) error {
		if err := os.Symlink(master, staged); err == nil {
			return nil
		}
		if err := copyDirectory(master, staged); err != nil {
			return err
		}
		deployment, err = hashSkillDirectory(staged)
		return err
	})
	if err != nil {
		return fmt.Errorf("%s: %w", app, err)
	}
	state.Deployments[app] = deployment
	return nil
}

func (ss *SkillService) disableManagedSkill(changes *skillFileChanges, state *managedSkillState, app string) error {
	root, err := ss.getInstallPath(app, skillLocationUser)
	if err != nil {
		return err
	}
	target := filepath.Join(root, state.Directory)
	master := filepath.Join(ss.managedSkillRoot(), state.Directory)
	if isSkillDeployment(target, master, state.Deployments[app]) {
		if _, err := ss.managedSkillAppTarget(app, state.Directory); err != nil {
			return err
		}
		if info, err := os.Lstat(target); err != nil {
			return err
		} else if info.IsDir() {
			hash, err := hashSkillDirectory(target)
			if err != nil {
				return err
			}
			if hash != state.Deployments[app] {
				return fmt.Errorf("%s 中的 %s 已被修改，请先将该目录移出应用的技能目录", app, state.Directory)
			}
		}
		if err := changes.remove(target); err != nil {
			return err
		}
	}
	delete(state.Deployments, app)
	return nil
}

func (ss *SkillService) managedSkillAppTarget(app, directory string) (string, error) {
	if err := validateSkillApps([]string{app}); err != nil {
		return "", err
	}
	root, err := ss.getInstallPath(app, skillLocationUser)
	if err != nil {
		return "", err
	}
	target := filepath.Join(root, directory)
	if err := ss.checkLocalSkillMutation(app, root, target, false); err != nil {
		return "", err
	}
	if _, err := os.Stat(root); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return target, nil
}

func hashSkillDirectory(root string) (string, error) {
	hash := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			_, _ = fmt.Fprintf(hash, "dir %q\n", filepath.ToSlash(rel))
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("技能包含不支持的文件类型 %s", rel)
		}
		_, _ = fmt.Fprintf(hash, "file %q %d %o\n", filepath.ToSlash(rel), info.Size(), info.Mode().Perm())
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(hash, file)
		return errors.Join(err, file.Close())
	})
	return fmt.Sprintf("%x", hash.Sum(nil)), err
}

// Stage removals beside the original directory so rollback works across volumes.
type skillFileChanges struct {
	undo      []func() error
	staging   []string
	committed bool
}

func (changes *skillFileChanges) writeConfig(path string, original, updated []byte) error {
	write := func(data, expected []byte) error {
		current, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if !bytes.Equal(current, expected) || (err == nil) != (expected != nil) {
			return fmt.Errorf("配置已被其他程序修改 %s", path)
		}
		if data == nil {
			return os.Remove(path)
		}
		mode := os.FileMode(0o600)
		if info, err := os.Stat(path); err == nil {
			mode = info.Mode().Perm()
		}
		temp, err := changes.stage(path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(temp, data, mode); err != nil {
			return err
		}
		return atomicRename(temp, path)
	}
	if err := write(updated, original); err != nil {
		return err
	}
	changes.undo = append(changes.undo, func() error { return write(original, updated) })
	return nil
}

func (changes *skillFileChanges) stage(path string) (string, error) {
	root := filepath.Dir(path)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp(root, ".codeswitch-skill-")
	if err != nil {
		return "", err
	}
	changes.staging = append(changes.staging, dir)
	return filepath.Join(dir, "skill"), nil
}

func (changes *skillFileChanges) publish(target string, populate func(string) error) error {
	if _, err := os.Lstat(target); err == nil {
		return fmt.Errorf("目标目录已存在 %s", target)
	} else if !os.IsNotExist(err) {
		return err
	}
	staged, err := changes.stage(target)
	if err != nil {
		return err
	}
	if err := populate(staged); err != nil {
		return err
	}
	if _, err := os.Lstat(target); err == nil {
		return fmt.Errorf("目标目录已存在 %s", target)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(staged, target); err != nil {
		return err
	}
	changes.undo = append(changes.undo, func() error { return os.Rename(target, staged) })
	return nil
}

func (changes *skillFileChanges) remove(target string) error {
	if _, err := os.Lstat(target); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	staged, err := changes.stage(target)
	if err != nil {
		return err
	}
	if err := os.Rename(target, staged); err != nil {
		return err
	}
	changes.undo = append(changes.undo, func() error {
		if _, err := os.Lstat(target); !os.IsNotExist(err) {
			return fmt.Errorf("无法将技能恢复到已占用的路径 %s", target)
		}
		return os.Rename(staged, target)
	})
	return nil
}

func (changes *skillFileChanges) finish() {
	if !changes.committed {
		var rollbackErr error
		for i := len(changes.undo) - 1; i >= 0; i-- {
			rollbackErr = errors.Join(rollbackErr, changes.undo[i]())
		}
		if rollbackErr != nil {
			log.Printf("skill rollback failed, retained %v: %v", changes.staging, rollbackErr)
			return
		}
	}
	for _, dir := range changes.staging {
		if err := os.RemoveAll(dir); err != nil {
			log.Printf("skill staging cleanup failed for %s: %v", dir, err)
		}
	}
}

func (ss *SkillService) commitSkillChanges(store skillStore, changes *skillFileChanges) error {
	if err := ss.saveStoreLocked(store); err != nil {
		return err
	}
	changes.committed = true
	return nil
}
