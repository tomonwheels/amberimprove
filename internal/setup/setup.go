// Package setup backs the setup page: which drives exist, which folders they
// hold, whether a candidate configuration works, and whether this machine
// keeps bufhrt's timing with the chosen quality.
package setup

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/tomonwheels/amberimprove/internal/config"
	"github.com/tomonwheels/amberimprove/internal/guard"
	"github.com/tomonwheels/amberimprove/internal/improve"
	"github.com/tomonwheels/amberimprove/internal/queue"
)

// Drive is one mounted filesystem.
type Drive struct {
	Mount  string `json:"mount"`
	Device string `json:"device"`
	FSType string `json:"fstype"`
	Label  string `json:"label"`
	Model  string `json:"model"`
	Tran   string `json:"tran"`
	Size   uint64 `json:"size"`
	Free   uint64 `json:"free"`
	// Weak marks filesystems frankl heard worse results on (FAT, exFAT, NTFS).
	Weak bool `json:"weak"`
	// System marks the root and boot filesystems.
	System bool `json:"system"`
}

type lsblkDev struct {
	Name       string     `json:"name"`
	FSType     *string    `json:"fstype"`
	Label      *string    `json:"label"`
	Mountpoint *string    `json:"mountpoint"`
	Model      *string    `json:"model"`
	Tran       *string    `json:"tran"`
	Children   []lsblkDev `json:"children"`
}

