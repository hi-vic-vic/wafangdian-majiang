package live

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestWildIndicatorCycles(t *testing.T) {
	for _, s := range []byte{'m', 'p', 's'} {
		for n := byte('1'); n <= '9'; n++ {
			want := string([]byte{(n-'1'+1)%9 + '1', s})
			if got := nextWildTile(string([]byte{n, s})); got != want {
				t.Fatal(got, want)
			}
		}
	}
	for _, cycle := range [][]string{{"1z", "3z", "2z", "4z"}, {"5z", "6z", "7z"}} {
		for i, s := range cycle {
			if nextWildTile(s) != cycle[(i+1)%len(cycle)] {
				t.Fatal(s)
			}
		}
	}
}
func TestOpeningDiceAndIndicatorRemoval(t *testing.T) {
	wall := deck()
	first, second := [2]int{3, 4}, [2]int{2, 5}
	got, indicator, o := openWildWall(wall, 0, first, second)
	if o.WallSeat != 2 || o.IndicatorStack != 7 || o.BreakStacks != 14 || indicator != wall[80] || got[0] != wall[96] || len(got) != 135 {
		t.Fatal(o, indicator, got[0])
	}
	// High sums wrap across walls; each of the 135 physical positions occurs once.
	for dealer := 0; dealer < 4; dealer++ {
		for one := 2; one <= 12; one++ {
			for two := 2; two <= 12; two++ {
				f := [2]int{1, one - 1}
				q := [2]int{1, two - 1}
				got, i, _ := openWildWall(wall, dealer, f, q)
				all := append(append([]string{}, got...), i)
				if !reflect.DeepEqual(counts(all), counts(wall)) || len(got) != 135 {
					t.Fatal(dealer, one, two)
				}
			}
		}
	}
}
func TestMixedWinShapes(t *testing.T) {
	cases := []struct {
		h    []string
		en   []string
		want bool
	}{
		{[]string{"1m", "2m", "7z", "4p", "5p", "6p", "2s", "2s", "7z", "1z", "1z", "1z", "5z", "7z"}, nil, true},
		{[]string{"8m", "9m", "7z", "4p", "5p", "6p", "2s", "2s", "2s", "1z", "1z", "1z", "5z", "5z"}, nil, true},
		{[]string{"1m", "2m", "4m", "1p", "3p", "5p", "1s", "3s", "5s", "1z", "2z", "3z", "5z", "7z"}, nil, false},
		{[]string{"1m", "1m", "2m", "2m", "3p", "3p", "4p", "4p", "1z", "1z", "2z", "3z", "7z", "7z"}, []string{"七小对"}, true},
		{[]string{"1m", "9m", "1p", "9p", "1s", "9s", "1z", "2z", "3z", "4z", "5z", "6z", "7z", "7z"}, []string{"十三幺"}, true},
		{[]string{"1m", "2m", "7z", "5z", "7z"}, nil, true},
		{[]string{"7z", "7z"}, nil, true},
	}
	for _, c := range cases {
		if got := winningWild(c.h, c.en, "7z"); got != c.want {
			t.Fatalf("%v: got %v", c.h, got)
		}
	}
}
func TestMixedPengChiAndKongRestrictions(t *testing.T) {
	for _, kind := range []string{"peng", "chi"} {
		_, r, _ := table(t)
		r.Config.Wildcards = true
		r.Config.AllowChi = true
		r.WildTile = "7z"
		r.Phase = "running"
		r.Turn = 0
		r.wall = []string{"5m"}
		r.Players[0].Discards = []string{"3m"}
		r.Players[1].hand = []string{"3m", "2m", "7z", "9p"}
		r.ask(0, "3m", false)
		var choice Move
		for _, m := range r.options(1) {
			if m.Type == kind && len(m.Faces) > 0 && has(m.Tiles, "7z") {
				choice = m
				break
			}
		}
		if choice.Type == "" {
			t.Fatal("missing mixed option", kind)
		}
		forged := choice
		forged.Faces = []string{"9p", "9p"}
		if r.act(1, forged) == nil {
			t.Fatal("accepted forged representation")
		}
		for seat := range r.pending.options {
			r.pending.answers[seat] = Move{Type: "pass"}
		}
		r.pending.answers[1] = choice
		r.resolve()
		meld := r.Players[1].Melds[0]
		if count(meld.Tiles, "7z") != 1 || len(meld.Faces) != 3 || count(r.Players[1].hand, "7z") != 0 {
			t.Fatal(meld)
		}
		if kind == "peng" {
			r.Players[1].hand = append(r.Players[1].hand, "3m")
			for _, m := range r.options(1) {
				if m.Type == "add_kong" {
					t.Fatal("mixed peng promoted to kong")
				}
			}
		}
	}
	_, r, _ := table(t)
	r.WildTile = "7z"
	r.Phase = "running"
	r.Turn = 0
	r.wall = []string{"1m"}
	r.Players[0].hand = []string{"7z", "7z", "7z", "7z", "2m"}
	if err := r.act(0, Move{Type: "discard", Tile: "7z"}); err == nil {
		t.Fatal("discarded wild")
	}
	for _, m := range r.options(0) {
		if m.Type == "kong" {
			t.Fatal("wild concealed kong")
		}
	}
	r.Players[0].Discards = []string{"2m"}
	r.Players[1].hand = []string{"2m", "2m", "7z"}
	r.ask(0, "2m", false)
	for _, m := range r.options(1) {
		if m.Type == "kong" {
			t.Fatal("mixed open kong")
		}
	}
}
func TestWildDealPrivacyAndFullRound(t *testing.T) {
	h, r, tokens := table(t)
	r.Config.Wildcards = true
	r.Config.AllowChi = true
	for _, token := range tokens {
		mustAct(t, h, r, token, Move{Type: "ready"})
	}
	if len(r.wall) != 82 || r.Opening == nil || r.WildTile == "" || r.Indicator == "" {
		t.Fatal("missing opening")
	}
	conserve(t, r)
	for seat := 0; seat < 4; seat++ {
		v := r.view(seat)
		if v.WildTile != r.WildTile || v.Indicator != r.Indicator || (seat != r.Turn && v.DrawnTile != "") {
			t.Fatal("private draw leaked or missing public indicator")
		}
	}
	for step := 0; step < 600 && r.Phase != "settled"; step++ {
		if r.Phase == "running" {
			seat := r.Turn
			m := Move{Type: "discard", Tile: r.discardTile(seat)}
			for _, option := range r.options(seat) {
				if option.Type == "hu" {
					m = option
					break
				}
			}
			mustAct(t, h, r, tokens[seat], m)
		} else {
			opts := r.pending.options
			for seat, options := range opts {
				m := Move{Type: "pass"}
				for _, o := range options {
					if o.Type == "hu" || o.Type == "peng" || o.Type == "chi" {
						m = o
						break
					}
				}
				mustAct(t, h, r, tokens[seat], m)
			}
		}
		conserve(t, r)
	}
	if r.Phase != "settled" {
		t.Fatal("round never finished")
	}
	old := r.Opening
	for _, token := range tokens {
		mustAct(t, h, r, token, Move{Type: "ready"})
	}
	if r.Number != 2 || r.Opening == old {
		t.Fatal("opening not refreshed")
	}
	conserve(t, r)
}
func TestTimeoutSkipsWild(t *testing.T) {
	h, r, _ := table(t)
	r.WildTile = "7z"
	r.Phase = "running"
	r.Turn = 0
	r.Players[0].hand = []string{"1m", "7z"}
	r.wall = []string{"2m"}
	r.deadline = time.Now().Add(-time.Second)
	h.Tick()
	if !reflect.DeepEqual(r.Players[0].Discards, []string{"1m"}) {
		t.Fatal("timeout discarded wild", r.Players[0].Discards)
	}
}

