// Package schedule evaluates ScalePlan windows against wall clock time.
//
// Windows are expressed as cron fires plus a duration, evaluated in the IANA
// zone of the window. Time zones matter: an office hours window must keep
// firing at local 08:00 across a DST change, and a fire that falls into the
// spring forward gap is skipped rather than shifted.
package schedule

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

// Window is the evaluator view of one spec.windows entry.
type Window struct {
	Name     string
	Schedule string
	TimeZone string
	Duration time.Duration
	Replicas int32
}

// compiled is a parsed window ready for repeated evaluation.
type compiled struct {
	Window
	spec *cron.SpecSchedule
	loc  *time.Location
}

// Plan is a compiled, immutable window set.
type Plan struct {
	windows []compiled
}

// Compile parses schedules and resolves time zones once per reconcile. An
// invalid window fails the whole plan so the CRD condition names it.
func Compile(windows []Window) (*Plan, error) {
	p := &Plan{windows: make([]compiled, 0, len(windows))}
	for _, w := range windows {
		loc, err := time.LoadLocation(orUTC(w.TimeZone))
		if err != nil {
			return nil, fmt.Errorf("window %q: loading time zone: %w", w.Name, err)
		}
		parsed, err := cron.ParseStandard(w.Schedule)
		if err != nil {
			return nil, fmt.Errorf("window %q: parsing schedule: %w", w.Name, err)
		}
		spec, ok := parsed.(*cron.SpecSchedule)
		if !ok {
			return nil, fmt.Errorf("window %q: unsupported schedule type %T", w.Name, parsed)
		}
		p.windows = append(p.windows, compiled{Window: w, spec: spec, loc: loc})
	}
	return p, nil
}

// Desired returns the last open window and its replicas. Windows are ordered
// by spec position, so a later window overrides an earlier one.
func (p *Plan) Desired(now time.Time) (string, int32) {
	name, replicas := "", int32(0)
	for i := range p.windows {
		if open(&p.windows[i], now) {
			name, replicas = p.windows[i].Name, p.windows[i].Replicas
		}
	}
	return name, replicas
}

// NextBoundary returns the nearest upcoming fire across all windows, capped at
// max so the caller has a fallback poll. It returns max when nothing is sooner.
func (p *Plan) NextBoundary(now time.Time, max time.Duration) time.Duration {
	next := max
	for i := range p.windows {
		w := &p.windows[i]
		fire := w.spec.Next(now.In(w.loc))
		if d := fire.Sub(now); d > 0 && d < next {
			next = d
		}
	}
	return next
}

// open reports whether now falls inside the window opened by the most recent
// fire. Lookback is bounded to seven days, which is also the backfill limit
// documented in the README.
func open(w *compiled, now time.Time) bool {
	local := now.In(w.loc)
	var last time.Time
	for fire := w.spec.Next(local.Add(-7 * 24 * time.Hour)); fire.Before(local); fire = w.spec.Next(fire) {
		last = fire
	}
	if last.IsZero() {
		return false
	}
	return local.Before(last.Add(w.Duration))
}

func orUTC(zone string) string {
	if zone == "" {
		return "UTC"
	}
	return zone
}
