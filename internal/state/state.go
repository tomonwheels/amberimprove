// Package state keeps zustand.json: what the night run is doing right now and
// the last events. `status` and `serve` only read it.
package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MaxEvents is the number of events kept in the file.
const MaxEvents = 300

// Event is one log line. Kind is a stable key that the UI translates.
type Event struct {
	Time   time.Time `json:"time"`
	Kind   string    `json:"kind"`
	File   string    `json:"file,omitempty"`
	Detail string    `json:"detail,omitempty"`
}

// State is the content of zustand.json.
type State struct {
	// Phase: "running", "waiting", "stopped".
	Phase     string    `json:"phase"`
	Reason    string    `json:"reason,omitempty"`
	File      string    `json:"file,omitempty"`
	Pass      int       `json:"pass,omitempty"`
	Passes    int       `json:"passes,omitempty"`
	FileSize  int64     `json:"file_size,omitempty"`
	FileStart time.Time `json:"file_start,omitempty"`
	RunStart  time.Time `json:"run_start,omitempty"`
	Done      int       `json:"done_this_run"`
	Skipped   int       `json:"skipped_this_run"`
	Failed    int       `json:"failed_this_run"`
	Updated   time.Time `json:"updated"`
	Events    []Event   `json:"events"`
}

// Store writes the state file after every change.
type Store struct {
	mu   sync.Mutex
	path string
	s    State
}

// Open loads an existing state (to keep the event history) or starts fresh.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	st := &Store{path: filepath.Join(dir, "zustand.json")}
	// Keep the whole previous state: the daemon re-opens the store every
	// minute while it waits, and the counters of the last night must survive.
	if old, err := Read(dir); err == nil {
		st.s = old
	}
	return st, nil
}

// Read loads zustand.json from dir.
func Read(dir string) (State, error) {
	var s State
	b, err := os.ReadFile(filepath.Join(dir, "zustand.json"))
	if err != nil {
		return s, err
	}
	err = json.Unmarshal(b, &s)
	return s, err
}

// Update applies f and saves.
func (st *Store) Update(f func(*State)) {
	st.mu.Lock()
	defer st.mu.Unlock()
	f(&st.s)
	st.save()
}

// Log appends an event and saves.
func (st *Store) Log(kind, file, detail string) {
	st.Update(func(s *State) {
		s.Events = append(s.Events, Event{Time: time.Now(), Kind: kind, File: file, Detail: detail})
		if n := len(s.Events); n > MaxEvents {
			s.Events = s.Events[n-MaxEvents:]
		}
	})
}

func (st *Store) save() {
	st.s.Updated = time.Now()
	b, err := json.MarshalIndent(st.s, "", "  ")
	if err != nil {
		return
	}
	tmp := st.path + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		_ = os.Rename(tmp, st.path)
	}
}
