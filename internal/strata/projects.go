package strata

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
)

var (
	ErrProjectExists   = errors.New("project already exists")
	ErrProjectNotFound = errors.New("project not found")
)

func NormalizeProjectPath(input string) (string, error) {
	path, err := normalizeProjectPath(input, false)
	if err != nil {
		return "", err
	}
	return path, nil
}

func NormalizeOptionalProjectPath(input string) (string, error) {
	return normalizeProjectPath(input, true)
}

func ValidateProjectName(input string) (string, error) {
	name := strings.TrimSpace(input)
	if name == "" {
		return "", errors.New("project name cannot be empty")
	}
	if strings.Contains(name, "/") {
		return "", errors.New("project name cannot contain /")
	}
	if err := validateProjectSegment(name); err != nil {
		return "", err
	}
	return name, nil
}

func (s *Store) CreateProject(parentPath, name string) (string, error) {
	parent, err := NormalizeOptionalProjectPath(parentPath)
	if err != nil {
		return "", err
	}
	cleanName, err := ValidateProjectName(name)
	if err != nil {
		return "", err
	}

	projects, err := s.LoadProjects()
	if err != nil {
		return "", err
	}
	if parent != "" && !projectExists(projects, parent) {
		return "", fmt.Errorf("%w: %s", ErrProjectNotFound, parent)
	}

	newPath := joinProjectPath(parent, cleanName)
	if projectExists(projects, newPath) {
		return "", fmt.Errorf("%w: %s", ErrProjectExists, newPath)
	}

	projects = append(projects, Project{
		Path:      newPath,
		CreatedAt: time.Now().UTC(),
	})
	if err := s.SaveProjects(projects); err != nil {
		return "", err
	}
	return newPath, nil
}

func (s *Store) ProjectExists(path string) (bool, error) {
	cleanPath, err := NormalizeProjectPath(path)
	if err != nil {
		return false, err
	}
	projects, err := s.LoadProjects()
	if err != nil {
		return false, err
	}
	return projectExists(projects, cleanPath), nil
}

func (s *Store) ListChildProjects(parentPath string) ([]Project, error) {
	children, projects, err := s.loadChildProjects(parentPath)
	if err != nil {
		return nil, err
	}
	records, err := s.LoadRecords()
	if err != nil {
		return nil, err
	}
	state, err := s.LoadState()
	if err != nil {
		return nil, err
	}
	sortProjectsByRecentActivity(children, projects, records, state)
	return children, nil
}

func (s *Store) listChildProjectsByRecentActivity(parentPath string, records []Record, state *TimerState) ([]Project, error) {
	children, projects, err := s.loadChildProjects(parentPath)
	if err != nil {
		return nil, err
	}
	sortProjectsByRecentActivity(children, projects, records, state)
	return children, nil
}

func (s *Store) loadChildProjects(parentPath string) ([]Project, []Project, error) {
	parent, err := NormalizeOptionalProjectPath(parentPath)
	if err != nil {
		return nil, nil, err
	}

	projects, err := s.LoadProjects()
	if err != nil {
		return nil, nil, err
	}
	if parent != "" && !projectExists(projects, parent) {
		return nil, nil, fmt.Errorf("%w: %s", ErrProjectNotFound, parent)
	}

	return childProjects(projects, parent), projects, nil
}

func (s *Store) RenameProject(sourcePath, targetPath string) error {
	source, err := NormalizeProjectPath(sourcePath)
	if err != nil {
		return fmt.Errorf("invalid source: %w", err)
	}
	target, err := NormalizeProjectPath(targetPath)
	if err != nil {
		return fmt.Errorf("invalid target: %w", err)
	}
	if source == target {
		return errors.New("source and target are the same project")
	}
	if isProjectInSubtree(target, source) {
		return errors.New("cannot rename a project into its own subtree")
	}

	projects, err := s.LoadProjects()
	if err != nil {
		return err
	}
	if !projectExists(projects, source) {
		return fmt.Errorf("%w: %s", ErrProjectNotFound, source)
	}
	if projectExists(projects, target) {
		return fmt.Errorf("%w: %s", ErrProjectExists, target)
	}

	targetParent := ParentProjectPath(target)
	if targetParent != "" && !projectExists(projects, targetParent) {
		return fmt.Errorf("%w: target parent %s", ErrProjectNotFound, targetParent)
	}

	updatedProjects := make([]Project, 0, len(projects))
	movedPaths := map[string]bool{}
	for _, project := range projects {
		if isProjectInSubtree(project.Path, source) {
			project.Path = replaceProjectPrefix(project.Path, source, target)
			movedPaths[project.Path] = true
		}
		updatedProjects = append(updatedProjects, project)
	}

	for _, project := range updatedProjects {
		if movedPaths[project.Path] {
			continue
		}
		if isProjectInSubtree(project.Path, target) {
			return fmt.Errorf("rename would conflict with existing project %s", project.Path)
		}
	}

	records, err := s.LoadRecords()
	if err != nil {
		return err
	}
	for i := range records {
		if isProjectInSubtree(records[i].ProjectPath, source) {
			records[i].ProjectPath = replaceProjectPrefix(records[i].ProjectPath, source, target)
		}
	}

	state, err := s.LoadState()
	if err != nil {
		return err
	}
	if state != nil && isProjectInSubtree(state.ProjectPath, source) {
		state.ProjectPath = replaceProjectPrefix(state.ProjectPath, source, target)
	}

	if err := s.SaveProjects(updatedProjects); err != nil {
		return err
	}
	if err := s.SaveRecords(records); err != nil {
		return err
	}
	if state != nil {
		if err := s.SaveState(state); err != nil {
			return err
		}
	}
	return nil
}

