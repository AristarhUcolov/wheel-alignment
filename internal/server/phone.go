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
	// Besides its fixed list, the interface may open the page a built-in
	// figure was taken from — so that anyone can check it at the source.
	var err error
	if !desktop.Allowed(req.URL) && s.db.IsSourceURL(req.URL) {
		err = desktop.Open(req.URL)
	} else {
		err = desktop.OpenURL(req.URL)
	}
	if err != nil {
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
		// Forget disconnects one phone, Allow lets it back; either leaves
		// access as it is.
		Forget string `json:"forget"`
		Allow  string `json:"allow"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if s.phone == nil {
		writeErr(w, http.StatusServiceUnavailable, phone.ErrDisabled)
		return
	}
	switch {
	case req.Forget != "":
		s.phone.Forget(req.Forget)
	case req.Allow != "":
		s.phone.Allow(req.Allow)
	case req.Enable:
		if err := s.phone.Enable(); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
	default:
		s.phone.Disable()
	}
	writeJSON(w, http.StatusOK, s.phone.Info())
}
