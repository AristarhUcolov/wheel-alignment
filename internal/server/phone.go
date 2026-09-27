package server

import (
	"encoding/json"
	"net/http"

	"github.com/AristarhUcolov/wheel-alignment/internal/desktop"
	"github.com/AristarhUcolov/wheel-alignment/internal/phone"
)

// openURL opens a link in the person's own browser. Inside the program's
// window a link would otherwise open in another bare window of the embedded
// engine; the donation pages and GitHub belong in the real browser, with its
// address bar and saved logins. Only an allow-listed set of sites is opened.
func (s *Server) openURL(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := desktop.OpenURL(req.URL); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// SetPhone attaches the phone link: the desktop interface switches it on and
// off and shows who is connected, and sensor heads on the network reach the
// open sensor protocol through it.
func (s *Server) SetPhone(l *phone.Link) {
	s.phone = l
	l.SetSensorHandler(http.HandlerFunc(s.liveSample))
}

func (s *Server) phoneInfo(w http.ResponseWriter, r *http.Request) {
	if s.phone == nil {
		writeJSON(w, http.StatusOK, phone.Info{Devices: []phone.DeviceInfo{}})
		return
	}
	writeJSON(w, http.StatusOK, s.phone.Info())
}

func (s *Server) phoneSet(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enable bool `json:"enable"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if s.phone == nil {
		writeErr(w, http.StatusServiceUnavailable, phone.ErrDisabled)
		return
	}
	if req.Enable {
		if err := s.phone.Enable(); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
	} else {
		s.phone.Disable()
	}
	writeJSON(w, http.StatusOK, s.phone.Info())
}
