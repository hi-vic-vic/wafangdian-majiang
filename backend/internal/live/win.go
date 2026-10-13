package live

import (
	"fmt"
	"strings"
)

func counts(h []string) [34]int {
	var c [34]int
	for _, t := range h {
		off := map[byte]int{'m': 0, 'p': 9, 's': 18, 'z': 27}[t[1]]
		c[off+int(t[0]-'1')]++
	}
	return c
}
func melds(c [34]int) bool {
	i := 0
	for i < 34 && c[i] == 0 {
		i++
	}
	if i == 34 {
		return true
	}
	if c[i] >= 3 {
		c[i] -= 3
		if melds(c) {
			return true
		}
		c[i] += 3
	}
	if i < 27 && i%9 <= 6 && c[i+1] > 0 && c[i+2] > 0 {
		c[i]--
		c[i+1]--
		c[i+2]--
		if melds(c) {
			return true
		}
	}
	return false
}
func seven(c [34]int) bool {
	n := 0
	for _, v := range c {
		if v%2 != 0 {
			return false
		}
		n += v
	}
	return n == 14
}
func orphan(c [34]int) bool {
	total := 0
	for _, v := range c {
		total += v
	}
	if total != 14 {
		return false
	}
	for _, i := range []int{0, 8, 9, 17, 18, 26, 27, 28, 29, 30, 31, 32, 33} {
		if c[i] == 0 {
			return false
		}
	}
	return true
}
func winning(h []string, en []string) bool {
	if len(h)%3 != 2 {
		return false
	}
	c := counts(h)
	if len(h) == 14 && ((has(en, "七小对") && seven(c)) || (has(en, "十三幺") && orphan(c))) {
		return true
	}
	for i, v := range c {
		if v >= 2 {
			next := c
			next[i] -= 2
			if melds(next) {
				return true
			}
		}
	}
	return false
}
func (r *Room) multiplier(s, source int) (int, string) {
	mul := 2
	names := []string{"点炮×2"}
	if source < 0 {
		names = []string{"自摸×2"}
	}
	if s == r.Dealer {
		mul *= 2
		names = append(names, "庄家×2")
	}
	h := r.Players[s].hand
	c := counts(h)
	all := append([]string{}, h...)
	for _, m := range r.Players[s].Melds {
		all = append(all, m.Tiles...)
	}
	ac := counts(all)
	suit := byte(0)
	pure := true
	for _, t := range all {
		if t[1] == 'z' {
			pure = false
		}
		if suit == 0 {
			suit = t[1]
		} else if suit != t[1] {
			pure = false
		}
	}
	ps := map[string]bool{"七小对": len(h) == 14 && seven(c), "十三幺": len(h) == 14 && orphan(c), "清一色": pure, "大三元": ac[31] >= 3 && ac[32] >= 3 && ac[33] >= 3, "大四喜": ac[27] >= 3 && ac[28] >= 3 && ac[29] >= 3 && ac[30] >= 3, "天胡": source < 0 && s == r.Dealer && r.draws[s] == 1 && !r.interrupted && len(r.Players[s].Discards) == 0, "地胡": source < 0 && s != r.Dealer && r.draws[s] == 1 && !r.interrupted}
	for _, name := range r.Config.BigHands {
		if ps[name] {
			v := 2
			if name == "天胡" {
				v = 8
			}
			if name == "地胡" {
				v = 6
			}
			mul *= v
			names = append(names, fmt.Sprintf("%s×%d", name, v))
		}
	}
	return mul, strings.Join(names, "，")
}
