package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"

	"tactile-atlas-gate/internal/domain"
)

type FileStore struct {
	root  string
	mu    sync.Mutex
	cache map[string]domain.Aggregate
}

func NewFileStore(root string) (*FileStore, error) {
	if root == "" {
		return nil, fmt.Errorf("数据目录不能为空")
	}
	if err := os.MkdirAll(filepath.Join(root, "projects"), 0700); err != nil {
		return nil, err
	}
	return &FileStore{root: root, cache: map[string]domain.Aggregate{}}, nil
}

var safeID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

func (s *FileStore) projectDir(id string) (string, error) {
	if !safeID.MatchString(id) {
		return "", fmt.Errorf("%w: project_id 格式非法", domain.ErrInvalid)
	}
	return filepath.Join(s.root, "projects", id), nil
}

func (s *FileStore) Create(a domain.Aggregate, event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir, err := s.projectDir(a.Project.ProjectID)
	if err != nil {
		return err
	}
	if _, err = os.Stat(dir); err == nil {
		return domain.ErrConflict
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err = os.Mkdir(dir, 0700); err != nil {
		return err
	}
	if err = writeSnapshot(dir, a); err != nil {
		return err
	}
	if err = appendEvent(dir, event); err != nil {
		return err
	}
	s.cache[a.Project.ProjectID] = a
	return nil
}
func (s *FileStore) Load(id string) (domain.Aggregate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cached, ok := s.cache[id]; ok {
		dir, err := s.projectDir(id)
		if err != nil {
			return domain.Aggregate{}, err
		}
		if err = verifyEvents(dir); err != nil {
			return domain.Aggregate{}, err
		}
		if err = verifyEvidence(dir, cached); err != nil {
			return domain.Aggregate{}, err
		}
		// Return a deep copy so that callers mutating the returned
		// aggregate (e.g. appending proofs or recording idempotency)
		// cannot corrupt the cached snapshot. This is critical when a
		// subsequent Save fails: the cache must keep reflecting the last
		// successfully persisted state, not the in-flight mutation.
		return cloneAggregate(cached), nil
	}
	a, err := s.loadLocked(id)
	if err == nil {
		s.cache[id] = a
	}
	return a, err
}

// cloneAggregate produces a value-independent copy of a by marshalling and
// unmarshalling. Aggregate holds reference-typed fields (maps and slices)
// that a struct value copy would share, allowing in-flight mutations to leak
// into the cache. The JSON round-trip allocates fresh copies of every
// reference-typed field, matching the isolation provided by loadLocked when
// reading from disk.
func cloneAggregate(a domain.Aggregate) domain.Aggregate {
	b, err := json.Marshal(a)
	if err != nil {
		return a
	}
	var copy domain.Aggregate
	if err = json.Unmarshal(b, &copy); err != nil {
		return a
	}
	if copy.Idempotency == nil {
		copy.Idempotency = map[string]domain.IdempotencyRecord{}
	}
	return copy
}
func (s *FileStore) loadLocked(id string) (domain.Aggregate, error) {
	dir, err := s.projectDir(id)
	if err != nil {
		return domain.Aggregate{}, err
	}
	if err = verifyEvents(dir); err != nil {
		return domain.Aggregate{}, err
	}
	b, err := os.ReadFile(filepath.Join(dir, "snapshot.json"))
	if errors.Is(err, os.ErrNotExist) {
		return domain.Aggregate{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Aggregate{}, err
	}
	var a domain.Aggregate
	if err = json.Unmarshal(b, &a); err != nil {
		return a, fmt.Errorf("聚合快照损坏: %w", err)
	}
	if a.Idempotency == nil {
		a.Idempotency = map[string]domain.IdempotencyRecord{}
	}
	if err = verifyEvidence(dir, a); err != nil {
		return domain.Aggregate{}, err
	}
	return a, nil
}
func (s *FileStore) Save(a domain.Aggregate, expected int64, event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.loadLocked(a.Project.ProjectID)
	if err != nil {
		return err
	}
	if current.Project.Revision != expected {
		return domain.ErrConflict
	}
	if err = verifyImmutableTransition(current, a); err != nil {
		return err
	}
	dir, _ := s.projectDir(a.Project.ProjectID)
	if err = writeNewEvidence(dir, current, a); err != nil {
		return err
	}
	if err = writeSnapshot(dir, a); err != nil {
		return err
	}
	if err = appendEvent(dir, event); err != nil {
		return err
	}
	s.cache[a.Project.ProjectID] = a
	return nil
}
func (s *FileStore) List() ([]domain.Aggregate, error) {
	entries, err := os.ReadDir(filepath.Join(s.root, "projects"))
	if err != nil {
		return nil, err
	}
	out := []domain.Aggregate{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		a, err := s.Load(e.Name())
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Project.CreatedAt.Before(out[j].Project.CreatedAt) })
	return out, nil
}
