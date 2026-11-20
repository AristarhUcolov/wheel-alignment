package server_test

import (
	"net/http"
	"testing"

	"github.com/AristarhUcolov/wheel-alignment/internal/live"
	"github.com/AristarhUcolov/wheel-alignment/internal/server"
)

func fullManual(t *testing.T, srv http.Handler) {
	t.Helper()
	for _, w := range []string{"FL", "FR", "RL", "RR"} {
		req := map[string]any{"wheel": w, "replace": true, "camber": -0.3, "camber_180": -0.4,
			"toe_front_mm": 50.6, "toe_rear_mm": 50.0}
		if w == "FL" {
			req["sweep_out"], req["sweep_in"], req["half_sweep"] = -1.2, 1.6, 20.0
		}
		if rec := sendJSON(t, srv, "/api/live/manual", req); rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", w, rec.Code, rec.Body.String())
		}
	}
}

// TestJobSurvivesRestart: everything the person entered for the car on the
// stand — the car, its rim and tracks, the typed-in readings, the "before"
// snapshot, the checklist and the plate number — is there again after the
// program is closed and opened.
func TestJobSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	srv := testServer(t)
	srv.SetDataDir(dir)
	sendJSON(t, srv, "/api/session", map[string]any{"spec_id": "vaz-2101-2107-classic"})
	sendJSON(t, srv, "/api/session", map[string]any{"rim_diameter_in": 14, "track_front_mm": 1365, "track_rear_mm": 1321})
	fullManual(t, srv)
	if rec := sendJSON(t, srv, "/api/live/snapshot", map[string]string{"label": "before"}); rec.Code != http.StatusOK {
		t.Fatalf("snapshot: %d %s", rec.Code, rec.Body.String())
	}
	rec := sendJSON(t, srv, "/api/job", map[string]any{
		"checks": map[string]bool{"vaz|safety|s1": true},
		"report": map[string]string{"plate": "А123ВС 77", "odo": "123456"},
		"box":    map[string]string{"lf": "120", "lr": "118"},
		"notes":  map[string]string{"toot_l": "22.5", "road_pull": "none"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("job: %d %s", rec.Code, rec.Body.String())
	}

	again := testServer(t)
	again.SetDataDir(dir)
	v := decode[server.SessionView](t, get(t, again, "/api/session"))
	if v.Vehicle == nil || v.Vehicle.ID != "vaz-2101-2107-classic" {
		t.Fatalf("the car was lost: %+v", v.Vehicle)
	}
	if v.RimDiameterIn != 14 || v.TrackFrontMM != 1365 || v.TrackRearMM != 1321 {
		t.Errorf("rim and tracks lost: %v %v %v", v.RimDiameterIn, v.TrackFrontMM, v.TrackRearMM)
	}
	if !v.HasBefore || v.HasAfter {
		t.Errorf("snapshots after a restart: before %v, after %v", v.HasBefore, v.HasAfter)
	}
	if !v.Checks["vaz|safety|s1"] || v.ReportMeta.Plate != "А123ВС 77" || v.ReportMeta.Odo != "123456" || v.Box["lf"] != "120" {
		t.Errorf("checklist, report header or string box lost: %v %+v %v", v.Checks, v.ReportMeta, v.Box)
	}
	if v.Notes["toot_l"] != "22.5" || v.Notes["road_pull"] != "none" {
		t.Errorf("notes lost: %v", v.Notes)
	}
	if m := v.Manual["FL"]; m.Camber == nil || *m.Camber != -0.3 || m.SweepOut == nil || *m.SweepOut != -1.2 {
		t.Errorf("the typed-in form of FL was lost: %+v", m)
	}

	fr := decode[live.Frame](t, get(t, again, "/api/live/frame"))
	for _, k := range []string{"camber_FL", "toe_FL", "caster_FL", "camber_RR", "toe_RR", "thrust_angle"} {
		if !fr.Params[k].Has {
			t.Errorf("%s not back on the live screen after a restart", k)
		}
	}
	if p := fr.Params["camber_FL"]; p.Value != -0.35 {
		t.Errorf("camber_FL %.4f after a restart, want −0.35 (runout-compensated)", p.Value)
	}

	rv := decode[server.ReportView](t, get(t, again, "/api/report"))
	if rv.Before == nil || !rv.Before.Report.HasSpec || len(rv.Before.Report.Params) == 0 {
		t.Error("the report of the kept snapshot is empty")
	}
}

// TestManualFormClearsEmptiedField: the form is the source of truth for the
// numbers typed in. Emptying a field takes that number off the screen — and
// only that number: a sensor's reading on the same wheel is not the form's.
func TestManualFormClearsEmptiedField(t *testing.T) {
	srv := testServer(t)
	send := func(body map[string]any) live.Frame {
		t.Helper()
		rec := sendJSON(t, srv, "/api/live/manual", body)
		if rec.Code != http.StatusOK {
			t.Fatalf("%v: %d %s", body, rec.Code, rec.Body.String())
		}
		return decode[live.Frame](t, rec)
	}
	fr := send(map[string]any{"wheel": "FL", "replace": true, "camber": 0.5, "toe_front_mm": 50.5, "toe_rear_mm": 50, "sweep_out": -1, "sweep_in": 1.5})
	if !fr.Params["camber_FL"].Has || !fr.Params["toe_FL"].Has || !fr.Params["caster_FL"].Has {
		t.Fatalf("form not on screen: %+v", fr.Params["camber_FL"])
	}
	fr = send(map[string]any{"wheel": "FL", "replace": true, "toe_front_mm": 50.5, "toe_rear_mm": 50, "sweep_out": -1})
	if fr.Params["camber_FL"].Has || fr.Params["caster_FL"].Has {
		t.Error("emptied camber or half-emptied caster sweep still on screen")
	}
	if !fr.Params["toe_FL"].Has {
		t.Error("the toe still in the form left the screen")
	}

	sendJSON(t, srv, "/api/live/sample", map[string]any{"source": "esp", "wheel": "FR", "camber": -0.2})
	fr = send(map[string]any{"wheel": "FR", "replace": true})
	if !fr.Params["camber_FR"].Has {
		t.Error("an empty form wiped the sensor's camber on FR")
	}

	if rec := sendJSON(t, srv, "/api/live/manual", map[string]any{"wheel": "RL", "replace": true, "sweep_out": 1, "sweep_in": -1}); rec.Code != http.StatusBadRequest {
		t.Errorf("a caster sweep on a rear wheel was accepted: %d", rec.Code)
	}
}

// TestNewJobStartsClean: "new car" leaves nothing of the previous one — no
// readings, no snapshots, no checklist, no plate number — and stays clean
// after a restart.
func TestNewJobStartsClean(t *testing.T) {
	dir := t.TempDir()
	srv := testServer(t)
	srv.SetDataDir(dir)
	sendJSON(t, srv, "/api/session", map[string]any{"spec_id": "gaz-3110"})
	fullManual(t, srv)
	sendJSON(t, srv, "/api/live/snapshot", map[string]string{"label": "before"})
	sendJSON(t, srv, "/api/job", map[string]any{"report": map[string]string{"plate": "X"}, "checks": map[string]bool{"k": true}, "notes": map[string]string{"road_pull": "left"}})

	rec := sendJSON(t, srv, "/api/job/new", map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("new job: %d %s", rec.Code, rec.Body.String())
	}
	check := func(h http.Handler, when string) {
		t.Helper()
		v := decode[server.SessionView](t, get(t, h, "/api/session"))
		if v.Vehicle != nil || v.HasBefore || len(v.Manual) != 0 || len(v.Checks) != 0 || v.ReportMeta.Plate != "" || len(v.Notes) != 0 {
			t.Errorf("%s: the previous car is still there: vehicle %v, before %v, manual %v, checks %v, report %+v",
				when, v.Vehicle, v.HasBefore, v.Manual, v.Checks, v.ReportMeta)
		}
		if v.RimDiameterIn != 15 {
			t.Errorf("%s: rim %v, want the neutral 15″", when, v.RimDiameterIn)
		}
		if fr := decode[live.Frame](t, get(t, h, "/api/live/frame")); fr.Measured != 0 {
			t.Errorf("%s: %d readings of the previous car on screen", when, fr.Measured)
		}
	}
	check(srv, "right away")
	again := testServer(t)
	again.SetDataDir(dir)
	check(again, "after a restart")
}

// TestJobRefusesOversizedInput: the job file is the interface's, not a dump
// for anything a page sends.
func TestJobRefusesOversizedInput(t *testing.T) {
	srv := testServer(t)
	long := make([]byte, 500)
	for i := range long {
		long[i] = 'a'
	}
	if rec := sendJSON(t, srv, "/api/job", map[string]any{"report": map[string]string{"plate": string(long)}}); rec.Code != http.StatusBadRequest {
		t.Errorf("a 500-character plate number accepted: %d", rec.Code)
	}
	if rec := sendJSON(t, srv, "/api/session", map[string]any{"track_front_mm": -5}); rec.Code != http.StatusBadRequest {
		t.Errorf("a negative track accepted: %d", rec.Code)
	}
}

// TestRimChangeRecomputesStringToe: string-line millimetres are an angle only
// for a given rim, so a new rim diameter must change the toe on screen.
func TestRimChangeRecomputesStringToe(t *testing.T) {
	srv := testServer(t)
	sendJSON(t, srv, "/api/session", map[string]any{"rim_diameter_in": 13})
	sendJSON(t, srv, "/api/live/manual", map[string]any{"wheel": "FL", "replace": true, "toe_front_mm": 51, "toe_rear_mm": 50})
	small := decode[live.Frame](t, get(t, srv, "/api/live/frame")).Params["toe_FL"].Value
	sendJSON(t, srv, "/api/session", map[string]any{"rim_diameter_in": 17})
	big := decode[live.Frame](t, get(t, srv, "/api/live/frame")).Params["toe_FL"].Value
	if !(big < small) || big <= 0 {
		t.Errorf("1 mm of toe: %.4f° on 13″, %.4f° on 17″ — the larger rim must give the smaller angle", small, big)
	}
}

// TestBadFieldDoesNotBlockTheRest: a mistyped camber is refused and leaves
// the screen, but the toe typed on the same wheel still shows.
func TestBadFieldDoesNotBlockTheRest(t *testing.T) {
	srv := testServer(t)
	sendJSON(t, srv, "/api/live/manual", map[string]any{"wheel": "FR", "replace": true, "camber": 0.3})
	rec := sendJSON(t, srv, "/api/live/manual", map[string]any{"wheel": "FR", "replace": true, "camber": 45, "toe_front_mm": 50.5, "toe_rear_mm": 50})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("an implausible 45° camber accepted: %d", rec.Code)
	}
	fr := decode[live.Frame](t, get(t, srv, "/api/live/frame"))
	if fr.Params["camber_FR"].Has {
		t.Error("the old camber stayed on screen although the form now holds another number")
	}
	if !fr.Params["toe_FR"].Has {
		t.Error("a bad camber kept the toe of the same wheel off the screen")
	}
}
