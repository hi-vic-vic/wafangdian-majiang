package live

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"sync"
	"time"
)

type Config struct {
	BaseScore int      `json:"baseScore"`
	AllowChi  bool     `json:"allowChi"`
	Wildcards bool     `json:"wildcards"`
	BigHands  []string `json:"bigHands"`
	Rounds    int      `json:"rounds"`
}
type Move struct {
	Type  string   `json:"type"`
	Tile  string   `json:"tile,omitempty"`
	Tiles []string `json:"tiles,omitempty"`
}
type Meld struct {
	Kind  string   `json:"kind"`
	Tiles []string `json:"tiles"`
}
type Player struct {
	Name      string   `json:"name"`
	Seat      int      `json:"seat"`
	Score     int      `json:"score"`
	Ready     bool     `json:"ready"`
	Connected bool     `json:"connected"`
	Count     int      `json:"count"`
	Melds     []Meld   `json:"melds"`
	Discards  []string `json:"discards"`
	token     string
	hand      []string
}
type Bill struct {
	Winner int    `json:"winner"`
	Source int    `json:"source"`
	Delta  [4]int `json:"delta"`
	Reason string `json:"reason"`
}
type View struct {
	Code       string   `json:"code"`
	Phase      string   `json:"phase"`
	Config     Config   `json:"config"`
	Players    []Player `json:"players"`
	Seat       int      `json:"seat"`
	Hand       []string `json:"hand"`
	DrawnTile  string   `json:"drawnTile,omitempty"`
	Turn       int      `json:"turn"`
	Dealer     int      `json:"dealer"`
	Wall       int      `json:"wall"`
	Actions    []Move   `json:"actions"`
	Deadline   int64    `json:"deadline"`
	Revision   int      `json:"revision"`
	HandNumber int      `json:"handNumber"`
	Bill       *Bill    `json:"bill"`
	History    []Bill   `json:"history"`
	Votes      []int    `json:"votes"`
}
type pending struct {
	from    int
	tile    string
	rob     bool
	options map[int][]Move
	answers map[int]Move
}
type Room struct {
	Code, Phase                    string
	Config                         Config
	Players                        []*Player
	wall                           []string
	Turn, Dealer, Revision, Number int
	deadline                       time.Time
	pending                        *pending
	gangs                          [4]int
	draws                          [4]int
	drawn                          [4]string
	interrupted                    bool
	Bill                           *Bill
	History                        []Bill
	votes                          map[int]bool
}
type Hub struct {
	Mu    sync.Mutex
	Rooms map[string]*Room
}