// Drives lists mounted filesystems (Linux, via lsblk).
func Drives() ([]Drive, error) {
	out, err := exec.Command("lsblk", "-J", "-o", "NAME,FSTYPE,LABEL,MOUNTPOINT,MODEL,TRAN").Output()
	if err != nil {
		return nil, fmt.Errorf("lsblk: %w", err)
	}
	var doc struct {
		Blockdevices []lsblkDev `json:"blockdevices"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		return nil, err
	}
	var drives []Drive
	var walk func(d lsblkDev, model, tran string)
	walk = func(d lsblkDev, model, tran string) {
		if d.Model != nil && *d.Model != "" {
			model = strings.TrimSpace(*d.Model)
		}
		if d.Tran != nil && *d.Tran != "" {
			tran = *d.Tran
		}
		// Boot partitions, files bound in by containers (/etc/hosts …) and
		// the configuration volume never hold music: not offered.
		if d.Mountpoint != nil && offerable(*d.Mountpoint) {
			mp := *d.Mountpoint
			dr := Drive{Mount: mp, Device: "/dev/" + d.Name, Model: model, Tran: tran,
				System: mp == "/"}
			if d.FSType != nil {
				dr.FSType = *d.FSType
			}
			if d.Label != nil {
				dr.Label = *d.Label
			}
			dr.Weak = weakFS(dr.FSType)
			var s syscall.Statfs_t
			if syscall.Statfs(mp, &s) == nil {
				dr.Size = uint64(s.Blocks) * uint64(s.Bsize)
				dr.Free = uint64(s.Bavail) * uint64(s.Bsize)
			}
			drives = append(drives, dr)
		}
		for _, ch := range d.Children {
			walk(ch, model, tran)
		}
	}
	for _, d := range doc.Blockdevices {
		walk(d, "", "")
	}
	drives = append(drives, bindMounts(drives)...)
	sort.Slice(drives, func(i, j int) bool {
		if drives[i].System != drives[j].System {
			return !drives[i].System
		}
		return drives[i].Mount < drives[j].Mount
	})
	return drives, nil
}

// networkFS reports filesystems that only reach the medium over the network.
// bufhrt's timing then ends at the network client, not on the medium.
func networkFS(t string) bool {
	switch strings.ToLower(t) {
	case "nfs", "nfs4", "cifs", "smb3", "smbfs", "sshfs", "fuse.sshfs", "9p", "davfs", "fuse.rclone", "glusterfs", "ceph":
		return true
	}
	return false
}

// bindMounts adds mounts of real devices that lsblk does not show — in a
// container the music folder is a bind mount (/music) that lsblk lists under
// the host's mount point, not the container's.
func bindMounts(known []Drive) []Drive {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil
	}
	defer f.Close()
	seen := map[string]bool{}
	for _, d := range known {
		seen[d.Mount] = true
	}
	var extra []Drive
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fs := strings.Fields(sc.Text())
		if len(fs) < 3 || !strings.HasPrefix(fs[0], "/dev/") {
			continue
		}
		mp := strings.ReplaceAll(fs[1], `\040`, " ")
		if seen[mp] || !offerable(mp) {
			continue
		}
		seen[mp] = true
		dr := Drive{Mount: mp, Device: fs[0], FSType: fs[2], Weak: weakFS(fs[2]), System: mp == "/"}
		var s syscall.Statfs_t
		if syscall.Statfs(mp, &s) == nil {
			dr.Size = uint64(s.Blocks) * uint64(s.Bsize)
			dr.Free = uint64(s.Bavail) * uint64(s.Bsize)
		}
		extra = append(extra, dr)
	}
	return extra
}

// offerable: a directory that could hold music (not /boot, /etc, /config,
// not a single file bound into a container).
func offerable(mp string) bool {
	if !strings.HasPrefix(mp, "/") || strings.HasPrefix(mp, "/boot") || strings.HasPrefix(mp, "/etc/") ||
		mp == "/config" || strings.HasPrefix(mp, "/config/") {
		return false
	}
	st, err := os.Stat(mp)
	return err == nil && st.IsDir()
}

func weakFS(t string) bool {
	switch strings.ToLower(t) {
	case "vfat", "fat", "fat32", "exfat", "ntfs", "ntfs3", "fuseblk":
		return true
	}
	return false
}

// Dirs lists the visible subdirectories of path.
func Dirs(path string) ([]string, error) {
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("Pfad muss absolut sein")
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			dirs = append(dirs, e.Name())
		}
	}
	sort.Strings(dirs)
	return dirs, nil
}

// MountOf returns the mount point and filesystem type holding path.
func MountOf(path string) (mount, fstype string) {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return "", ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fs := strings.Fields(sc.Text())
		if len(fs) < 3 {
			continue
		}
		mp := strings.ReplaceAll(fs[1], `\040`, " ")
		if (path == mp || strings.HasPrefix(path, strings.TrimSuffix(mp, "/")+"/")) && len(mp) >= len(mount) {
			mount, fstype = mp, fs[2]
		}
	}
	return mount, fstype
}

// Check is one line of the setup check. Level: "ok", "warn", "error".
type Check struct {
	Key    string `json:"key"`
	Level  string `json:"level"`
	Detail string `json:"detail,omitempty"`
}

// Report is the result of checking a candidate configuration.
type Report struct {
	Checks []Check `json:"checks"`
	Titles int     `json:"titles"`
	Albums int     `json:"albums"`
	Bytes  int64   `json:"bytes"`
	// Nights needed for the whole library with this configuration.
	Nights float64 `json:"nights"`
	OK     bool    `json:"ok"`
}

// OverheadFactor: RAM copies and verification on top of the pure writing
// time (measured on the NUC, Sept 2026: 1.43).
const OverheadFactor = 1.5

// Examine checks c without changing anything.
func Examine(ctx context.Context, c config.Config) Report {
	var r Report
	add := func(key, level, detail string) { r.Checks = append(r.Checks, Check{key, level, detail}) }

	if err := c.Validate(); err != nil {
		add("config", "error", err.Error())
	}
	ms, err := os.Stat(c.Music)
	if err != nil || !ms.IsDir() {
		add("music", "error", c.Music)
	} else {
		titles, albums, err := queue.Scan(c)
		if err != nil {
			add("music", "error", err.Error())
		} else {
			r.Titles, r.Albums = len(titles), len(albums)
			for _, t := range titles {
				r.Bytes += t.Size
			}
			level := "ok"
			if len(titles) == 0 {
				level = "warn"
			}
			add("music", level, fmt.Sprintf("%d / %d", len(titles), len(albums)))
		}
		if _, fstype := MountOf(c.Music); networkFS(fstype) {
			add("network", "error", fstype)
		} else if weakFS(fstype) {
			add("filesystem", "warn", fstype)
		} else if fstype != "" {
			add("filesystem", "ok", fstype)
		}
		if sameFS(c.Music, c.WorkDir) {
			add("workdir", "ok", c.WorkDir)
		} else {
			add("workdir", "error", c.WorkDir)
		}
	}

	switch c.Source.Kind {
	case "self":
		add("source", "ok", "self")
	case "path":
		if st, err := os.Stat(c.Source.Path); err == nil && st.IsDir() {
			add("source", "ok", c.Source.Path)
		} else {
			add("source", "error", c.Source.Path)
		}
	case "rsync":
		cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		cmd := exec.CommandContext(cctx, "rsync", "--list-only", "-e", strings.Join(quoteAll(append([]string{c.Source.SSHProgram()}, c.Source.SSHArgs()...)), " "),
			strings.TrimSuffix(c.Source.Remote, "/")+"/")
		out, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			add("source", "error", strings.TrimSpace(lastLine(string(out))))
		} else {
			add("source", "ok", c.Source.Remote)
		}
	}

	if out, err := exec.Command(c.Bufhrt.Program, "--version").CombinedOutput(); err != nil {
		add("bufhrt", "error", c.Bufhrt.Program)
	} else {
		add("bufhrt", "ok", strings.TrimSpace(lastLine(string(out))))
	}

	if c.Playback.URL == "" {
		add("player", "warn", "")
	} else if _, why := guard.Playing(ctx, c.Playback); why == guard.ReasonNoAnswer {
		add("player", "error", c.Playback.URL)
	} else {
		add("player", "ok", c.Playback.URL)
	}

	if c.Bufhrt.BytesPerSecond > 0 && r.Bytes > 0 {
		from, _ := config.ParseClock(c.Window.From)
		to, _ := config.ParseClock(c.Window.To)
		win := float64((to-from+24*60)%(24*60)) * 60
		if win == 0 {
			win = 24 * 3600
		}
		r.Nights = float64(r.Bytes) * float64(c.Passes) * OverheadFactor / float64(c.Bufhrt.BytesPerSecond) / win
	}

	r.OK = true
	for _, ch := range r.Checks {
		if ch.Level == "error" {
			r.OK = false
		}
	}
	return r
}

// Timing writes 1 MB with the given parameters into the work directory and
// reports how many blocks bufhrt wrote late on this machine.
type Timing struct {
	Millis    int64  `json:"millis"`
	Delayed   int    `json:"delayed"`
	Identical bool   `json:"identical"`
	Error     string `json:"error,omitempty"`
}

// MeasureTiming runs the timing test.
func MeasureTiming(ctx context.Context, c config.Config) Timing {
	// Measured on the chosen drive — without one there is nothing to measure.
	if c.WorkDir == "" || !filepath.IsAbs(c.WorkDir) {
		return Timing{Error: "choose_drive_first"}
	}
	if err := os.MkdirAll(c.WorkDir, 0o700); err != nil {
		return Timing{Error: err.Error()}
	}
	data := make([]byte, 1<<20)
	_, _ = rand.Read(data)
	in := filepath.Join(os.TempDir(), "amberimprove-taktprobe")
	out := filepath.Join(c.WorkDir, "taktprobe")
	defer os.Remove(in)
	defer os.Remove(out)
	os.Remove(out)
	if err := os.WriteFile(in, data, 0o600); err != nil {
		return Timing{Error: err.Error()}
	}
	start := time.Now()
	n, err := improve.RunBufhrt(ctx, c.Bufhrt, in, out)
	t := Timing{Millis: time.Since(start).Milliseconds(), Delayed: n}
	if err != nil {
		t.Error = err.Error()
		return t
	}
	got, err := os.ReadFile(out)
	t.Identical = err == nil && bytes.Equal(got, data)
	return t
}

func sameFS(a, b string) bool {
	// b may not exist yet: compare its nearest existing parent.
	for {
		if _, err := os.Stat(b); err == nil {
			break
		}
		parent := filepath.Dir(b)
		if parent == b {
			return false
		}
		b = parent
	}
	var sa, sb syscall.Stat_t
	if syscall.Stat(a, &sa) != nil || syscall.Stat(b, &sb) != nil {
		return false
	}
	return sa.Dev == sb.Dev
}

func quoteAll(args []string) []string {
	q := make([]string, len(args))
	for i, a := range args {
		q[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
	}
	return q
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return lines[len(lines)-1]
}
