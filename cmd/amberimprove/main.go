// Command amberimprove rewrites the music files on the playback medium with
// frankl's improvefile procedure, title by title, at night — bit-identical,
// verified after every pass — and records each improved title in the album's
// .amberimprove.json.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/tomonwheels/amberimprove/internal/config"
	"github.com/tomonwheels/amberimprove/internal/guard"
	"github.com/tomonwheels/amberimprove/internal/improve"
	"github.com/tomonwheels/amberimprove/internal/manifest"
	"github.com/tomonwheels/amberimprove/internal/queue"
	"github.com/tomonwheels/amberimprove/internal/setup"
	"github.com/tomonwheels/amberimprove/internal/state"
	"github.com/tomonwheels/amberimprove/web"
)

// version is set at build time (-ldflags "-X main.version=…").
var version = "dev"

const usage = `amberIMPROVE — improvefile für die Musikbibliothek (GPL-3.0-or-later)

  amberimprove version                         Version anzeigen

  amberimprove [-config DATEI] run            Nachtlauf (im Zeitfenster, Titel für Titel)
  amberimprove [-config DATEI] once PFAD      genau einen Titel improven (PFAD relativ zu music)
  amberimprove [-config DATEI] plan           Warteschlange anzeigen, nichts ändern
  amberimprove [-config DATEI] status         Zustand des Nachtlaufs
  amberimprove [-config DATEI] serve [-listen :8093]   Statusseite im Browser
  amberimprove [-config DATEI] all   [-listen :8093]   Statusseite und Nachtlauf in einem
                                                     Prozess (für Container ohne systemd)
`

func main() {
	defCfg := "/etc/amberimprove/config.json"
	if env := os.Getenv("AMBERIMPROVE_CONFIG"); env != "" {
		defCfg = env
	}
	cfgPath := flag.String("config", defCfg, "Konfigurationsdatei")
	flag.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(2)
	}
	args := flag.Args()[1:]
	if flag.Arg(0) == "version" {
		fmt.Println("amberIMPROVE", version)
		return
	}
	load := func() config.Config {
		cfg, err := config.Load(*cfgPath)
		if err != nil {
			log.Fatal(err)
		}
		return cfg
	}
	switch flag.Arg(0) {
	case "run":
		os.Exit(daemon(*cfgPath))
	case "serve", "all":
		fsf := flag.NewFlagSet(flag.Arg(0), flag.ExitOnError)
		listen := fsf.String("listen", ":8093", "Adresse der Statusseite")
		_ = fsf.Parse(args)
		if flag.Arg(0) == "all" {
			go serve(*cfgPath, *listen)
			os.Exit(daemon(*cfgPath))
		}
		serve(*cfgPath, *listen)
		return
	}
	cfg := load()
	switch flag.Arg(0) {
	case "once":
		if len(args) != 1 {
			log.Fatal("once braucht genau einen Pfad (relativ zu music)")
		}
		os.Exit(once(cfg, args[0]))
	case "plan":
		plan(cfg)
	case "status":
		status(cfg)
	default:
		flag.Usage()
		os.Exit(2)
	}
}

// signalContext is cancelled on SIGINT/SIGTERM (systemd stop).
func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
}