func TestMixedBigHandMultipliers(t *testing.T) {
	_, r, _ := table(t)
	r.WildTile = "7z"
	r.Dealer = 3
	r.Config.BigHands = []string{"大三元"}
	r.Players[0].hand = []string{"5z", "5z", "7z", "6z", "6z", "6z", "7z", "7z", "7z", "1m", "2m", "3m", "2p", "2p"}
	// Four mixed 白 can represent 中 and the required 白 triplet.
	if !r.canWin(0, "") {
		t.Fatal("big hand not winning")
	}
	_, why := r.multiplier(0, -1)
	if !strings.Contains(why, "大三元") {
		t.Fatal(why)
	}
	r.Config.BigHands = []string{"清一色"}
	r.Players[0].hand = []string{"1m", "2m", "7z", "4m", "5m", "6m", "7m", "8m", "9m", "2m", "2m", "2m", "5m", "7z"}
	_, why = r.multiplier(0, -1)
	if !strings.Contains(why, "清一色") {
		t.Fatal(why)
	}
	r.Players[0].Melds = []Meld{{Kind: "peng", Tiles: []string{"1p", "1p", "7z"}, Faces: []string{"1p", "1p", "1p"}}}
	r.Players[0].hand = []string{"1m", "2m", "7z", "4m", "5m", "6m", "7m", "8m", "9m", "5m", "5m"}
	_, why = r.multiplier(0, -1)
	if strings.Contains(why, "清一色") {
		t.Fatal("represented meld ignored", why)
	}
}
