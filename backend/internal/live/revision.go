package live

import "errors"

func (s *Server) actRevision(code, token string, m Move, rev int) (View, error) {
	h := s.Hub
	h.Mu.Lock()
	defer h.Mu.Unlock()
	r := h.Rooms[code]
	if r == nil || r.seat(token) < 0 {
		return View{}, errors.New("无权访问")
	}
	if rev != r.Revision {
		return View{}, errors.New("牌局已更新，请重试")
	}
	seat := r.seat(token)
	if e := r.act(seat, m); e != nil {
		return View{}, e
	}
	r.Revision++
	return r.view(seat), nil
}
