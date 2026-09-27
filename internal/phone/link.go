package phone

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"embed"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
	"github.com/AristarhUcolov/wheel-alignment/internal/i18n"
	"github.com/AristarhUcolov/wheel-alignment/internal/live"
	"github.com/AristarhUcolov/wheel-alignment/internal/specs"
)

//go:embed web/phone.html
var pageFS embed.FS

// Link serves the phone page on the local network and turns what phones send
// into live readings.
//
// It is off until switched on from the interface, and every address it serves
// carries a random key that changes each time it is switched on: a phone that
// has not scanned the code on this computer's screen cannot send anything.
type Link struct {
	hub  *live.Hub
	dir  string
	now  func() time.Time
	sens http.Handler

	mu      sync.Mutex
	enabled bool
	token   string
	srv     *http.Server
	port    int
	ips     []net.IP
	devices map[string]*Device
	cals    map[string]Calibration

	// blocked holds phones disconnected from the desktop, by id, with the name
	// they had. A disconnected phone that keeps sending is refused, so a
	// wrongly connected phone cannot come straight back; it is let in again
	// when the person allows it or switches access off and on.
	blocked map[string]string
}

// NewLink returns a switched-off link that feeds hub and keeps phone
// calibrations in dir.
func NewLink(hub *live.Hub, dir string) *Link {
	l := &Link{hub: hub, dir: dir, now: time.Now, devices: map[string]*Device{}, cals: map[string]Calibration{},
		blocked: map[string]string{}}
	l.loadCals()
	return l
}

// SetSensorHandler lets DIY sensor heads on the network use the same address
// as the phones, with the open sensor protocol.
func (l *Link) SetSensorHandler(h http.Handler) { l.sens = h }

// Enabled reports whether phones can connect.
func (l *Link) Enabled() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.enabled
}

// Enable starts the HTTPS listener on the local network.
func (l *Link) Enable() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.enabled {
		return nil
	}
	ips := lanIPs()
	cert, err := certificate(filepath.Join(l.dir, "tls"), ips)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("не удалось создать сертификат"), err)
	}
	var ln net.Listener
	for port := 8701; port <= 8720; port++ {
		ln, err = net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			l.port = port
			break
		}
	}
	if ln == nil {
		return fmt.Errorf("%s: %w", i18n.T("не удалось открыть порт для телефонов"), err)
	}
	l.token = newToken()
	l.ips = ips
	// A new key is a fresh start: phones disconnected last time may join again.
	l.blocked = map[string]string{}
	l.srv = &http.Server{
		Handler:           l.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		TLSConfig:         &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12},
		ErrorLog:          nil,
	}
	srv := l.srv
	go func() { _ = srv.ServeTLS(ln, "", "") }()
	l.enabled = true
	return nil
}

// Disable stops accepting phones. Readings already on the live screen stay
// there and grow stale.
func (l *Link) Disable() {
	l.mu.Lock()
	srv := l.srv
	l.enabled, l.srv, l.token = false, nil, ""
	l.mu.Unlock()
	if srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}
}

// Close is Disable, for deferred shutdown.
func (l *Link) Close() { l.Disable() }

// ErrForgotten is what a disconnected phone is told.
var ErrForgotten = i18n.Err("Этот телефон отключён на компьютере. Чтобы подключить его снова, разрешите его в списке телефонов или отсканируйте новый код.")

// Forget disconnects one phone: it leaves the list, its readings leave the
// live screen, and it is refused until allowed back. Its calibration is kept —
// it belongs to the phone, not to this session. Reports whether the phone was
// known.
func (l *Link) Forget(id string) bool {
	l.mu.Lock()
	d, ok := l.devices[id]
	if ok {
		delete(l.devices, id)
		l.blocked[id] = d.Name
	}
	l.mu.Unlock()
	if ok {
		src := "phone:" + id
		l.hub.Clear(src)
		l.hub.RemoveSource(src)
	}
	return ok
}

// Allow lets a disconnected phone connect again.
func (l *Link) Allow(id string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, ok := l.blocked[id]
	delete(l.blocked, id)
	return ok
}

func newToken() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b))
}

// ── For the desktop interface ────────────────────────────────────────

// DeviceInfo is one phone as the desktop shows it.
type DeviceInfo struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Wheel      string   `json:"wheel,omitempty"`
	Calibrated bool     `json:"calibrated"`
	Camber     *float64 `json:"camber"`
	Prompt     string   `json:"prompt"`
	Online     bool     `json:"online"`
	AgeS       float64  `json:"age_s"`
}

// Info is the link's state for the desktop interface.
type Info struct {
	Enabled bool         `json:"enabled"`
	URLs    []string     `json:"urls"`
	QRSVG   string       `json:"qr_svg,omitempty"`
	Devices []DeviceInfo `json:"devices"`
	// Blocked lists phones disconnected from the desktop, so they can be let
	// back in.
	Blocked []DeviceInfo `json:"blocked"`
}

