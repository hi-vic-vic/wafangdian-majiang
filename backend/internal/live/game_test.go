package live

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func table(t *testing.T) (*Hub, *Room, []string) {
	t.Helper()
	h := New()
	v, token, e := h.Create("A", Config{BaseScore: 1})
	if e != nil {
		t.Fatal(e)
	}
	tokens := []string{token}
	for _, name := range []string{"B", "C", "D"} {
		_, x, e := h.Join(v.Code, name)
		if e != nil {
			t.Fatal(e)
		}
		tokens = append(tokens, x)
	}
	return h, h.Rooms[v.Code], tokens
}
func drawnTile(v View) string {
	field := reflect.ValueOf(v).FieldByName("DrawnTile")
	if !field.IsValid() {
		return ""
	}
	return field.String()
}
func mustAct(t *testing.T, h *Hub, r *Room, token string, m Move) View {
	t.Helper()
	v, e := h.Act(r.Code, token, m)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func startTable(t *testing.T) (*Hub, *Room, []string) {
	h, r, ts := table(t)
	for _, x := range ts {
		mustAct(t, h, r, x, Move{Type: "ready"})
	}
	return h, r, ts
}
func conserve(t *testing.T, r *Room) {
	t.Helper()
	a := append([]string{}, r.wall...)
	if r.Indicator != "" {
		a = append(a, r.Indicator)
	}
	for _, p := range r.Players {
		a = append(a, p.hand...)
		a = append(a, p.Discards...)
		for _, m := range p.Melds {
			a = append(a, m.Tiles...)
		}
	}
	if len(a) != 136 {
		t.Fatalf("tile conservation: %d", len(a))
	}
	for _, n := range counts(a) {
		if n != 4 {
			t.Fatal("tile multiplicity changed")
		}
	}
}
func TestDealPrivacyAndOwnership(t *testing.T) {
	h, r, ts := table(t)
	for _, x := range ts[:3] {
		mustAct(t, h, r, x, Move{Type: "ready"})
	}
	if r.Phase != "lobby" {
		t.Fatal("early start")
	}
	mustAct(t, h, r, ts[3], Move{Type: "ready"})
	conserve(t, r)
	if len(r.wall) != 83 || len(r.Players[r.Dealer].hand) != 14 {
		t.Fatal("bad deal")
	}
	if _, e := h.Act(r.Code, ts[(r.Turn+1)%4], Move{Type: "discard", Tile: r.Players[r.Turn].hand[0]}); e == nil {
		t.Fatal("wrong turn accepted")
	}
	if _, e := h.Act(r.Code, ts[r.Turn], Move{Type: "discard", Tile: "invalid"}); e == nil {
		t.Fatal("invalid tile accepted")
	}
	if _, e := h.Snapshot(r.Code, "invalid"); e == nil {
		t.Fatal("unauthenticated read")
	}
	v, _ := h.Snapshot(r.Code, ts[0])
	b, _ := json.Marshal(v)
	for _, x := range ts {
		if strings.Contains(string(b), x) {
			t.Fatal("token leak")
		}
	}
	if strings.Contains(string(b), "\"hand\"") && len(v.Hand) != len(r.Players[0].hand) {
		t.Fatal("private hand mismatch")
	}
}
func TestDrawerSeesNewTileButOtherSeatsDoNot(t *testing.T) {
	h, r, tokens := startTable(t)
	dealer := r.Turn
	view, err := h.Snapshot(r.Code, tokens[dealer])
	if err != nil {
		t.Fatal(err)
	}
	if drawn := drawnTile(view); drawn == "" || !has(view.Hand, drawn) {
		t.Fatalf("drawer should receive the drawn tile, got %q", drawn)
	}
	if _, err := h.Act(r.Code, tokens[dealer], Move{Type: "discard", Tile: view.Hand[0]}); err != nil {
		t.Fatal(err)
	}
	for r.Phase == "react" {
		for seat := range r.pending.options {
			mustAct(t, h, r, tokens[seat], Move{Type: "pass"})
		}
	}
	drawer := (dealer + 1) % 4
	if r.Turn != drawer || len(r.Players[drawer].hand) != 14 {
		t.Fatal("next player should draw before being allowed to discard")
	}
	view, err = h.Snapshot(r.Code, tokens[drawer])
	if err != nil {
		t.Fatal(err)
	}
	if drawn := drawnTile(view); drawn == "" || !has(view.Hand, drawn) {
		t.Fatalf("next drawer should receive the drawn tile, got %q", drawn)
	}
	other, err := h.Snapshot(r.Code, tokens[(drawer+1)%4])
	if err != nil {
		t.Fatal(err)
	}
	if drawn := drawnTile(other); drawn != "" {
		t.Fatalf("other seat received private drawn tile %q", drawn)
	}
}
func TestFullGamesConserveAndTerminate(t *testing.T) {
	for trial := 0; trial < 30; trial++ {
		h, r, ts := startTable(t)
		for n := 0; n < 400 && r.Phase != "settled"; n++ {
			conserve(t, r)
			if r.Phase == "running" {
				s := r.Turn
				opts := r.options(s)
				m := Move{Type: "discard", Tile: r.Players[s].hand[len(r.Players[s].hand)-1]}
				if len(opts) > 0 {
					m = opts[0]
				}
				mustAct(t, h, r, ts[s], m)
			} else {
				p := r.pending
				for s, opts := range p.options {
					mustAct(t, h, r, ts[s], opts[0])
				}
			}
		}
		if r.Phase != "settled" {
			t.Fatal("did not terminate")
		}
		conserve(t, r)
		sum := 0
		for _, p := range r.Players {
			sum += p.Score
		}
		if sum != 0 {
			t.Fatal("non-zero-sum")
		}
	}
}
func TestHonorsAndSpecialWins(t *testing.T) {
	good := strings.Fields("1m 2m 3m 2p 3p 4p 5s 6s 7s 1z 1z 1z 5z 5z")
	if !winning(good, nil) {
		t.Fatal("valid win rejected")
	}
	good[10] = "2z"
	good[11] = "3z"
	if winning(good, nil) {
		t.Fatal("honor sequence accepted")
	}
	pairs := strings.Fields("1m 1m 1m 1m 2m 2m 3p 3p 4p 4p 1z 1z 7z 7z")
	if !winning(pairs, []string{"七小对"}) {
		t.Fatal("quad as pairs")
	}
}
func TestHuPriorityAndNearestSeat(t *testing.T) {
	_, r, _ := startTable(t)
	wait := strings.Fields("1m 2m 3m 2p 3p 4p 5s 6s 7s 1z 1z 1z 5z")
	for _, p := range r.Players {
		p.hand = append([]string{}, wait...)
	}
	r.Players[0].Discards = []string{"5z"}
	r.ask(0, "5z", false)
	r.pending.answers[3] = Move{Type: "hu"}
	r.pending.answers[1] = Move{Type: "hu"}
	r.pending.answers[2] = Move{Type: "pass"}
	r.resolve()
	if r.Bill.Winner != 1 {
		t.Fatal("nearest winner did not win")
	}
}
func TestRobKongNoKongPayment(t *testing.T) {
	h, r, ts := startTable(t)
	r.Turn = 0
	r.Players[0].Melds = []Meld{{Kind: "peng", Tiles: []string{"5z", "5z", "5z"}}}
	r.Players[0].hand = []string{"5z"}
	r.Players[1].hand = strings.Fields("1m 2m 3m 2p 3p 4p 5s 6s 7s 1z 1z 1z 5z")
	r.Players[2].hand = []string{}
	r.Players[3].hand = []string{}
	mustAct(t, h, r, ts[0], Move{Type: "add_kong", Tile: "5z"})
	mustAct(t, h, r, ts[1], Move{Type: "hu"})
	if r.gangs[0] != 0 || r.Players[0].Melds[0].Kind != "peng" || r.Bill.Winner != 1 {
		t.Fatal("robbed kong finalized")
	}
}
func TestTimeoutDrawAndKongAccounting(t *testing.T) {
	_, r, _ := startTable(t)
	r.gangs[0] = 10
	r.finish(-1, -1)
	for _, p := range r.Players {
		if p.Score != 0 {
			t.Fatal("draw charged kong")
		}
	}
	r.start()
	r.gangs[0] = 10
	r.Dealer = 1
	r.finish(1, -1)
	if r.Bill.Delta != [4]int{26, 2, -14, -14} {
		t.Fatalf("bad ledger %v", r.Bill.Delta)
	}
}
func TestTimeoutAndRejoin(t *testing.T) {
	h, r, ts := startTable(t)
	r.deadline = time.Now().Add(-time.Second)
	if len(h.Tick()) != 1 {
		t.Fatal("timeout failed")
	}
	v, _ := h.Snapshot(r.Code, ts[0])
	h.Connect(r.Code, ts[0], false)
	h.Connect(r.Code, ts[0], true)
	v2, _ := h.Snapshot(r.Code, ts[0])
	if strings.Join(v.Hand, ",") != strings.Join(v2.Hand, ",") {
		t.Fatal("reconnect changed hand")
	}
}