func New() *Hub { return &Hub{Rooms: map[string]*Room{}} }
func Token() string {
	b := make([]byte, 24)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func random(n int) int {
	v, e := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if e != nil {
		panic(e)
	}
	return int(v.Int64())
}
func has(a []string, s string) bool {
	for _, v := range a {
		if v == s {
			return true
		}
	}
	return false
}
func (h *Hub) Create(name string, c Config) (View, string, error) {
	h.Mu.Lock()
	defer h.Mu.Unlock()
	if len([]rune(name)) < 1 || len([]rune(name)) > 16 {
		return View{}, "", errors.New("昵称需1至16字")
	}
	if c.BaseScore == 0 {
		c.BaseScore = 1
	}
	if c.BaseScore != 1 && c.BaseScore != 2 && c.BaseScore != 5 && c.BaseScore != 10 {
		return View{}, "", errors.New("底分不合法")
	}
	if c.Wildcards {
		return View{}, "", errors.New("混牌映射尚未确认，联机暂不开混")
	}
	if c.Rounds != 0 {
		return View{}, "", errors.New("圈数定义待确认，联机采用逐局继续模式")
	}
	seen := map[string]bool{}
	for _, p := range c.BigHands {
		if seen[p] || !has([]string{"七小对", "十三幺", "清一色", "大三元", "大四喜", "天胡", "地胡"}, p) {
			return View{}, "", errors.New("番型不合法或重复")
		}
		seen[p] = true
	}
	code := ""
	for {
		code = fmt.Sprintf("%06d", 100000+random(900000))
		if h.Rooms[code] == nil {
			break
		}
	}
	token := Token()
	r := &Room{Code: code, Config: c, Phase: "lobby", Players: []*Player{{Name: name, Seat: 0, token: token}}, votes: map[int]bool{}}
	h.Rooms[code] = r
	return r.view(0), token, nil
}
func (h *Hub) Join(code, name string) (View, string, error) {
	h.Mu.Lock()
	defer h.Mu.Unlock()
	r := h.Rooms[code]
	if r == nil || r.Phase != "lobby" || len(r.Players) >= 4 {
		return View{}, "", errors.New("房间不存在、已满或已开始")
	}
	if len([]rune(name)) < 1 || len([]rune(name)) > 16 {
		return View{}, "", errors.New("昵称需1至16字")
	}
	t := Token()
	s := len(r.Players)
	r.Players = append(r.Players, &Player{Name: name, Seat: s, token: t})
	r.Revision++
	return r.view(s), t, nil
}
func (r *Room) seat(t string) int {
	for s, p := range r.Players {
		if p.token == t {
			return s
		}
	}
	return -1
}
func (h *Hub) Snapshot(code, token string) (View, error) {
	h.Mu.Lock()
	defer h.Mu.Unlock()
	r := h.Rooms[code]
	if r == nil || r.seat(token) < 0 {
		return View{}, errors.New("无权访问该房间")
	}
	return r.view(r.seat(token)), nil
}
func (h *Hub) Connect(code, token string, on bool) {
	h.Mu.Lock()
	defer h.Mu.Unlock()
	if r := h.Rooms[code]; r != nil && r.seat(token) >= 0 {
		r.Players[r.seat(token)].Connected = on
		r.Revision++
	}
}
func (r *Room) view(s int) View {
	ps := []Player{}
	for i, p := range r.Players {
		v := *p
		v.Count = len(p.hand)
		v.Discards = append([]string{}, p.Discards...)
		v.Melds = []Meld{}
		for _, m := range p.Melds {
			ts := append([]string{}, m.Tiles...)
			if i != s && m.Kind == "concealed" {
				ts = []string{"back", "back", "back", "back"}
			}
			v.Melds = append(v.Melds, Meld{m.Kind, ts})
		}
		ps = append(ps, v)
	}
	vs := []int{}
	for s := range r.votes {
		vs = append(vs, s)
	}
	sort.Ints(vs)
	var bill *Bill
	if r.Bill != nil {
		b := *r.Bill
		bill = &b
	}
	dl := int64(0)
	if !r.deadline.IsZero() {
		dl = r.deadline.UnixMilli()
	}
	return View{Code: r.Code, Phase: r.Phase, Config: r.Config, Players: ps, Seat: s, Hand: append([]string{}, r.Players[s].hand...), DrawnTile: r.drawn[s], Turn: r.Turn, Dealer: r.Dealer, Wall: len(r.wall), Actions: r.options(s), Deadline: dl, Revision: r.Revision, HandNumber: r.Number, Bill: bill, History: append([]Bill{}, r.History...), Votes: vs}
}
func deck() []string {
	d := []string{}
	for _, s := range []byte{'m', 'p', 's', 'z'} {
		max := 9
		if s == 'z' {
			max = 7
		}
		for n := 1; n <= max; n++ {
			for k := 0; k < 4; k++ {
				d = append(d, fmt.Sprintf("%d%c", n, s))
			}
		}
	}
	return d
}
func (r *Room) clock() { r.deadline = time.Now().Add(30 * time.Second) }
func (r *Room) start() {
	r.Number++
	if r.Number == 1 {
		r.Dealer = random(4)
	}
	r.wall = deck()
	for i := len(r.wall) - 1; i > 0; i-- {
		j := random(i + 1)
		r.wall[i], r.wall[j] = r.wall[j], r.wall[i]
	}
	r.gangs = [4]int{}
	r.draws = [4]int{}
	r.drawn = [4]string{}
	r.interrupted = false
	r.pending = nil
	r.Bill = nil
	r.votes = map[int]bool{}
	for _, p := range r.Players {
		p.Ready = false
		p.Melds = []Meld{}
		p.Discards = []string{}
		p.hand = append([]string{}, r.wall[:13]...)
		r.wall = r.wall[13:]
		sort.Strings(p.hand)
	}
	r.draw(r.Dealer, false)
}
func (r *Room) draw(s int, tail bool) {
	if len(r.wall) == 0 {
		r.finish(-1, -1)
		return
	}
	i := 0
	if tail {
		i = len(r.wall) - 1
	}
	t := r.wall[i]
	r.wall = append(r.wall[:i], r.wall[i+1:]...)
	r.Players[s].hand = append(r.Players[s].hand, t)
	r.drawn[s] = t
	r.draws[s]++
	r.Turn = s
	r.Phase = "running"
	r.clock()
}
func count(h []string, t string) int {
	n := 0
	for _, v := range h {
		if v == t {
			n++
		}
	}
	return n
}
func remove(h []string, t string, n int) []string {
	a := []string{}
	for _, v := range h {
		if v == t && n > 0 {
			n--
		} else {
			a = append(a, v)
		}
	}
	return a
}