// Info describes the link for the desktop interface.
func (l *Link) Info() Info {
	l.mu.Lock()
	defer l.mu.Unlock()
	inf := Info{Enabled: l.enabled, Devices: []DeviceInfo{}, Blocked: []DeviceInfo{}}
	if l.enabled {
		for _, ip := range l.ips {
			inf.URLs = append(inf.URLs, fmt.Sprintf("https://%s:%d/p/%s", ip, l.port, l.token))
		}
		if len(inf.URLs) > 0 {
			inf.QRSVG, _ = qrSVG(inf.URLs[0])
		}
	}
	now := l.now()
	for _, d := range l.devices {
		di := DeviceInfo{
			ID: d.ID, Name: d.Name, Calibrated: d.Cal.Ready(), Prompt: d.Prompt,
			Online: now.Sub(d.LastSeen) < 3*time.Second, AgeS: now.Sub(d.LastSeen).Seconds(),
		}
		if d.HasWheel {
			di.Wheel = d.Wheel.String()
		}
		if d.camberOK {
			c := round2(d.camber)
			di.Camber = &c
		}
		inf.Devices = append(inf.Devices, di)
	}
	sort.Slice(inf.Devices, func(i, j int) bool { return inf.Devices[i].ID < inf.Devices[j].ID })
	for id, name := range l.blocked {
		inf.Blocked = append(inf.Blocked, DeviceInfo{ID: id, Name: name})
	}
	sort.Slice(inf.Blocked, func(i, j int) bool { return inf.Blocked[i].ID < inf.Blocked[j].ID })
	return inf
}

// ── For the phone ────────────────────────────────────────────────────

// PhoneState is everything the phone page shows.
type PhoneState struct {
	Wheel     string       `json:"wheel,omitempty"`
	WheelName string       `json:"wheel_name,omitempty"`
	Front     bool         `json:"front"`
	Step      Step         `json:"step"`
	Prompt    string       `json:"prompt"`
	Error     string       `json:"error,omitempty"`
	Progress  float64      `json:"progress"`
	Flat      bool         `json:"flat"`
	Mounted   bool         `json:"mounted"`
	Camber    *float64     `json:"camber"`
	Stable    bool         `json:"stable"`
	Status    align.Status `json:"status,omitempty"`
	Spec      *align.Range `json:"spec,omitempty"`
	Steer     *float64     `json:"steer,omitempty"`
	Caster    *float64     `json:"caster,omitempty"`
	Runout    *float64     `json:"runout,omitempty"`
	Gyro      bool         `json:"gyro"`
	// Lang tells the page which language the program speaks, for its own
	// buttons; the prompts above already come translated.
	Lang string `json:"lang"`
}

type phoneRequest struct {
	Device  string   `json:"d"`
	Name    string   `json:"name"`
	Cmd     string   `json:"cmd"`
	Wheel   string   `json:"wheel"`
	Samples []Sample `json:"samples"`
}

// Handler serves the phone side. It is exported for tests; in use it runs
// behind the HTTPS listener started by Enable.
func (l *Link) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /p/{token}", l.page)
	mux.HandleFunc("POST /p/{token}/data", l.data)
	mux.HandleFunc("POST /p/{token}/sample", func(w http.ResponseWriter, r *http.Request) {
		if !l.authorised(r) || l.sens == nil {
			http.NotFound(w, r)
			return
		}
		l.sens.ServeHTTP(w, r)
	})
	return mux
}

func (l *Link) authorised(r *http.Request) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.token != "" && r.PathValue("token") == l.token
}

// SetToken fixes the key, for tests that use Handler without Enable.
func (l *Link) SetToken(t string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.token = t
}

func (l *Link) page(w http.ResponseWriter, r *http.Request) {
	if !l.authorised(r) {
		http.Error(w, i18n.T("Ссылка устарела: отсканируйте код на экране компьютера заново."), http.StatusNotFound)
		return
	}
	b, _ := pageFS.ReadFile("web/phone.html")
	// Страница переводит свои надписи сама, но язык должна знать до первого
	// ответа, иначе на миг покажется русский текст.
	b = bytes.Replace(b, []byte(`<html lang="ru">`), []byte(`<html lang="`+string(i18n.Current())+`">`), 1)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(b)
}

func (l *Link) data(w http.ResponseWriter, r *http.Request) {
	if !l.authorised(r) {
		http.Error(w, i18n.T("ключ устарел"), http.StatusNotFound)
		return
	}
	var req phoneRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<18)).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	id := sanitizeID(req.Device)
	if id == "" {
		http.Error(w, i18n.T("нет идентификатора телефона"), http.StatusBadRequest)
		return
	}
	st, err := l.handle(id, req)
	if errors.Is(err, ErrForgotten) {
		// 410: the page stops sending instead of retrying ten times a second.
		http.Error(w, err.Error(), http.StatusGone)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err != nil {
		st.Error = err.Error()
	}
	_ = json.NewEncoder(w).Encode(st)
}

