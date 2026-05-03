package strata

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Store struct {
	dir string
}

func NewStoreFromEnv() (*Store, error) {
	if dir := strings.TrimSpace(os.Getenv("STRATA_HOME")); dir != "" {
		return NewStore(dir), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("find home directory: %w", err)
	}
	return NewStore(filepath.Join(home, ".strata")), nil
}

func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

func (s *Store) Dir() string {
	return s.dir
}

func (s *Store) LoadProjects() ([]Project, error) {
	var file projectsFile
	if err := readJSONIfExists(s.projectsPath(), &file); err != nil {
		return nil, err
	}
	sortProjects(file.Projects)
	return file.Projects, nil
}

func (s *Store) SaveProjects(projects []Project) error {
	if err := s.ensureDir(); err != nil {
		return err
	}

	copied := append([]Project(nil), projects...)
	sortProjects(copied)
	data, err := json.MarshalIndent(projectsFile{Projects: copied}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode projects: %w", err)
	}
	data = append(data, '\n')
	return writeFileAtomic(s.projectsPath(), data, 0o644)
}

func (s *Store) LoadRecords() ([]Record, error) {
	file, err := os.Open(s.recordsPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open records: %w", err)
	}
	defer file.Close()

	var records []Record
	scanner := bufio.NewScanner(file)
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		var record Record
		if err := json.Unmarshal([]byte(text), &record); err != nil {
			return nil, fmt.Errorf("decode records line %d: %w", line, err)
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read records: %w", err)
	}
	return records, nil
}

func (s *Store) AppendRecord(record Record) error {
	if err := s.ensureDir(); err != nil {
		return err
	}

	file, err := os.OpenFile(s.recordsPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open records: %w", err)
	}
	defer file.Close()

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode record: %w", err)
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write record: %w", err)
	}
	return nil
}

func (s *Store) SaveRecords(records []Record) error {
	if err := s.ensureDir(); err != nil {
		return err
	}

	var builder strings.Builder
	for _, record := range records {
		data, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("encode record %q: %w", record.ID, err)
		}
		builder.Write(data)
		builder.WriteByte('\n')
	}
	return writeFileAtomic(s.recordsPath(), []byte(builder.String()), 0o644)
}

func (s *Store) LoadState() (*TimerState, error) {
	var state TimerState
	if err := readJSONIfExists(s.statePath(), &state); err != nil {
		return nil, err
	}
	if state.Status == "" {
		return nil, nil
	}
	return &state, nil
}

func (s *Store) SaveState(state *TimerState) error {
	if state == nil {
		return s.ClearState()
	}
	if err := s.ensureDir(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode timer state: %w", err)
	}
	data = append(data, '\n')
	return writeFileAtomic(s.statePath(), data, 0o644)
}

func (s *Store) ClearState() error {
	if err := os.Remove(s.statePath()); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("clear timer state: %w", err)
	}
	return nil
}

func (s *Store) ensureDir() error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	return nil
}

func (s *Store) projectsPath() string {
	return filepath.Join(s.dir, "projects.json")
}

func (s *Store) recordsPath() string {
	return filepath.Join(s.dir, "records.jsonl")
}

func (s *Store) statePath() string {
	return filepath.Join(s.dir, "state.json")
}

func readJSONIfExists(filename string, target any) error {
	data, err := os.ReadFile(filename)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", filepath.Base(filename), err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("decode %s: %w", filepath.Base(filename), err)
	}
	return nil
}

func writeFileAtomic(filename string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(filename)
	temp, err := os.CreateTemp(dir, "."+filepath.Base(filename)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)

	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := temp.Chmod(perm); err != nil {
		temp.Close()
		return fmt.Errorf("set temp permissions: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tempName, filename); err != nil {
		return fmt.Errorf("replace %s: %w", filepath.Base(filename), err)
	}
	return nil
}

func sortProjects(projects []Project) {
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Path < projects[j].Path
	})
}
