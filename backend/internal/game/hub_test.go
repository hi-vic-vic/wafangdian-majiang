package game

import "testing"

func TestFourPlayersMustReady(t *testing.T) {
	h := NewHub()
	r, e := h.CreateRoom("a", "甲", DefaultConfig())
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range []struct{ id, name string }{{"b", "乙"}, {"c", "丙"}, {"d", "丁"}} {
		if _, e = h.JoinRoom(r.Code, p.id, p.name); e != nil {
			t.Fatal(e)
		}
	}
	for _, id := range []string{"a", "b", "c"} {
		_, _ = h.Ready(r.Code, id, true)
	}
	v, _ := h.Get(r.Code)
	if v.Phase != "lobby" {
		t.Fatal("started too early")
	}
	v, e = h.Ready(r.Code, "d", true)
	if e != nil || v.Phase != "running" {
		t.Fatal("did not start")
	}
}
func TestOnlyTurnOwnerDiscards(t *testing.T) {
	h := NewHub()
	r, _ := h.CreateRoom("a", "甲", DefaultConfig())
	for _, p := range []struct{ id, name string }{{"b", "乙"}, {"c", "丙"}, {"d", "丁"}} {
		_, _ = h.JoinRoom(r.Code, p.id, p.name)
	}
	for _, id := range []string{"a", "b", "c", "d"} {
		_, _ = h.Ready(r.Code, id, true)
	}
	if _, e := h.Discard(r.Code, "b", "1m"); e == nil {
		t.Fatal("accepted invalid turn")
	}
	v, e := h.Discard(r.Code, "a", "1m")
	if e != nil || v.Turn != 1 {
		t.Fatal("did not advance")
	}
}