// handle applies one request from a phone and returns what it should show.
func (l *Link) handle(id string, req phoneRequest) (PhoneState, error) {
	l.mu.Lock()
	if _, no := l.blocked[id]; no {
		l.mu.Unlock()
		return PhoneState{Lang: string(i18n.Current())}, ErrForgotten
	}
	d, ok := l.devices[id]
	if !ok {
		d = newDevice(id, l.cals[id])
		l.devices[id] = d
	}
	if n := strings.TrimSpace(req.Name); n != "" {
		d.Name = truncate(n, 40)
	} else if d.Name == "" {
		d.Name = i18n.F("Телефон %s", strings.ToUpper(id[:min(4, len(id))]))
	}

	var cmdErr error
	if req.Wheel != "" {
		for _, p := range align.AllPositions {
			if strings.EqualFold(req.Wheel, p.String()) {
				d.SetWheel(p)
			}
		}
	}
	if req.Cmd != "" {
		cmdErr = d.Start(req.Cmd)
	}

	now := l.now()
	calChanged := false
	var pushCamber []float64
	for _, s := range req.Samples {
		if s.DT <= 0 || s.DT > 2 {
			continue
		}
		if d.Feed(s, now) {
			calChanged = true
		}
		if d.Step == StepLive && d.camberOK && d.HasWheel {
			pushCamber = append(pushCamber, d.camber)
		}
	}
	if len(req.Samples) == 0 {
		d.LastSeen = now
	}
	sweep := d.sweepDone
	d.sweepDone = nil
	if calChanged {
		l.cals[id] = d.Cal
	}
	st := l.stateLocked(d)
	wheel, hasWheel, name := d.Wheel, d.HasWheel, d.Name
	sai := d.SAI
	l.mu.Unlock()

	if calChanged {
		l.saveCals()
	}
	src := "phone:" + id
	if hasWheel {
		detail := i18n.N("развал")
		if wheel.IsFront() {
			detail = i18n.N("развал, кастер")
		}
		l.hub.Touch(live.SourceInfo{ID: src, Kind: "phone", Name: name, Wheel: wheel.String(), Detail: detail})
		for _, c := range pushCamber {
			c := c
			_ = l.hub.Push(live.Input{Wheel: wheel, Camber: &c, Source: src})
		}
		if sweep != nil {
			var sv *float64
			if sai != nil {
				v := *sai
				sv = &v
			}
			_ = l.hub.PushSweep(live.SweepInput{Wheel: wheel, Caster: sweep.Caster.Deg(), SAI: sv, Source: src, Warnings: sweep.Warnings})
		}
	}
	// What the desktop thinks of this wheel's camber, so the phone can show
	// the number in the same colour as the big screen.
	if hasWheel {
		if p, ok := l.hub.Frame().Params[specs.KeyCamber+wheel.String()]; ok && p.Has {
			st.Status, st.Spec = p.Status, p.Spec
		}
	}
	return st, cmdErr
}

func (l *Link) stateLocked(d *Device) PhoneState {
	st := PhoneState{
		Step: d.Step, Prompt: d.Prompt, Error: d.Err, Progress: round2(d.Progress),
		Flat: d.Cal.Flat, Mounted: d.Cal.Mounted, Stable: d.stable, Gyro: d.hasGyro,
		Lang: string(i18n.Current()),
	}
	if d.HasWheel {
		st.Wheel, st.WheelName, st.Front = d.Wheel.String(), d.Wheel.Label(), d.Wheel.IsFront()
	}
	if d.camberOK {
		c := round2(d.camber)
		st.Camber = &c
	}
	switch d.Step {
	case StepCasterOut, StepCasterIn, StepCasterBack:
		y := round2(d.yaw)
		st.Steer = &y
	}
	if d.Caster != nil {
		c := round2(*d.Caster)
		st.Caster = &c
	}
	if d.hasRunout {
		r := round2(d.runout)
		st.Runout = &r
	}
	return st
}

// ── Calibration storage ──────────────────────────────────────────────

func (l *Link) calPath() string { return filepath.Join(l.dir, "phones.json") }

func (l *Link) loadCals() {
	b, err := os.ReadFile(l.calPath())
	if err != nil {
		return
	}
	var m map[string]Calibration
	if json.Unmarshal(b, &m) == nil {
		l.cals = m
	}
}

func (l *Link) saveCals() {
	l.mu.Lock()
	b, err := json.MarshalIndent(l.cals, "", "  ")
	l.mu.Unlock()
	if err != nil || l.dir == "" {
		return
	}
	_ = os.MkdirAll(l.dir, 0o755)
	tmp := l.calPath() + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		_ = os.Rename(tmp, l.calPath())
	}
}

// ErrDisabled is returned when the link is used while switched off.
var ErrDisabled = i18n.Err("доступ для телефонов выключен")

func sanitizeID(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
		if b.Len() >= 24 {
			break
		}
	}
	return b.String()
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

func round2(v float64) float64 {
	if v < 0 {
		return -float64(int64(-v*100+0.5)) / 100
	}
	return float64(int64(v*100+0.5)) / 100
}
