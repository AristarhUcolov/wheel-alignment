package phone

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
	"github.com/AristarhUcolov/wheel-alignment/internal/live"
)

// loopback stands in for the phones' port in tests: listening on every
// network would bring up the firewall's question on a developer's screen.
func loopback(network, addr string) (net.Listener, error) { return net.Listen("tcp", "127.0.0.1:0") }

func testLink(t *testing.T, dir string) *Link {
	t.Helper()
	l := NewLink(live.NewHub(), dir)
	l.listen = loopback
	t.Cleanup(l.Close)
	return l
}

// TestAccessSurvivesRestart: access switched on keeps its key and port, so
// the address in the code stays the same and a phone left with the page open
// reconnects when the program starts again. Switched off, it stays off.
func TestAccessSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	l := testLink(t, dir)
	if err := l.Enable(); err != nil {
		t.Fatal(err)
	}
	key, port := l.token, l.port
	if key == "" || port == 0 {
		t.Fatalf("enabled without a key or port: %q %d", key, port)
	}
	l.Close() // the program ends; access stays marked as on

	l2 := testLink(t, dir)
	if err := l2.Restore(); err != nil {
		t.Fatal(err)
	}
	if !l2.Enabled() || l2.token != key || l2.port != port {
		t.Fatalf("after a restart: enabled %v, key %q, port %d; want key %q, port %d", l2.Enabled(), l2.token, l2.port, key, port)
	}

	l2.Disable()
	l3 := testLink(t, dir)
	if err := l3.Restore(); err != nil {
		t.Fatal(err)
	}
	if l3.Enabled() {
		t.Error("access switched off came back on by itself")
	}
}

// TestNewKeyShutsOutOldAddress: after "new key" the old address answers
// nothing, the new one works, and phones disconnected under the old key may
// connect again.
func TestNewKeyShutsOutOldAddress(t *testing.T) {
	l := testLink(t, t.TempDir())
	if err := l.Enable(); err != nil {
		t.Fatal(err)
	}
	old := l.token
	if _, err := l.handle("phoneaaa", phoneRequest{Wheel: "FL"}); err != nil {
		t.Fatal(err)
	}
	l.Forget("phoneaaa")
	l.NewKey()
	if l.token == old || l.token == "" {
		t.Fatalf("no new key: %q", l.token)
	}
	if inf := l.Info(); len(inf.Blocked) != 0 {
		t.Errorf("a new key keeps phones blocked: %+v", inf.Blocked)
	}
	post := func(key string) int {
		rec := httptest.NewRecorder()
		l.Handler().ServeHTTP(rec, httptest.NewRequest("POST", "/p/"+key+"/data", strings.NewReader(`{"d":"phonebbb","wheel":"FR"}`)))
		return rec.Code
	}
	if c := post(old); c != http.StatusNotFound {
		t.Errorf("old key: status %d, want 404", c)
	}
	if c := post(l.token); c != http.StatusOK {
		t.Errorf("new key: status %d, want 200", c)
	}
}

// TestWheelResentKeepsRunout: the page sends its wheel with every request, so
// that it carries on after the computer restarts. The same wheel again must
// not throw away the runout measured on it; another wheel must.
func TestWheelResentKeepsRunout(t *testing.T) {
	r := newRig(t)
	r.cmd(phoneRequest{Wheel: "FL"})
	d := r.l.devices[r.id]
	d.hasRunout, d.runout = true, 0.2
	r.cmd(phoneRequest{Wheel: "FL"})
	if !d.hasRunout {
		t.Error("the same wheel sent again dropped its runout")
	}
	r.cmd(phoneRequest{Wheel: "FR"})
	if d.hasRunout {
		t.Error("moving to another wheel kept the old wheel's runout")
	}
}

// TestNewCarKeepsCalibration: the next car on the stand has other wheels, so
// runout and caster go; the phone's own calibration stays.
func TestNewCarKeepsCalibration(t *testing.T) {
	r := newRig(t)
	p := typicalPhone(5, 1)
	r.cmd(phoneRequest{Wheel: "FL"})
	r.calibrate(p, spinAxis(align.FL, 0.3, 0))
	d := r.l.devices[r.id]
	c := 4.5
	d.hasRunout, d.runout, d.Caster = true, 0.2, &c
	r.l.NewCar()
	if d.hasRunout || d.Caster != nil {
		t.Error("runout or caster of the previous car survived")
	}
	if !d.Cal.Ready() {
		t.Error("a new car threw away the phone's calibration")
	}
	if !d.HasWheel || d.Wheel != align.FL {
		t.Error("a new car moved the phone off its wheel")
	}
}
