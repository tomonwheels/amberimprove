package guard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tomonwheels/amberimprove/internal/config"
)

func TestInSpanAcrossMidnight(t *testing.T) {
	from, to := 23*60+30, 6*60+30
	cases := map[int]bool{23*60 + 29: false, 23*60 + 30: true, 0: true, 6*60 + 29: true, 6*60 + 30: false, 12 * 60: false}
	for m, want := range cases {
		if got := InSpan(m, from, to); got != want {
			t.Errorf("InSpan(%d) = %v, want %v", m, got, want)
		}
	}
}

func TestBlocked(t *testing.T) {
	w := config.Window{From: "23:30", To: "06:30", Blocked: []config.Span{{From: "02:15", To: "03:15"}}}
	at := func(h, m int) time.Time { return time.Date(2026, 9, 24, h, m, 0, 0, time.Local) }
	if !Blocked(w, at(2, 30)) || Blocked(w, at(3, 15)) || Blocked(w, at(1, 0)) {
		t.Fatal("blocked span wrong")
	}
}

func TestUntilEnd(t *testing.T) {
	w := config.Window{From: "23:30", To: "06:30"}
	at := func(h, m int) time.Time { return time.Date(2026, 9, 24, h, m, 0, 0, time.Local) }
	if d := UntilEnd(w, at(23, 30)); d != 7*time.Hour {
		t.Fatalf("23:30 → %v", d)
	}
	if d := UntilEnd(w, at(6, 0)); d != 30*time.Minute {
		t.Fatalf("06:00 → %v", d)
	}
	if d := UntilEnd(w, at(12, 0)); d != 0 {
		t.Fatalf("12:00 → %v", d)
	}
}

func TestPlaying(t *testing.T) {
	state := "idle"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"engine_state":"` + state + `","paused":true}`))
	}))
	defer srv.Close()
	p := config.Playback{URL: srv.URL, Field: "engine_state", Idle: []string{"idle", "stopped", ""}}
	if playing, _ := Playing(context.Background(), p); playing {
		t.Fatal("idle reported as playing")
	}
	state = "playing"
	if playing, why := Playing(context.Background(), p); !playing || why != ReasonPlaying {
		t.Fatal("playing not detected")
	}
	p.URL = "http://127.0.0.1:1/none"
	if playing, why := Playing(context.Background(), p); !playing || why != ReasonNoAnswer {
		t.Fatal("unreachable player must block by default")
	}
}
