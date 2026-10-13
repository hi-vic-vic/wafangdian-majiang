package live

import (
	"context"
	"encoding/json"
	"github.com/coder/websocket"
	"net/http"
	"sync"
	"time"
)

type Server struct {
	Hub     *Hub
	mu      sync.Mutex
	clients map[string]map[string]*peer
}
type peer struct {
	conn  *websocket.Conn
	queue chan View
}

func NewServer() *Server { return &Server{Hub: New(), clients: map[string]map[string]*peer{}} }
func reply(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
func (s *Server) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		reply(w, 200, map[string]string{"status": "ok", "service": "live-mahjong"})
	})
	m.HandleFunc("POST /v1/rooms", func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			Name   string `json:"name"`
			Config Config `json:"config"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&q) != nil {
			reply(w, 400, map[string]string{"error": "请求格式错误"})
			return
		}
		v, t, e := s.Hub.Create(q.Name, q.Config)
		if e != nil {
			reply(w, 400, map[string]string{"error": e.Error()})
			return
		}
		reply(w, 201, map[string]any{"room": v, "token": t})
	})
	m.HandleFunc("POST /v1/rooms/{code}/join", func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			Name string `json:"name"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&q) != nil {
			reply(w, 400, map[string]string{"error": "请求格式错误"})
			return
		}
		v, t, e := s.Hub.Join(r.PathValue("code"), q.Name)
		if e != nil {
			reply(w, 400, map[string]string{"error": e.Error()})
			return
		}
		s.Broadcast(v.Code)
		reply(w, 200, map[string]any{"room": v, "token": t})
	})
	m.HandleFunc("GET /v1/rooms/{code}", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Hub.Snapshot(r.PathValue("code"), r.URL.Query().Get("token"))
		if e != nil {
			reply(w, 403, map[string]string{"error": e.Error()})
			return
		}
		reply(w, 200, v)
	})
	m.HandleFunc("GET /v1/rooms/{code}/ws", s.socket)
	return m
}
func (s *Server) Broadcast(code string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for token, p := range s.clients[code] {
		v, e := s.Hub.Snapshot(code, token)
		if e != nil {
			continue
		}
		select {
		case p.queue <- v:
		default:
			select {
			case <-p.queue:
			default:
			}
			select {
			case p.queue <- v:
			default:
			}
		}
	}
}
func (s *Server) socket(w http.ResponseWriter, r *http.Request) {
	code, token := r.PathValue("code"), r.URL.Query().Get("token")
	if _, e := s.Hub.Snapshot(code, token); e != nil {
		reply(w, 403, map[string]string{"error": "无权访问"})
		return
	}
	conn, e := websocket.Accept(w, r, nil)
	if e != nil {
		return
	}
	conn.SetReadLimit(4096)
	p := &peer{conn: conn, queue: make(chan View, 1)}
	s.mu.Lock()
	if s.clients[code] == nil {
		s.clients[code] = map[string]*peer{}
	}
	old := s.clients[code][token]
	s.clients[code][token] = p
	s.mu.Unlock()
	if old != nil {
		old.conn.CloseNow()
	}
	s.Hub.Connect(code, token, true)
	s.Broadcast(code)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	defer conn.CloseNow()
	defer func() {
		s.mu.Lock()
		current := s.clients[code][token] == p
		if current {
			delete(s.clients[code], token)
			s.Hub.Connect(code, token, false)
		}
		s.mu.Unlock()
		if current {
			s.Broadcast(code)
		}
	}()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			case v := <-p.queue:
				b, _ := json.Marshal(map[string]any{"type": "room_state", "room": v})
				wc, stop := context.WithTimeout(ctx, 3*time.Second)
				e := conn.Write(wc, websocket.MessageText, b)
				stop()
				if e != nil {
					cancel()
					conn.CloseNow()
					return
				}
			}
		}
	}()
	for {
		_, b, e := conn.Read(ctx)
		if e != nil {
			break
		}
		var msg struct {
			Move
			Revision int `json:"revision"`
		}
		if json.Unmarshal(b, &msg) != nil {
			continue
		}
		s.mu.Lock()
		active := s.clients[code][token] == p
		s.mu.Unlock()
		if !active {
			break
		}
		_, e = s.actRevision(code, token, msg.Move, msg.Revision)
		if e != nil {
			payload, _ := json.Marshal(map[string]string{"type": "error", "message": e.Error()})
			wc, stop := context.WithTimeout(ctx, 3*time.Second)
			_ = conn.Write(wc, websocket.MessageText, payload)
			stop()
		}
		s.Broadcast(code)
	}
	cancel()
	<-done
}
func (s *Server) RunClock(ctx context.Context) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			for _, code := range s.Hub.Tick() {
				s.Broadcast(code)
			}
		}
	}
}
