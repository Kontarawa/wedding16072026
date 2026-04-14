package guests

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"crypto/rand"
	"encoding/hex"
)

var ErrNotFound = errors.New("guest not found")

type Guest struct {
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	Salutation string `json:"salutation,omitempty"`
}

// GuestEntry — гость со ссылочным hash (для админки).
type GuestEntry struct {
	Hash       string `json:"hash"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	Salutation string `json:"salutation,omitempty"`
}

type Store struct {
	mu   sync.RWMutex
	path string
	data map[string]Guest
}

func New(path string) (*Store, error) {
	s := &Store{
		path: path,
		data: make(map[string]Guest),
	}
	if err := s.load(); err != nil {
		if os.IsNotExist(err) {
			dir := filepath.Dir(path)
			if dir != "." && dir != "" {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return nil, err
				}
			}
			return s, nil
		}
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	b, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	var m map[string]Guest
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	s.data = m
	return nil
}

func (s *Store) persistLocked() error {
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func newHash() (string, error) {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

func (s *Store) Create(g Guest) (hash string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	h, err := newHash()
	if err != nil {
		return "", err
	}
	for {
		if _, ok := s.data[h]; !ok {
			break
		}
		h, err = newHash()
		if err != nil {
			return "", err
		}
	}
	s.data[h] = g
	if err := s.persistLocked(); err != nil {
		delete(s.data, h)
		return "", err
	}
	return h, nil
}

func (s *Store) Get(hash string) (Guest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.data[hash]
	if !ok {
		return Guest{}, ErrNotFound
	}
	return g, nil
}

// Wipe очищает всех гостей и перезаписывает файл пустым объектом.
func (s *Store) Wipe() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = make(map[string]Guest)
	return s.persistLocked()
}

// All возвращает всех гостей, отсортировано по hash.
func (s *Store) All() []GuestEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]GuestEntry, 0, len(keys))
	for _, h := range keys {
		g := s.data[h]
		out = append(out, GuestEntry{Hash: h, FirstName: g.FirstName, LastName: g.LastName, Salutation: g.Salutation})
	}
	return out
}