// processOne improves one title and records it. It returns the error of the
// improve step (nil, *improve.Skip, context error or failure).
func processOne(ctx context.Context, cfg config.Config, st *state.Store, t queue.Title) error {
	st.Update(func(s *state.State) {
		s.Phase, s.Reason = "running", ""
		s.File, s.FileSize, s.FileStart = t.Rel, t.Size, time.Now()
		s.Pass, s.Passes = 0, cfg.Passes
	})
	st.Log("start", t.Rel, "")
	start := time.Now()
	entry, err := improve.Title(ctx, cfg, t.Rel, improve.Hooks{
		Pass: func(p int) { st.Update(func(s *state.State) { s.Pass = p }) },
		Retry: func(p, try int) {
			st.Log("retry", t.Rel, fmt.Sprintf("%d/%d", p, try))
		},
		Delayed: func(p, n int) {
			st.Log("delayed", t.Rel, fmt.Sprintf("%d: %d", p, n))
		},
	})
	defer st.Update(func(s *state.State) { s.File, s.Pass, s.FileSize = "", 0, 0 })

	var skip *improve.Skip
	switch {
	case err == nil:
		dir := filepath.Join(cfg.Music, filepath.Dir(t.Rel))
		m, rerr := manifest.Read(dir)
		if rerr != nil {
			m = manifest.Manifest{Files: map[string]manifest.Entry{}}
		}
		m.Files[filepath.Base(t.Rel)] = entry
		m.StatusURL = cfg.StatusURL
		if werr := manifest.Write(dir, m); werr != nil {
			st.Log("manifest_failed", t.Rel, werr.Error())
			st.Update(func(s *state.State) { s.Failed++ })
			return werr
		}
		st.Log("done", t.Rel, time.Since(start).Round(time.Second).String())
		st.Update(func(s *state.State) { s.Done++ })
	case errors.As(err, &skip):
		st.Log("skipped_"+skip.Reason, t.Rel, skip.Detail)
		st.Update(func(s *state.State) { s.Skipped++ })
	case ctx.Err() != nil:
		st.Log("interrupted", t.Rel, context.Cause(ctx).Error())
	default:
		st.Log("failed", t.Rel, err.Error())
		st.Update(func(s *state.State) { s.Failed++ })
	}
	return err
}

// watch cancels the running title as soon as work is no longer allowed
// (playback starts, blocked span begins, window ends).
func watch(ctx context.Context, cfg config.Config, cancel context.CancelCauseFunc) {
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if ok, why := guard.May(ctx, cfg, time.Now()); !ok {
				cancel(errors.New(why))
				return
			}
		}
	}
}

// night works through the queue until the window ends, nothing is left or
// something is broken. It returns the event key describing why it ended.
func night(ctx context.Context, cfg config.Config, st *state.Store) string {
	st.Update(func(s *state.State) {
		*s = state.State{Events: s.Events, Phase: "running", RunStart: time.Now()}
	})
	st.Log("run_start", "", cfg.Method)

	skipped := map[string]bool{} // not retried in this night
	failures := 0
	for ctx.Err() == nil {
		now := time.Now()
		if !guard.InWindow(cfg.Window, now) {
			return "window_over"
		}
		if ok, why := guard.May(ctx, cfg, now); !ok {
			st.Update(func(s *state.State) { s.Phase, s.Reason = "waiting", why })
			sleep(ctx, time.Minute)
			continue
		}
		titles, _, err := queue.Scan(cfg)
		if err != nil {
			st.Log("scan_failed", "", err.Error())
			return "scan_failed"
		}
		var next *queue.Title
		for _, t := range queue.Pending(titles) {
			if !skipped[t.Rel] {
				t := t
				next = &t
				break
			}
		}
		if next == nil {
			return "queue_empty"
		}
		// Only start a title that can finish before the window ends; an
		// abandoned title costs the whole night's work on it.
		need := estimate(cfg, next.Size)
		if need > windowLength(cfg.Window) {
			// Would never fit into one night: report it instead of stalling
			// the whole queue behind it.
			st.Log("too_big_for_window", next.Rel, need.Round(time.Minute).String())
			skipped[next.Rel] = true
			continue
		}
		if left := guard.UntilEnd(cfg.Window, time.Now()); need > left {
			st.Log("no_time_left", next.Rel, fmt.Sprintf("%s > %s", need.Round(time.Minute), left.Round(time.Minute)))
			return "no_time_left"
		}

		fctx, fcancel := context.WithCancelCause(ctx)
		go watch(fctx, cfg, fcancel)
		err = processOne(fctx, cfg, st, *next)
		interrupted := fctx.Err() != nil
		fcancel(nil)

		var skip *improve.Skip
		switch {
		case err == nil:
			failures = 0
		case interrupted:
			// playback, blocked span or window end: the loop waits or ends.
		case errors.As(err, &skip):
			skipped[next.Rel] = true
		default:
			skipped[next.Rel] = true
			failures++
			if failures >= 3 {
				return "too_many_failures"
			}
		}
	}
	return "run_stopped"
}

