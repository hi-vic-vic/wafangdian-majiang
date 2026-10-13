package live

// Tile encodings use 东南西北, but the local indicator cycle is 东西南北.
func nextWildTile(t string) string {
	if t[1] != 'z' {
		return string([]byte{byte((t[0]-'1'+1)%9 + '1'), t[1]})
	}
	return map[string]string{"1z": "3z", "3z": "2z", "2z": "4z", "4z": "1z", "5z": "6z", "6z": "7z", "7z": "5z"}[t]
}

type Opening struct {
	FirstDice      [2]int `json:"firstDice"`
	SecondDice     [2]int `json:"secondDice"`
	WallSeat       int    `json:"wallSeat"`
	IndicatorStack int    `json:"indicatorStack"`
	BreakStacks    int    `json:"breakStacks"`
}

// Four walls of 17 stacks, ordered right-to-left in each owner's view.
// Count the dealer as one, moving in the same direction as seat turns.
// The break skips the summed number of stacks, wrapping into the next wall.
func openWildWall(wall []string, dealer int, first, second [2]int) ([]string, string, Opening) {
	one, two := first[0]+first[1], second[0]+second[1]
	seat := (dealer + one - 1) % 4
	indicatorIndex := seat*34 + (one-1)*2
	indicator := wall[indicatorIndex]
	start := (seat*34 + (one+two)*2) % len(wall)
	result := make([]string, 0, len(wall)-1)
	for step := 0; step < len(wall); step++ {
		i := (start + step) % len(wall)
		if i != indicatorIndex {
			result = append(result, wall[i])
		}
	}
	return result, indicator, Opening{first, second, seat, one, one + two}
}

func splitWild(h []string, wildTile string) ([34]int, int) {
	c := counts(h)
	if wildTile == "" {
		return c, 0
	}
	w := counts([]string{wildTile})
	for i, v := range w {
		if v > 0 {
			wild := c[i]
			c[i] = 0
			return c, wild
		}
	}
	return c, 0
}

func standardWild(c [34]int, wild int) bool {
	total := wild
	for _, n := range c {
		total += n
	}
	if total%3 != 2 {
		return false
	}
	memo := map[[35]int]bool{}
	known := map[[35]int]bool{}
	var groups func([34]int, int) bool
	groups = func(c [34]int, w int) bool {
		key := [35]int{}
		copy(key[:34], c[:])
		key[34] = w
		if known[key] {
			return memo[key]
		}
		i := 0
		for i < 34 && c[i] == 0 {
			i++
		}
		if i == 34 {
			return w%3 == 0
		}
		known[key] = true
		// Sometimes a natural duplicate must be saved for another sequence.
		for take := 1; take <= 3 && take <= c[i]; take++ {
			if 3-take <= w {
				next := c
				next[i] -= take
				if groups(next, w-(3-take)) {
					memo[key] = true
					return true
				}
			}
		}
		if i < 27 {
			for start := i - 2; start <= i; start++ {
				if start < 0 || start/9 != i/9 || (start+2)/9 != i/9 {
					continue
				}
				// A mixed sequence must consume the first remaining natural tile.
				for mask := 0; mask < 8; mask++ {
					if mask&(1<<uint(i-start)) == 0 {
						continue
					}
					next := c
					need := 0
					valid := true
					for j := 0; j < 3; j++ {
						if mask&(1<<uint(j)) != 0 {
							if next[start+j] == 0 {
								valid = false
								break
							}
							next[start+j]--
						} else {
							need++
						}
					}
					if valid && need <= w && groups(next, w-need) {
						memo[key] = true
						return true
					}
				}
			}
		}
		return false
	}
	for i, n := range c {
		for take := 0; take <= 2 && take <= n; take++ {
			if 2-take <= wild {
				next := c
				next[i] -= take
				if groups(next, wild-(2-take)) {
					return true
				}
			}
		}
	}
	return false
}
func sevenWild(c [34]int, wild int) bool {
	total, odd := wild, 0
	for _, n := range c {
		total += n
		odd += n % 2
	}
	return total == 14 && odd <= wild
}
func orphanWild(c [34]int, wild int) bool {
	allowed := map[int]bool{}
	for _, i := range []int{0, 8, 9, 17, 18, 26, 27, 28, 29, 30, 31, 32, 33} {
		allowed[i] = true
	}
	total := wild
	for i, n := range c {
		total += n
		if n > 0 && !allowed[i] {
			return false
		}
	}
	if total != 14 {
		return false
	}
	for pair := range allowed {
		needed := 0
		valid := true
		for i := range allowed {
			target := 1
			if i == pair {
				target = 2
			}
			if c[i] > target {
				valid = false
				break
			}
			needed += target - c[i]
		}
		if valid && needed == wild {
			return true
		}
	}
	return false
}
func winningWild(h []string, en []string, wildTile string) bool {
	if wildTile == "" {
		return winning(h, en)
	}
	c, w := splitWild(h, wildTile)
	return standardWild(c, w) || (has(en, "七小对") && sevenWild(c, w)) || (has(en, "十三幺") && orphanWild(c, w))
}
func (r *Room) isWild(t string) bool { return r.WildTile != "" && t == r.WildTile }
func meldFaces(m Meld) []string {
	if len(m.Faces) > 0 {
		return m.Faces
	}
	return m.Tiles
}
func (r *Room) discardTile(s int) string {
	h := r.Players[s].hand
	for i := len(h) - 1; i >= 0; i-- {
		if !r.isWild(h[i]) {
			return h[i]
		}
	}
	return ""
}

// Honor big hands must still admit a complete decomposition after allocating
// mixed tiles to their required triplets; counting mixed tiles alone is not enough.
func (r *Room) honorPattern(s int, required []int) bool {
	c, w := splitWild(r.Players[s].hand, r.WildTile)
	for _, i := range required {
		found := false
		for _, m := range r.Players[s].Melds {
			faces := meldFaces(m)
			if m.Kind != "chi" && len(faces) >= 3 && counts(faces)[i] >= 3 {
				found = true
				break
			}
		}
		if found {
			continue
		}
		take := c[i]
		if take > 3 {
			take = 3
		}
		need := 3 - take
		if need > w {
			return false
		}
		c[i] -= take
		w -= need
	}
	return standardWild(c, w)
}

// Keep alternative natural/mixed choices, recording actual tiles spent.
func (r *Room) meldSupplies(h, desired []string) [][]string {
	result := [][]string{}
	seen := map[string]bool{}
	var visit func(int, []string)
	visit = func(i int, spent []string) {
		if i == len(desired) {
			key := ""
			for _, t := range spent {
				key += t + ","
			}
			if !seen[key] {
				seen[key] = true
				result = append(result, append([]string{}, spent...))
			}
			return
		}
		t := desired[i]
		if !r.isWild(t) && count(h, t) > count(spent, t) {
			visit(i+1, append(spent, t))
		}
		if r.WildTile != "" && count(h, r.WildTile) > count(spent, r.WildTile) {
			visit(i+1, append(spent, r.WildTile))
		}
	}
	visit(0, nil)
	return result
}
