// Package manifest reads and writes .amberimprove.json, the per-album record
// of improved titles. It is the only interface to amberSUITE: the suite reads
// this file (data), it never calls amberIMPROVE.
package manifest

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// FileName is the manifest name inside each album directory.
const FileName = ".amberimprove.json"

// Format is the manifest format version.
const Format = 1

// Manifest is the content of one .amberimprove.json.
type Manifest struct {
	Tool   string `json:"tool"`
	Format int    `json:"format"`
	// StatusURL: amberIMPROVE's status page, for a link back from players.
	StatusURL string           `json:"status_url,omitempty"`
	Files     map[string]Entry `json:"files"`
}

// Entry records one improved title.
type Entry struct {
	Size       int64          `json:"size"`
	Mtime      time.Time      `json:"mtime"`
	SHA256     string         `json:"sha256"`
	ImprovedAt time.Time      `json:"improved_at"`
	Method     string         `json:"method"`
	Passes     int            `json:"passes"`
	Host       string         `json:"host"`
	Params     map[string]any `json:"params"`
	// Delayed counts blocks bufhrt wrote late, summed over all passes
	// (0 = every block on time).
	Delayed int `json:"delayed_blocks"`
}

// Status of a title relative to its manifest entry.
type Status int

const (
	// Open: no entry, or the file changed since it was improved.
	Open Status = iota
	// Older: improved, but with another method.
	Older
	// Done: improved with the current method, file unchanged.
	Done
)

// Read loads the manifest of dir; a missing file yields an empty manifest.
func Read(dir string) (Manifest, error) {
	m := Manifest{Tool: "amberIMPROVE", Format: Format, Files: map[string]Entry{}}
	b, err := os.ReadFile(filepath.Join(dir, FileName))
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, err
	}
	if m.Files == nil {
		m.Files = map[string]Entry{}
	}
	return m, nil
}

// Write stores m atomically (temp file + rename) and gives it the owner of
// the album directory, so the player's user can read it.
func Write(dir string, m Manifest) error {
	m.Tool, m.Format = "amberIMPROVE", Format
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, FileName+".tmp")
	if err := os.WriteFile(tmp, append(b, '\n'), 0o664); err != nil {
		return err
	}
	if st, err := os.Stat(dir); err == nil {
		chownLike(tmp, st)
	}
	return os.Rename(tmp, filepath.Join(dir, FileName))
}

// StatusOf compares a file (size, mtime) with its entry.
func (m Manifest) StatusOf(name string, size int64, mtime time.Time, method string) Status {
	e, ok := m.Files[name]
	if !ok || e.Size != size || !e.Mtime.Equal(mtime) {
		return Open
	}
	if e.Method != method {
		return Older
	}
	return Done
}

// StatusFileName marks a music folder as looked after by amberIMPROVE. It
// sits in the music root and tells players where the status page is — so
// they can offer a link even before the first album is improved.
const StatusFileName = ".amberimprove-status.json"

// FolderStatus is the content of StatusFileName.
type FolderStatus struct {
	Tool      string `json:"tool"`
	StatusURL string `json:"status_url"`
	Method    string `json:"method,omitempty"`
}

// WriteStatus writes the status file into music if its content changed.
// An empty statusURL removes nothing and writes nothing.
func WriteStatus(music, statusURL, method string) error {
	if statusURL == "" {
		return nil
	}
	b, err := json.MarshalIndent(FolderStatus{Tool: "amberIMPROVE", StatusURL: statusURL, Method: method}, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	path := filepath.Join(music, StatusFileName)
	if old, err := os.ReadFile(path); err == nil && string(old) == string(b) {
		return nil
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o664); err != nil {
		return err
	}
	if st, err := os.Stat(music); err == nil {
		chownLike(tmp, st)
	}
	return os.Rename(tmp, path)
}
