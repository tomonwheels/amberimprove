// Package queue walks the music library and classifies every title against
// the album manifests.
package queue

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/tomonwheels/amberimprove/internal/config"
	"github.com/tomonwheels/amberimprove/internal/manifest"
)

// Title is one audio file below the music root.
type Title struct {
	Rel    string          `json:"rel"`
	Size   int64           `json:"size"`
	Status manifest.Status `json:"status"`
}

// Album summarises one directory.
type Album struct {
	Dir   string `json:"dir"`
	Total int    `json:"total"`
	Done  int    `json:"done"`
	Older int    `json:"older"`
}

// Scan returns all titles (sorted by path) and the per-album summary.
func Scan(c config.Config) ([]Title, []Album, error) {
	var titles []Title
	albums := map[string]*Album{}
	manifests := map[string]manifest.Manifest{}

	err := filepath.WalkDir(c.Music, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable directory: skip, do not abort the night
		}
		if d.IsDir() {
			if p != c.Music && len(d.Name()) > 0 && d.Name()[0] == '.' {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || !c.IsAudio(d.Name()) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		dir := filepath.Dir(p)
		m, ok := manifests[dir]
		if !ok {
			m, _ = manifest.Read(dir)
			manifests[dir] = m
		}
		rel, _ := filepath.Rel(c.Music, p)
		st := m.StatusOf(d.Name(), info.Size(), info.ModTime(), c.Method)
		titles = append(titles, Title{Rel: rel, Size: info.Size(), Status: st})

		relDir, _ := filepath.Rel(c.Music, dir)
		a := albums[relDir]
		if a == nil {
			a = &Album{Dir: relDir}
			albums[relDir] = a
		}
		a.Total++
		switch st {
		case manifest.Done:
			a.Done++
		case manifest.Older:
			a.Older++
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	if _, err := os.Stat(c.Music); err != nil {
		return nil, nil, err
	}
	sort.Slice(titles, func(i, j int) bool { return titles[i].Rel < titles[j].Rel })
	list := make([]Album, 0, len(albums))
	for _, a := range albums {
		list = append(list, *a)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Dir < list[j].Dir })
	return titles, list, nil
}

// Pending returns the titles that still need work: open first, then those
// improved with an older method.
func Pending(titles []Title) []Title {
	var open, older []Title
	for _, t := range titles {
		switch t.Status {
		case manifest.Open:
			open = append(open, t)
		case manifest.Older:
			older = append(older, t)
		}
	}
	return append(open, older...)
}
