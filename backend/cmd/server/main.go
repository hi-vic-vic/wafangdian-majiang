package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/coder/websocket"
	"github.com/lmc10242/wafangdian-majiang/backend/internal/game"
	"log"
	"net/http"
	"sync"
	"time"
)

type server struct {
	hub     *game.Hub
	mu      sync.Mutex
	clients map[string]map[*client]struct{}
}
type client struct {
	conn *websocket.Conn
	mu   sync.Mutex
}
type createReq struct {
	HostID   string      `json:"hostId"`
	HostName string      `json:"hostName"`
	Config   game.Config `json:"config"`
}
type joinReq struct {
	PlayerID string `json:"playerId"`
	Name     string `json:"name"`
}
type message struct {
	Type  string `json:"type"`
	Ready bool   `json:"ready"`
	Tile  string `json:"tile"`
}

func main() {
	s := &server{hub: game.NewHub(), clients: map[string]map[*client]struct{}{}}
	m := http.NewServeMux()
	m.HandleFunc("GET /healthz", s.health)
	m.HandleFunc("POST /v1/rooms", s.create)
	m.HandleFunc("GET /v1/rooms/{code}", s.get)
	m.HandleFunc("POST /v1/rooms/{code}/join", s.join)
	m.HandleFunc("GET /v1/rooms/{code}/ws", s.ws)
	log.Println("瓦房店推倒胡服务端监听 :8080")
	log.Fatal(http.ListenAndServe(":8080", cors(m)))
}
func (s *server) health(w http.ResponseWriter, _ *http.Request) {
	write(w, http.StatusOK, map[string]string{"status": "ok"})
}
func (s *server) create(w http.ResponseWriter, r *http.Request) {
	var q createReq
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&q) != nil {
		fail(w, "请求格式错误")
		return
	}
	if q.Config.Rounds == 0 {
		q.Config = game.DefaultConfig()
	}
	v, e := s.hub.CreateRoom(q.HostID, q.HostName, q.Config)
	if e != nil {
		fail(w, e.Error())
		return
	}
	write(w, http.StatusCreated, v)
}
func (s *server) get(w http.ResponseWriter, r *http.Request) {
	v, e := s.hub.Get(r.PathValue("code"))
	if e != nil {
		fail(w, e.Error())
		return
	}
	write(w, http.StatusOK, v)
}
func (s *server) join(w http.ResponseWriter, r *http.Request) {
	var q joinReq
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&q) != nil {
		fail(w, "请求格式错误")
		return
	}
	v, e := s.hub.JoinRoom(r.PathValue("code"), q.PlayerID, q.Name)
	if e != nil {
		fail(w, e.Error())
		return
	}
	s.broadcast(v.Code, v)
	write(w, http.StatusOK, v)
}
func (s *server) ws(w http.ResponseWriter, r *http.Request) {
	code, id := r.PathValue("code"), r.URL.Query().Get("playerId")
	if id == "" {
		fail(w, "缺少 playerId")
		return
	}
	if _, e := s.hub.Get(code); e != nil {
		fail(w, e.Error())
		return
	}
	conn, e := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if e != nil {
		return
	}
	defer conn.CloseNow()
	c := &client{conn: conn}
	s.add(code, c)
	defer s.remove(code, c)
	if v, e := s.hub.Connected(code, id, true); e == nil {
		s.broadcast(code, v)
	}
	for {
		_, body, e := conn.Read(context.Background())
		if e != nil {
			break
		}
		var msg message
		if json.Unmarshal(body, &msg) != nil {
			s.send(c, map[string]string{"type": "error", "message": "消息格式错误"})
			continue
		}
		var v game.RoomView
		switch msg.Type {
		case "ready":
			v, e = s.hub.Ready(code, id, msg.Ready)
		case "discard":
			v, e = s.hub.Discard(code, id, msg.Tile)
		default:
			e = errors.New("不支持的牌局动作")
		}
		if e != nil {
			s.send(c, map[string]string{"type": "error", "message": e.Error()})
			continue
		}
		s.broadcast(code, v)
	}
	if v, e := s.hub.Connected(code, id, false); e == nil {
		s.broadcast(code, v)
	}
}
func (s *server) add(code string, c *client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.clients[code] == nil {
		s.clients[code] = map[*client]struct{}{}
	}
	s.clients[code][c] = struct{}{}
}
func (s *server) remove(code string, c *client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients[code], c)
	if len(s.clients[code]) == 0 {
		delete(s.clients, code)
	}
}
func (s *server) broadcast(code string, v any) {
	s.mu.Lock()
	cs := make([]*client, 0, len(s.clients[code]))
	for c := range s.clients[code] {
		cs = append(cs, c)
	}
	s.mu.Unlock()
	for _, c := range cs {
		s.send(c, map[string]any{"type": "room_state", "room": v})
	}
}
func (s *server) send(c *client, v any) {
	b, e := json.Marshal(v)
	if e != nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = c.conn.Write(ctx, websocket.MessageText, b)
}
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, msg string) {
	write(w, http.StatusBadRequest, map[string]string{"error": msg})
}
