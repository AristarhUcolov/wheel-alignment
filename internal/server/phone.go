package server

import (
	"encoding/json"
	"net/http"

	"github.com/AristarhUcolov/wheel-alignment/internal/phone"
)

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
