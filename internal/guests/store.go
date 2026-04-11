package guests

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"crypto/rand"
	"encoding/hex"
)

var ErrNotFound = errors.New("guest not found")

type Guest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
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
