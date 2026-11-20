package schedule

import (
	"testing"
	"time"
)

func mustCompile(t *testing.T, windows ...Window) *Plan {
	t.Helper()
	p, err := Compile(windows)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	return p
}

func mustParse(t *testing.T, layout, value string) time.Time {
	t.Helper()
	ts, err := time.Parse(layout, value)
	if err != nil {
		t.Fatalf("parsing %q: %v", value, err)
	}
	return ts
}

func TestDesiredPicksLastOpenWindow(t *testing.T) {
	p := mustCompile(t,
		Window{Name: "early", Schedule: "0 5 * * *", Duration: 4 * time.Hour, Replicas: 4},
		Window{Name: "late", Schedule: "0 6 * * *", Duration: time.Hour, Replicas: 9},
	)
	now := mustParse(t, time.RFC3339, "2026-05-04T06:30:00Z")

	name, replicas := p.Desired(now)
	if name != "late" || replicas != 9 {
		t.Fatalf("Desired = %q/%d, want late/9", name, replicas)
	}
}

func TestDesiredClosesWindow(t *testing.T) {
	p := mustCompile(t, Window{Name: "early", Schedule: "0 5 * * *", Duration: 2 * time.Hour, Replicas: 4})
	now := mustParse(t, time.RFC3339, "2026-05-04T07:00:00Z")

	if name, _ := p.Desired(now); name != "" {
		t.Fatalf("Desired = %q, want no open window", name)
	}
}

func TestDesiredUsesWindowZone(t *testing.T) {
	// 08:00 Prague is 06:00 UTC in summer; evaluating in UTC would open at 08:00Z.
	p := mustCompile(t, Window{Name: "office", Schedule: "0 8 * * *", TimeZone: "Europe/Prague", Duration: time.Hour, Replicas: 3})

	if name, _ := p.Desired(mustParse(t, time.RFC3339, "2026-06-01T06:15:00Z")); name != "office" {
		t.Fatalf("window not open at local 08:15, got %q", name)
	}
	if name, _ := p.Desired(mustParse(t, time.RFC3339, "2026-06-01T08:15:00Z")); name != "" {
		t.Fatalf("window still open at 10:15 local, got %q", name)
	}
}

func TestSpringForwardGapIsSkipped(t *testing.T) {
	// Prague jumps 02:00 to 03:00 on 2026-03-29, so a 02:30 fire does not exist.
	p := mustCompile(t, Window{Name: "gap", Schedule: "30 2 * * *", TimeZone: "Europe/Prague", Duration: time.Hour, Replicas: 5})

	if name, _ := p.Desired(mustParse(t, time.RFC3339, "2026-03-29T05:00:00Z")); name != "" {
		t.Fatalf("gap fire opened a window, got %q", name)
	}
	next := p.NextBoundary(mustParse(t, time.RFC3339, "2026-03-29T05:00:00Z"), 48*time.Hour)
	want := 19*time.Hour + 30*time.Minute // 2026-03-30 02:30 CEST is 00:30Z
	if next != want {
		t.Fatalf("NextBoundary = %s, want %s", next, want)
	}
}

func TestNextBoundaryCapsAtMax(t *testing.T) {
	p := mustCompile(t, Window{Name: "weekly", Schedule: "0 3 * * 1", Duration: time.Hour, Replicas: 2})
	now := mustParse(t, time.RFC3339, "2026-05-06T00:00:00Z")

	if got := p.NextBoundary(now, 10*time.Minute); got != 10*time.Minute {
		t.Fatalf("NextBoundary = %s, want the 10m cap", got)
	}
}

func TestCompileRejectsBadWindow(t *testing.T) {
	if _, err := Compile([]Window{{Name: "bad", Schedule: "not cron"}}); err == nil {
		t.Fatal("expected an error for a malformed schedule")
	}
	if _, err := Compile([]Window{{Name: "zone", Schedule: "0 5 * * *", TimeZone: "Mars/Olympus"}}); err == nil {
		t.Fatal("expected an error for an unknown time zone")
	}
}
