package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/AristarhUcolov/wheel-alignment/internal/desktop"
	"github.com/AristarhUcolov/wheel-alignment/internal/i18n"
)

// SetSettingsFile tells the server where to keep the person's preferences —
// for now, the interface language.
func (s *Server) SetSettingsFile(path string) { s.settingsPath = path }

func (s *Server) getLang(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"lang": string(i18n.Current())})
}

// setLang switches the language of everything the program produces. The
// session is re-applied so that the tolerances on the live screen carry their
// labels and adjuster descriptions in the new language too.
func (s *Server) setLang(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Lang string `json:"lang"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	l := i18n.Lang(req.Lang)
	if !i18n.Valid(l) {
		writeErr(w, http.StatusBadRequest, errors.New("unsupported language / неподдерживаемый язык"))
		return
	}
	i18n.Set(l)
	if s.settingsPath != "" {
		st := desktop.LoadSettings(s.settingsPath)
		st.Lang = string(l)
		_ = desktop.SaveSettings(s.settingsPath, st)
	}
	s.sess.mu.Lock()
	s.applySessionLocked()
	s.sess.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"lang": string(l)})
}
