package server_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AristarhUcolov/wheel-alignment/internal/live"
	"github.com/AristarhUcolov/wheel-alignment/internal/server"
)

func sendJSON(t *testing.T, srv http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decoding %s: %v", rec.Body.String(), err)
	}
	return v
}

// TestSessionForCatalogCar: choosing a car with no figures of its own — the
// ГАЗ-3110 — must bring its kingpin suspension and fall back to class guidance
// for tolerances, labelled as such.
func TestSessionForCatalogCar(t *testing.T) {
	srv := testServer(t)
	rec := sendJSON(t, srv, "/api/session", map[string]any{"spec_id": "gaz-3110"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	v := decode[server.SessionView](t, rec)
	if v.Vehicle == nil || v.Vehicle.ID != "gaz-3110" {
		t.Fatalf("vehicle not set: %+v", v.Vehicle)
	}
	if v.FrontSuspension.ID != "double_wishbone_kingpin" {
		t.Errorf("front suspension %q, want kingpin", v.FrontSuspension.ID)
	}
	if v.Limits == nil || v.Limits.SourceKind != "class_guidance" {
		t.Fatalf("limits should come from class guidance, got %+v", v.Limits)
	}
	if v.Vehicle.Disclaimer == "" {
		t.Error("a catalog car must carry its disclaimer")
	}
	if p := v.Params["camber_FL"]; p.Spec == nil {
		t.Error("camber tolerance missing from the session parameters")
	}

	// Switching to the ball-joint variant is the owner's call.
	rec = sendJSON(t, srv, "/api/session", map[string]any{"front_suspension": "double_wishbone_ball"})
	if v = decode[server.SessionView](t, rec); v.FrontSuspension.ID != "double_wishbone_ball" {
		t.Errorf("suspension override ignored: %q", v.FrontSuspension.ID)
	}
	// A rear-only design on the front axle is refused.
	if rec = sendJSON(t, srv, "/api/session", map[string]any{"front_suspension": "twist_beam"}); rec.Code != http.StatusBadRequest {
		t.Errorf("twist beam accepted as a front suspension: %d", rec.Code)
	}
}

// TestManualLiveAndSnapshot: typed-in string and gauge readings go straight to
// the screen, and a complete set can be frozen for the report.
func TestManualLiveAndSnapshot(t *testing.T) {
	srv := testServer(t)
	sendJSON(t, srv, "/api/session", map[string]any{"spec_id": "vaz-2101-2107-classic"})

	// Snapshot refused before all four wheels are measured.
	if rec := sendJSON(t, srv, "/api/live/snapshot", map[string]string{"label": "before"}); rec.Code != http.StatusConflict {
		t.Errorf("snapshot of an unmeasured car: status %d", rec.Code)
	}

	for _, w := range []string{"FL", "FR", "RL", "RR"} {
		rec := sendJSON(t, srv, "/api/live/manual", map[string]any{
			"wheel": w, "camber": 0.4, "camber_180": 0.5,
			"toe_front_mm": 50.8, "toe_rear_mm": 50.0,
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", w, rec.Code, rec.Body.String())
		}
	}
	fr := decode[live.Frame](t, sendJSON(t, srv, "/api/live/manual", map[string]any{
		"wheel": "FL", "camber": 0.4, "camber_180": 0.5, "toe_front_mm": 50.8, "toe_rear_mm": 50.0,
		"sweep": map[string]float64{"camber_out": -0.9, "camber_in": 1.8, "half_sweep_deg": 20},
	}))
	p := fr.Params["camber_FL"]
	if !p.Has || p.Value != 0.45 {
		t.Errorf("camber with runout compensation: %.4f, want 0.45 (mean of 0.4 and 0.5)", p.Value)
	}
	if !fr.Params["caster_FL"].Has {
		t.Error("caster sweep did not produce caster")
	}
	if !fr.Params["toe_FL"].Has || fr.Params["toe_FL"].Value <= 0 {
		t.Errorf("toe-in from the string line not shown: %+v", fr.Params["toe_FL"])
	}

	rec := sendJSON(t, srv, "/api/live/snapshot", map[string]string{"label": "before"})
	if rec.Code != http.StatusOK {
		t.Fatalf("snapshot: %d %s", rec.Code, rec.Body.String())
	}
	snap := decode[server.Snapshot](t, rec)
	if !snap.Report.HasSpec || len(snap.Report.Params) == 0 {
		t.Error("snapshot report is empty")
	}
	rv := decode[server.ReportView](t, get(t, srv, "/api/report"))
	if rv.Before == nil || rv.After != nil {
		t.Error("report does not carry exactly the before snapshot")
	}
}

// TestSensorProtocol: the open protocol for DIY sensor heads — single objects
// and batches, with bad input refused rather than displayed.
func TestSensorProtocol(t *testing.T) {
	srv := testServer(t)
	rec := sendJSON(t, srv, "/api/live/sample", []map[string]any{
		{"source": "esp32-a", "wheel": "FL", "camber": -0.3},
		{"source": "esp32-b", "wheel": "FR", "camber": -0.1, "toe": 0.05},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("batch refused: %d %s", rec.Code, rec.Body.String())
	}
	fr := decode[live.Frame](t, get(t, srv, "/api/live/frame"))
	if !fr.Params["camber_FL"].Has || !fr.Params["toe_FR"].Has {
		t.Error("sensor readings not on screen")
	}
	if len(fr.Sources) < 2 {
		t.Errorf("sensors not listed as sources: %+v", fr.Sources)
	}
	for _, bad := range []map[string]any{
		{"wheel": "FL", "camber": 1},                  // no source
		{"source": "x", "wheel": "XX", "camber": 1},   // no such wheel
		{"source": "x", "wheel": "FL", "camber": 60},  // implausible
		{"source": "x", "wheel": "RL", "caster": 3.0}, // caster on a rear wheel
	} {
		if rec := sendJSON(t, srv, "/api/live/sample", bad); rec.Code != http.StatusBadRequest {
			t.Errorf("%v accepted with status %d", bad, rec.Code)
		}
	}
}

// TestSimulatorAndStream: the demonstration car streams frames over SSE.
func TestSimulatorAndStream(t *testing.T) {
	srv := testServer(t)
	sendJSON(t, srv, "/api/session", map[string]any{"spec_id": "guidance-fwd-mcpherson"})
	if rec := sendJSON(t, srv, "/api/sim", map[string]string{"action": "start"}); rec.Code != http.StatusOK {
		t.Fatalf("sim start: %d", rec.Code)
	}
	if rec := sendJSON(t, srv, "/api/sim", map[string]any{"action": "adjust", "key": "toe_FL", "delta": 0.1}); rec.Code != http.StatusOK {
		t.Errorf("sim adjust: %d %s", rec.Code, rec.Body.String())
	}
	if rec := sendJSON(t, srv, "/api/sim", map[string]any{"action": "adjust", "key": "toe_FL", "delta": 30}); rec.Code != http.StatusBadRequest {
		t.Errorf("a 30° adjuster step accepted: %d", rec.Code)
	}

	hs := httptest.NewServer(srv)
	defer hs.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, hs.URL+"/api/live/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content type %q", ct)
	}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var fr live.Frame
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &fr); err != nil {
			t.Fatal(err)
		}
		if fr.Params["camber_RR"].Has && fr.Params["caster_FL"].Has {
			sendJSON(t, srv, "/api/sim", map[string]string{"action": "stop"})
			return
		}
	}
	t.Fatal("stream ended without a frame from the simulator")
}

func get(t *testing.T, srv http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}