// daemon is the permanent service: it waits for the window, works through
// the night and re-reads the configuration every time, so changes from the
// setup page apply from the next title on without a restart.
func daemon(cfgPath string) int {
	ctx, stop := signalContext()
	defer stop()
	hinweis := false
	for ctx.Err() == nil {
		// Fresh installation: nothing to do until the setup page saved a
		// configuration. Say so once, then wait quietly.
		if !config.Exists(cfgPath) {
			if !hinweis {
				log.Printf("noch keine Konfiguration (%s) — bitte die Setup-Seite öffnen", cfgPath)
				hinweis = true
			}
			sleep(ctx, time.Minute)
			continue
		}
		cfg, err := config.Load(cfgPath)
		if err != nil {
			log.Print(err)
			sleep(ctx, 5*time.Minute)
			continue
		}
		st, err := state.Open(cfg.StateDir)
		if err != nil {
			log.Print(err)
			sleep(ctx, 5*time.Minute)
			continue
		}
		// Tell players where the status page is (only written when it changed).
		if err := manifest.WriteStatus(cfg.Music, cfg.StatusURL, cfg.Method); err != nil {
			log.Print(err)
		}
		if !guard.InWindow(cfg.Window, time.Now()) {
			st.Update(func(s *state.State) { s.Phase, s.Reason, s.File = "waiting", guard.ReasonOutside, "" })
			sleep(ctx, time.Minute)
			continue
		}
		why := night(ctx, cfg, st)
		if why != "no_time_left" { // night() already logged it with the title
			st.Log(why, "", "")
		}
		st.Update(func(s *state.State) { s.Phase, s.Reason = "waiting", why })
		// Done for this night: wait until the window is over (or, with a
		// 24-hour window, half an hour) before looking for new titles.
		if windowLength(cfg.Window) >= 24*time.Hour {
			sleep(ctx, 30*time.Minute)
			continue
		}
		for ctx.Err() == nil && guard.InWindow(cfg.Window, time.Now()) {
			sleep(ctx, time.Minute)
		}
	}
	return 0
}

// overheadFactor: RAM copies and verification on top of the pure writing
// time (measured on the NUC, Sept 2026).
const overheadFactor = 1.5

// windowLength is the length of the nightly window.
func windowLength(w config.Window) time.Duration {
	from, _ := config.ParseClock(w.From)
	to, _ := config.ParseClock(w.To)
	m := (to - from + 24*60) % (24 * 60)
	if m == 0 {
		m = 24 * 60
	}
	return time.Duration(m) * time.Minute
}

// estimate is the expected duration of one title with all passes.
func estimate(cfg config.Config, size int64) time.Duration {
	sec := float64(size) * float64(cfg.Passes) / float64(cfg.Bufhrt.BytesPerSecond) * overheadFactor
	return time.Duration(sec * float64(time.Second))
}

func once(cfg config.Config, rel string) int {
	ctx, stop := signalContext()
	defer stop()
	if playing, why := guard.Playing(ctx, cfg.Playback); playing {
		fmt.Fprintln(os.Stderr, "Nicht gestartet:", why)
		return 1
	}
	st, err := state.Open(cfg.StateDir)
	if err != nil {
		log.Print(err)
		return 1
	}
	info, err := os.Stat(filepath.Join(cfg.Music, rel))
	if err != nil {
		log.Print(err)
		return 1
	}
	fctx, fcancel := context.WithCancelCause(ctx)
	defer fcancel(nil)
	onceCfg := cfg
	onceCfg.Window = config.Window{From: "00:00", To: "00:00"} // once ignores the window
	go watch(fctx, onceCfg, fcancel)
	start := time.Now()
	err = processOne(fctx, cfg, st, queue.Title{Rel: rel, Size: info.Size()})
	st.Update(func(s *state.State) { s.Phase = "stopped" })
	if err != nil {
		fmt.Fprintln(os.Stderr, "Fehler:", err)
		return 1
	}
	fmt.Printf("improvt: %s (%s)\n", rel, time.Since(start).Round(time.Second))
	return 0
}