func ParentProjectPath(projectPath string) string {
	index := strings.LastIndex(projectPath, "/")
	if index < 0 {
		return ""
	}
	return projectPath[:index]
}

func BaseProjectName(projectPath string) string {
	index := strings.LastIndex(projectPath, "/")
	if index < 0 {
		return projectPath
	}
	return projectPath[index+1:]
}

func AncestorProjectPaths(projectPath string) []string {
	var paths []string
	for current := projectPath; current != ""; current = ParentProjectPath(current) {
		paths = append(paths, current)
	}
	return paths
}

func joinProjectPath(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + "/" + name
}

func projectExists(projects []Project, path string) bool {
	for _, project := range projects {
		if project.Path == path {
			return true
		}
	}
	return false
}

func childProjects(projects []Project, parent string) []Project {
	children := make([]Project, 0)
	for _, project := range projects {
		if ParentProjectPath(project.Path) == parent {
			children = append(children, project)
		}
	}
	return children
}

func sortProjectsByRecentActivity(projects, allProjects []Project, records []Record, state *TimerState) {
	activity := projectActivityTimes(allProjects, records, state)
	sort.SliceStable(projects, func(i, j int) bool {
		left := activity[projects[i].Path]
		right := activity[projects[j].Path]
		if !left.Equal(right) {
			return left.After(right)
		}
		return projects[i].Path < projects[j].Path
	})
}

func projectActivityTimes(projects []Project, records []Record, state *TimerState) map[string]time.Time {
	activity := make(map[string]time.Time)
	for _, project := range projects {
		markProjectActivity(activity, project.Path, project.CreatedAt)
	}
	for _, record := range records {
		markProjectActivity(activity, record.ProjectPath, recordActivityTime(record))
	}
	if state != nil {
		markProjectActivity(activity, state.ProjectPath, timerStateActivityTime(*state))
	}
	return activity
}

func recordActivityTime(record Record) time.Time {
	if !record.EndedAt.IsZero() {
		return record.EndedAt
	}
	return record.StartedAt
}

func timerStateActivityTime(state TimerState) time.Time {
	if state.Status == TimerStatusRunning && !state.LastStartedAt.IsZero() {
		return state.LastStartedAt
	}
	return state.StartedAt
}

func markProjectActivity(activity map[string]time.Time, projectPath string, at time.Time) {
	if projectPath == "" || at.IsZero() {
		return
	}
	for _, path := range AncestorProjectPaths(projectPath) {
		if current := activity[path]; current.IsZero() || at.After(current) {
			activity[path] = at
		}
	}
}

func normalizeProjectPath(input string, allowRoot bool) (string, error) {
	clean := strings.Trim(strings.TrimSpace(input), "/")
	if clean == "" || clean == "." {
		if allowRoot {
			return "", nil
		}
		return "", errors.New("project path cannot be empty")
	}

	parts := strings.Split(clean, "/")
	for _, part := range parts {
		if err := validateProjectSegment(part); err != nil {
			return "", err
		}
	}
	return strings.Join(parts, "/"), nil
}

func validateProjectSegment(segment string) error {
	if segment == "" {
		return errors.New("project path cannot contain empty segments")
	}
	if segment == "." || segment == ".." {
		return errors.New("project path cannot contain . or .. segments")
	}
	for _, r := range segment {
		if unicode.IsControl(r) {
			return errors.New("project path cannot contain control characters")
		}
	}
	return nil
}

func isProjectInSubtree(projectPath, rootPath string) bool {
	return projectPath == rootPath || strings.HasPrefix(projectPath, rootPath+"/")
}

func replaceProjectPrefix(projectPath, source, target string) string {
	if projectPath == source {
		return target
	}
	return target + strings.TrimPrefix(projectPath, source)
}
