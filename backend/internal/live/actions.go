package live

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

func (r *Room) canWin(s int, t string) bool {
	h := append([]string{}, r.Players[s].hand...)
	if t != "" {
		h = append(h, t)
	}
	return winning(h, r.Config.BigHands)
}
func (r *Room) options(s int) []Move {
	a := []Move{}
	if r.Phase == "react" {
		if _, ok := r.pending.answers[s]; ok {
			return a
		}
		return append(a, r.pending.options[s]...)
	}
	if r.Phase != "running" || r.Turn != s {
		return a
	}
	if r.canWin(s, "") {
		a = append(a, Move{Type: "hu"})
	}
	p := r.Players[s]
	if len(r.wall) > 0 {
		seen := map[string]bool{}
		for _, t := range p.hand {
			if !seen[t] && count(p.hand, t) == 4 {
				a = append(a, Move{Type: "kong", Tile: t})
			}
			seen[t] = true
		}
		for _, m := range p.Melds {
			if m.Kind == "peng" && count(p.hand, m.Tiles[0]) > 0 {
				a = append(a, Move{Type: "add_kong", Tile: m.Tiles[0]})
			}
		}
	}
	return a
}
func (r *Room) ask(from int, t string, rob bool) {
	p := &pending{from: from, tile: t, rob: rob, options: map[int][]Move{}, answers: map[int]Move{}}
	r.pending = p
	for step := 1; step < 4; step++ {
		s := (from + step) % 4
		a := []Move{}
		if r.canWin(s, t) {
			a = append(a, Move{Type: "hu"})
		}
		h := r.Players[s].hand
		if !rob {
			n := count(h, t)
			if n >= 3 && len(r.wall) > 0 {
				a = append(a, Move{Type: "kong", Tile: t})
			}
			if n >= 2 {
				a = append(a, Move{Type: "peng", Tile: t})
			}
			if r.Config.AllowChi && step == 1 && t[1] != 'z' {
				rank := int(t[0] - '0')
				for start := rank - 2; start <= rank; start++ {
					if start < 1 || start > 7 {
						continue
					}
					ts := []string{}
					for v := start; v < start+3; v++ {
						if v != rank {
							ts = append(ts, fmt.Sprintf("%d%c", v, t[1]))
						}
					}
					if count(h, ts[0]) > 0 && count(h, ts[1]) > 0 {
						a = append(a, Move{Type: "chi", Tile: t, Tiles: ts})
					}
				}
			}
		}
		if len(a) > 0 {
			p.options[s] = append(a, Move{Type: "pass"})
		}
	}
	if len(p.options) == 0 {
		r.resolve()
		return
	}
	r.Phase = "react"
	r.clock()
}
func equal(a, b Move) bool {
	if a.Type != b.Type || a.Tile != b.Tile || len(a.Tiles) != len(b.Tiles) {
		return false
	}
	for i := range a.Tiles {
		if a.Tiles[i] != b.Tiles[i] {
			return false
		}
	}
	return true
}
func (r *Room) resolve() {
	p := r.pending
	priority := map[string]int{"hu": 4, "kong": 3, "peng": 2, "chi": 1}
	best := -1
	m := Move{}
	for d := 1; d < 4; d++ {
		s := (p.from + d) % 4
		v := p.answers[s]
		if priority[v.Type] > priority[m.Type] {
			best = s
			m = v
		}
	}
	r.pending = nil
	if best < 0 {
		if p.rob {
			r.completeAdd(p.from, p.tile)
		} else {
			r.draw((p.from+1)%4, false)
		}
		return
	}
	from := r.Players[p.from]
	target := r.Players[best]
	if p.rob {
		from.hand = remove(from.hand, p.tile, 1)
	} else {
		from.Discards = from.Discards[:len(from.Discards)-1]
	}
	if m.Type == "hu" {
		target.hand = append(target.hand, p.tile)
		r.finish(best, p.from)
		return
	}
	ts := []string{p.tile}
	if m.Type == "chi" {
		for _, t := range m.Tiles {
			target.hand = remove(target.hand, t, 1)
			ts = append(ts, t)
		}
	} else {
		n := 2
		if m.Type == "kong" {
			n = 3
		}
		target.hand = remove(target.hand, p.tile, n)
		for k := 0; k < n; k++ {
			ts = append(ts, p.tile)
		}
	}
	sort.Strings(ts)
	target.Melds = append(target.Melds, Meld{m.Type, ts})
	r.interrupted = true
	r.Turn = best
	r.Phase = "running"
	r.clock()
	if m.Type == "kong" {
		r.gangs[best] += 5
		r.draw(best, true)
	}
}
func (r *Room) completeAdd(s int, t string) {
	p := r.Players[s]
	p.hand = remove(p.hand, t, 1)
	for i, m := range p.Melds {
		if m.Kind == "peng" && m.Tiles[0] == t {
			p.Melds[i] = Meld{"kong", []string{t, t, t, t}}
			break
		}
	}
	r.gangs[s] += 5
	r.interrupted = true
	r.draw(s, true)
}
func (r *Room) finish(w, source int) {
	b := Bill{Winner: w, Source: source, Reason: "荒庄，杠分作废"}
	if w >= 0 {
		mul, why := r.multiplier(w, source)
		pay := r.Config.BaseScore * mul
		b.Reason = why
		for i := 0; i < 4; i++ {
			if i != w && (source < 0 || source == i) {
				b.Delta[i] -= pay
				b.Delta[w] += pay
			}
		}
		for owner, g := range r.gangs {
			for payer := 0; payer < 4; payer++ {
				if owner != payer {
					b.Delta[payer] -= g
					b.Delta[owner] += g
				}
			}
		}
		r.Dealer = w
	}
	for i, p := range r.Players {
		p.Score += b.Delta[i]
		p.Ready = false
	}
	r.Bill = &b
	r.History = append(r.History, b)
	r.Phase = "settled"
	r.deadline = time.Time{}
	r.pending = nil
	r.votes = map[int]bool{}
}
func (h *Hub) Act(code, token string, m Move) (View, error) {
	h.Mu.Lock()
	defer h.Mu.Unlock()
	r := h.Rooms[code]
	if r == nil || r.seat(token) < 0 {
		return View{}, errors.New("无权访问")
	}
	s := r.seat(token)
	if e := r.act(s, m); e != nil {
		return View{}, e
	}
	r.Revision++
	return r.view(s), nil
}
func (r *Room) act(s int, m Move) error {
	if r.Phase == "closed" {
		return errors.New("房间已解散")
	}
	if m.Type == "dissolve" {
		if r.Phase == "lobby" {
			if s != 0 {
				return errors.New("仅房主可解散未开局房间")
			}
			r.Phase = "closed"
		} else {
			r.votes[s] = true
			if len(r.votes) == 4 {
				if r.Phase == "running" || r.Phase == "react" {
					r.finish(-1, -1)
				}
				r.Phase = "closed"
			}
		}
		return nil
	}
	if m.Type == "cancel_vote" {
		r.votes = map[int]bool{}
		return nil
	}
	if m.Type == "ready" {
		if r.Phase != "lobby" && r.Phase != "settled" {
			return errors.New("当前不能准备")
		}
		r.Players[s].Ready = true
		if len(r.Players) == 4 {
			all := true
			for _, p := range r.Players {
				all = all && p.Ready
			}
			if all {
				r.start()
			}
		}
		return nil
	}
	if m.Type == "discard" {
		if r.Phase != "running" || r.Turn != s {
			return errors.New("还未轮到你出牌")
		}
		p := r.Players[s]
		if count(p.hand, m.Tile) == 0 {
			return errors.New("不能打出不在手中的牌")
		}
		r.drawn[s] = ""
		p.hand = remove(p.hand, m.Tile, 1)
		p.Discards = append(p.Discards, m.Tile)
		r.ask(s, m.Tile, false)
		return nil
	}
	valid := false
	for _, a := range r.options(s) {
		if equal(a, m) {
			valid = true
		}
	}
	if !valid {
		return errors.New("当前不能执行该动作")
	}
	if r.Phase == "react" {
		r.pending.answers[s] = m
		if len(r.pending.answers) == len(r.pending.options) {
			r.resolve()
		}
		return nil
	}
	switch m.Type {
	case "hu":
		r.finish(s, -1)
	case "kong":
		p := r.Players[s]
		p.hand = remove(p.hand, m.Tile, 4)
		p.Melds = append(p.Melds, Meld{"concealed", []string{m.Tile, m.Tile, m.Tile, m.Tile}})
		r.gangs[s] += 10
		r.interrupted = true
		r.draw(s, true)
	case "add_kong":
		r.ask(s, m.Tile, true)
	}
	return nil
}
func (h *Hub) Tick() []string {
	h.Mu.Lock()
	defer h.Mu.Unlock()
	changed := []string{}
	for code, r := range h.Rooms {
		if r.deadline.IsZero() || time.Now().Before(r.deadline) {
			continue
		}
		if r.Phase == "running" {
			s := r.Turn
			_ = r.act(s, Move{Type: "discard", Tile: r.Players[s].hand[len(r.Players[s].hand)-1]})
		} else if r.Phase == "react" {
			for s := range r.pending.options {
				if _, ok := r.pending.answers[s]; !ok {
					r.pending.answers[s] = Move{Type: "pass"}
				}
			}
			r.resolve()
		} else {
			continue
		}
		r.Revision++
		changed = append(changed, code)
	}
	return changed
}
