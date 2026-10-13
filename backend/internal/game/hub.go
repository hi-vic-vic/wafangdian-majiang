package game

import (
	"crypto/rand"
	"errors"
	"fmt"
	"sync"
)

type Config struct {
	Rounds      int      `json:"rounds"`
	BaseScore   int      `json:"baseScore"`
	AllowChi    bool     `json:"allowChi"`
	Wildcards   bool     `json:"wildcards"`
	BigHands    []string `json:"bigHands"`
	TurnSeconds int      `json:"turnSeconds"`
}

func DefaultConfig() Config {
	return Config{Rounds: 4, BaseScore: 1, TurnSeconds: 30, BigHands: []string{}}
}

type Player struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Seat      int    `json:"seat"`
	Ready     bool   `json:"ready"`
	Connected bool   `json:"connected"`
	Score     int    `json:"score"`
}
type RoomView struct {
	Code    string   `json:"code"`
	Phase   string   `json:"phase"`
	Config  Config   `json:"config"`
	Players []Player `json:"players"`
	Turn    int      `json:"turn"`
	Events  []string `json:"events"`
}
type room struct {
	code, phase string
	config      Config
	players     map[string]*Player
	order       []string
	turn        int
	events      []string
}
type Hub struct {
	mu    sync.RWMutex
	rooms map[string]*room
}

func NewHub() *Hub { return &Hub{rooms: map[string]*room{}} }

func (h *Hub) CreateRoom(hostID, hostName string, c Config) (RoomView, error) {
	if hostID == "" || hostName == "" {
		return RoomView{}, errors.New("房主身份和昵称不能为空")
	}
	if err := validateConfig(c); err != nil {
		return RoomView{}, err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	code, err := h.code()
	if err != nil {
		return RoomView{}, err
	}
	r := &room{code: code, phase: "lobby", config: c, players: map[string]*Player{}, order: []string{hostID}, events: []string{"房间已创建，等待牌友加入"}}
	r.players[hostID] = &Player{ID: hostID, Name: hostName, Seat: 0, Connected: true}
	h.rooms[code] = r
	return r.view(), nil
}
func (h *Hub) JoinRoom(code, id, name string) (RoomView, error) {
	if id == "" || name == "" {
		return RoomView{}, errors.New("玩家身份和昵称不能为空")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	r, ok := h.rooms[code]
	if !ok {
		return RoomView{}, errors.New("房间不存在或已结束")
	}
	if r.phase != "lobby" {
		return RoomView{}, errors.New("牌局已开始，不能加入")
	}
	if p, ok := r.players[id]; ok {
		p.Connected = true
		r.events = append(r.events, fmt.Sprintf("%s 已重新连接", p.Name))
		return r.view(), nil
	}
	if len(r.order) >= 4 {
		return RoomView{}, errors.New("房间已满")
	}
	r.players[id] = &Player{ID: id, Name: name, Seat: len(r.order), Connected: true}
	r.order = append(r.order, id)
	r.events = append(r.events, fmt.Sprintf("%s 加入房间", name))
	return r.view(), nil
}
func (h *Hub) Ready(code, id string, ready bool) (RoomView, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	r, p, e := h.player(code, id)
	if e != nil {
		return RoomView{}, e
	}
	if r.phase != "lobby" {
		return RoomView{}, errors.New("牌局已开始，准备状态不可修改")
	}
	p.Ready = ready
	if len(r.order) == 4 && allReady(r) {
		r.phase = "running"
		r.turn = 0
		r.events = append(r.events, "四名玩家已准备，牌局开始")
	} else {
		r.events = append(r.events, fmt.Sprintf("%s %s准备", p.Name, map[bool]string{true: "已", false: "取消"}[ready]))
	}
	return r.view(), nil
}
func (h *Hub) Discard(code, id, tile string) (RoomView, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	r, p, e := h.player(code, id)
	if e != nil {
		return RoomView{}, e
	}
	if r.phase != "running" {
		return RoomView{}, errors.New("牌局尚未开始")
	}
	if r.order[r.turn] != id {
		return RoomView{}, errors.New("尚未轮到你出牌")
	}
	if !validTile(tile) {
		return RoomView{}, errors.New("牌编码不合法")
	}
	r.events = append(r.events, fmt.Sprintf("%s 打出 %s", p.Name, tile))
	r.turn = (r.turn + 1) % 4
	return r.view(), nil
}
func (h *Hub) Get(code string) (RoomView, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	r, ok := h.rooms[code]
	if !ok {
		return RoomView{}, errors.New("房间不存在或已结束")
	}
	return r.view(), nil
}
func (h *Hub) Connected(code, id string, on bool) (RoomView, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	r, p, e := h.player(code, id)
	if e != nil {
		return RoomView{}, e
	}
	p.Connected = on
	if !on {
		r.events = append(r.events, fmt.Sprintf("%s 已断线，等待重连或托管", p.Name))
	}
	return r.view(), nil
}
func (h *Hub) player(code, id string) (*room, *Player, error) {
	r, ok := h.rooms[code]
	if !ok {
		return nil, nil, errors.New("房间不存在或已结束")
	}
	p, ok := r.players[id]
	if !ok {
		return nil, nil, errors.New("你不在这个房间中")
	}
	return r, p, nil
}
func (h *Hub) code() (string, error) {
	for i := 0; i < 8; i++ {
		var b [3]byte
		if _, e := rand.Read(b[:]); e != nil {
			return "", e
		}
		code := fmt.Sprintf("%02d%02d%02d", int(b[0])%100, int(b[1])%100, int(b[2])%100)
		if _, ok := h.rooms[code]; !ok {
			return code, nil
		}
	}
	return "", errors.New("暂时无法生成房间号")
}
func (r *room) view() RoomView {
	ps := make([]Player, 0, len(r.order))
	for _, id := range r.order {
		ps = append(ps, *r.players[id])
	}
	events := append([]string(nil), r.events...)
	if len(events) > 20 {
		events = events[len(events)-20:]
	}
	return RoomView{Code: r.code, Phase: r.phase, Config: r.config, Players: ps, Turn: r.turn, Events: events}
}
func allReady(r *room) bool {
	for _, id := range r.order {
		if !r.players[id].Ready {
			return false
		}
	}
	return true
}
func validTile(t string) bool {
	if len(t) != 2 || t[0] < '1' || t[0] > '9' {
		return false
	}
	if t[1] == 'z' {
		return t[0] <= '7'
	}
	return t[1] == 'm' || t[1] == 'p' || t[1] == 's'
}
func validateConfig(c Config) error {
	if c.Rounds != 1 && c.Rounds != 4 && c.Rounds != 8 {
		return errors.New("局数只支持 1、4 或 8 圈")
	}
	if c.BaseScore != 1 && c.BaseScore != 2 && c.BaseScore != 5 && c.BaseScore != 10 {
		return errors.New("底分不合法")
	}
	if c.TurnSeconds != 30 {
		return errors.New("首版操作时长固定为 30 秒")
	}
	return nil
}