func plan(cfg config.Config) {
	titles, albums, err := queue.Scan(cfg)
	if err != nil {
		log.Fatal(err)
	}
	pending := queue.Pending(titles)
	var bytes int64
	for _, t := range pending {
		bytes += t.Size
	}
	full := 0
	for _, a := range albums {
		if a.Done == a.Total {
			full++
		}
	}
	perPass := float64(bytes) / float64(cfg.Bufhrt.BytesPerSecond)
	fmt.Printf("Titel gesamt: %d · offen: %d (%.1f GB) · Alben: %d, davon vollständig improvt: %d\n",
		len(titles), len(pending), float64(bytes)/1e9, len(albums), full)
	fmt.Printf("Reine Schreibzeit bei %d Durchgängen: ca. %.1f Stunden\n",
		cfg.Passes, perPass*float64(cfg.Passes)/3600)
	for i, t := range pending {
		if i == 20 {
			fmt.Printf("… und %d weitere\n", len(pending)-20)
			break
		}
		fmt.Println("  ", t.Rel)
	}
}

func status(cfg config.Config) {
	s, err := state.Read(cfg.StateDir)
	if err != nil {
		fmt.Println("Noch kein Lauf.")
		return
	}
	fmt.Printf("Zustand: %s %s (Stand %s)\n", s.Phase, s.Reason, s.Updated.Format("02.01. 15:04:05"))
	if s.File != "" {
		fmt.Printf("Titel: %s — Durchgang %d/%d seit %s\n", s.File, s.Pass, s.Passes, s.FileStart.Format("15:04"))
	}
	fmt.Printf("Dieser Lauf: %d improvt, %d übersprungen, %d Fehler\n", s.Done, s.Skipped, s.Failed)
	from := len(s.Events) - 15
	if from < 0 {
		from = 0
	}
	for _, e := range s.Events[from:] {
		fmt.Printf("  %s %-24s %s %s\n", e.Time.Format("02.01. 15:04"), e.Kind, e.File, e.Detail)
	}
}

// ── Statusseite ─────────────────────────────────────────────────────────

type overview struct {
	Configured bool          `json:"configured"`
	State      state.State   `json:"state"`
	Albums     []queue.Album `json:"albums"`
	Titles     int           `json:"titles"`
	Done       int           `json:"done"`
	Older      int           `json:"older"`
	Pending    int64         `json:"pending_bytes"`
	Scanned    time.Time     `json:"scanned"`
	Method     string        `json:"method"`
	Passes     int           `json:"passes"`
	Window     config.Window `json:"window"`
	BPS        int64         `json:"bytes_per_second"`
}

