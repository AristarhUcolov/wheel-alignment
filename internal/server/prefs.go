package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"runtime"
	"sync"

	"github.com/AristarhUcolov/wheel-alignment/internal/desktop"
	"github.com/AristarhUcolov/wheel-alignment/internal/i18n"
	"github.com/AristarhUcolov/wheel-alignment/internal/specs"
)

// maxUIPrefs bounds what the interface may store: a theme, a few switches and
// a workshop's name and address fit many times over.
const maxUIPrefs = 16 << 10

// uiPrefs are the interface's own preferences. The program keeps them in the
// settings file but does not interpret them — the interface owns their shape.
type uiPrefs struct {
	mu  sync.Mutex
	raw json.RawMessage
}

// SetVersion is the program's version, shown in the interface and the report.
func (s *Server) SetVersion(v string) { s.version = v }

func (s *Server) loadUIPrefs() {
	if s.settingsPath == "" {
		return
	}
	st := desktop.LoadSettings(s.settingsPath)
	s.ui.mu.Lock()
	s.ui.raw = st.UI
	s.ui.mu.Unlock()
}

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	s.ui.mu.Lock()
	ui := s.ui.raw
	s.ui.mu.Unlock()
	if len(ui) == 0 {
		ui = json.RawMessage("{}")
	}
	writeJSON(w, http.StatusOK, map[string]any{"lang": string(i18n.Current()), "ui": ui})
}

// setSettings replaces the interface preferences with the object sent.
func (s *Server) setSettings(w http.ResponseWriter, r *http.Request) {
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxUIPrefs+1024))
	if err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, errors.New(i18n.T("настройки слишком большие")))
		return
	}
	var req struct {
		UI json.RawMessage `json:"ui"`
	}
	if err := json.Unmarshal(b, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	var obj map[string]any
	if err := json.Unmarshal(req.UI, &obj); err != nil || obj == nil {
		writeErr(w, http.StatusBadRequest, errors.New(i18n.T("настройки интерфейса должны быть объектом JSON")))
		return
	}
	if len(req.UI) > maxUIPrefs {
		writeErr(w, http.StatusRequestEntityTooLarge, errors.New(i18n.T("настройки слишком большие")))
		return
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, req.UI); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	ui := json.RawMessage(compact.Bytes())

	s.ui.mu.Lock()
	s.ui.raw = ui
	s.ui.mu.Unlock()
	if s.settingsPath != "" {
		st := desktop.LoadSettings(s.settingsPath)
		st.UI = ui
		if err := desktop.SaveSettings(s.settingsPath, st); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"lang": string(i18n.Current()), "ui": ui})
}

// about is what the "About" section shows.
func (s *Server) about(w http.ResponseWriter, r *http.Request) {
	v := s.version
	if v == "" {
		v = "dev"
	}
	makes := s.db.MakeCounts()
	entries := 0
	for _, m := range makes {
		entries += m.Count
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"version":  v,
		"data_dir": s.dataPath,
		"vehicles": entries,
		"makes":    len(makes),
		"platform": runtime.GOOS + "/" + runtime.GOARCH,
	})
}

// makeInfo is a make for browsing, with its region's name.
type makeInfo struct {
	specs.MakeCount
	RegionName string `json:"region_name"`
}

func (s *Server) makes(w http.ResponseWriter, r *http.Request) {
	mc := s.db.MakeCounts()
	out := make([]makeInfo, len(mc))
	for i, m := range mc {
		out[i] = makeInfo{MakeCount: m, RegionName: m.Region.Label()}
	}
	writeJSON(w, http.StatusOK, out)
}