func serve(cfgPath, listen string) {
	var mu sync.Mutex
	var cached overview
	var cachedFor string
	current := func() (config.Config, bool) {
		if cfg, err := config.Load(cfgPath); err == nil {
			return cfg, true
		}
		return config.Default(), false
	}
	scan := func(cfg config.Config) overview {
		mu.Lock()
		defer mu.Unlock()
		if time.Since(cached.Scanned) > 30*time.Second || cachedFor != cfg.Music+cfg.Method {
			titles, albums, err := queue.Scan(cfg)
			if err == nil {
				o := overview{Albums: albums, Titles: len(titles), Scanned: time.Now()}
				for _, t := range titles {
					switch t.Status {
					case manifest.Done:
						o.Done++
					default:
						if t.Status == manifest.Older {
							o.Older++
						}
						o.Pending += t.Size
					}
				}
				cached, cachedFor = o, cfg.Music+cfg.Method
			}
		}
		return cached
	}

	sub, _ := fs.Sub(web.FS, ".")
	http.Handle("/", http.FileServer(http.FS(sub)))
	http.HandleFunc("/api/overview", func(w http.ResponseWriter, r *http.Request) {
		cfg, ok := current()
		if !ok {
			writeJSON(w, map[string]any{"configured": false})
			return
		}
		o := scan(cfg)
		o.Configured = true
		o.State, _ = state.Read(cfg.StateDir)
		o.Method, o.Passes, o.Window, o.BPS = cfg.Method, cfg.Passes, cfg.Window, cfg.Bufhrt.BytesPerSecond
		writeJSON(w, o)
	})
	http.HandleFunc("/api/manifest", func(w http.ResponseWriter, r *http.Request) {
		cfg, _ := current()
		rel := filepath.Clean("/" + r.URL.Query().Get("dir"))
		dir := filepath.Join(cfg.Music, rel)
		if !strings.HasPrefix(dir, filepath.Clean(cfg.Music)) {
			http.Error(w, "bad dir", http.StatusBadRequest)
			return
		}
		m, err := manifest.Read(dir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, m)
	})

	// ── Setup ──
	http.HandleFunc("/api/setup", func(w http.ResponseWriter, r *http.Request) {
		cfg, ok := current()
		drives, derr := setup.Drives()
		host, _ := os.Hostname()
		resp := map[string]any{"config": cfg, "configured": ok, "presets": config.Presets,
			"drives": drives, "host": host, "overhead": setup.OverheadFactor}
		if derr != nil {
			resp["drives_error"] = derr.Error()
		}
		writeJSON(w, resp)
	})
	http.HandleFunc("/api/setup/dirs", func(w http.ResponseWriter, r *http.Request) {
		dirs, err := setup.Dirs(r.URL.Query().Get("path"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, dirs)
	})
	// candidate turns a submitted configuration into one that is safe to use:
	// program path and extra ssh options are never taken from the browser.
	candidate := func(r *http.Request) (config.Config, error) {
		cur, _ := current()
		c := cur
		if err := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(&c); err != nil {
			return c, err
		}
		c.Bufhrt.Program, c.Source.SSH, c.Source.Program = cur.Bufhrt.Program, cur.Source.SSH, cur.Source.Program
		if c.Preset != "" {
			if err := c.ApplyPreset(c.Preset); err != nil {
				return c, err
			}
		} else if c.Method == "" {
			c.Method = config.MethodName("eigen", c.Passes)
		}
		return c, nil
	}
	post := func(h func(http.ResponseWriter, config.Config)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "POST", http.StatusMethodNotAllowed)
				return
			}
			c, err := candidate(r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			h(w, c)
		}
	}
	http.HandleFunc("/api/setup/check", post(func(w http.ResponseWriter, c config.Config) {
		writeJSON(w, setup.Examine(context.Background(), c))
	}))
	var timingMu sync.Mutex
	http.HandleFunc("/api/setup/timing", post(func(w http.ResponseWriter, c config.Config) {
		if !timingMu.TryLock() {
			http.Error(w, "busy", http.StatusConflict)
			return
		}
		defer timingMu.Unlock()
		if st, err := state.Read(c.StateDir); err == nil && st.Phase == "running" && st.File != "" {
			http.Error(w, "night run active", http.StatusConflict)
			return
		}
		writeJSON(w, setup.MeasureTiming(context.Background(), c))
	}))
	http.HandleFunc("/api/setup/save", post(func(w http.ResponseWriter, c config.Config) {
		if err := config.Save(cfgPath, c); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := manifest.WriteStatus(c.Music, c.StatusURL, c.Method); err != nil {
			log.Print(err)
		}
		mu.Lock()
		cached.Scanned = time.Time{}
		mu.Unlock()
		writeJSON(w, map[string]any{"saved": true, "method": c.Method})
	}))

	log.Printf("amberIMPROVE %s — Statusseite auf %s", version, listen)
	log.Fatal(http.ListenAndServe(listen, nil))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
