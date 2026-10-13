package live

import (
	"context"
	"encoding/json"
	"github.com/coder/websocket"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFourSocketsDealAndReconnect(t *testing.T) {
	for _, wild := range []bool{false, true} {
		name := "without-mixed"
		if wild {
			name = "with-mixed"
		}
		t.Run(name, func(t *testing.T) { testFourSocketsDealAndReconnect(t, wild) })
	}
}
func testFourSocketsDealAndReconnect(t *testing.T, wild bool) {
	s := NewServer()
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	v, token, e := s.Hub.Create("A", Config{BaseScore: 1, Wildcards: wild})
	if e != nil {
		t.Fatal(e)
	}
	tokens := []string{token}
	for _, n := range []string{"B", "C", "D"} {
		_, x, e := s.Hub.Join(v.Code, n)
		if e != nil {
			t.Fatal(e)
		}
		tokens = append(tokens, x)
	}
	dial := func(tok string) *websocket.Conn {
		c, _, e := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/v1/rooms/"+v.Code+"/ws?token="+tok, nil)
		if e != nil {
			t.Fatal(e)
		}
		return c
	}
	var cs [4]*websocket.Conn
	for i := range cs {
		cs[i] = dial(tokens[i])
		defer cs[i].CloseNow()
	}
	read := func(c *websocket.Conn, min int) View {
		for {
			_, b, e := c.Read(ctx)
			if e != nil {
				t.Fatal(e)
			}
			var env struct {
				Type    string `json:"type"`
				Room    View   `json:"room"`
				Message string `json:"message"`
			}
			if e = json.Unmarshal(b, &env); e != nil {
				t.Fatal(e)
			}
			if env.Type == "error" {
				t.Fatal(env.Message)
			}
			if env.Room.Revision >= min {
				return env.Room
			}
		}
	}
	for i, c := range cs {
		read(c, 0)
		cur, _ := s.Hub.Snapshot(v.Code, tokens[i])
		b, _ := json.Marshal(map[string]any{"type": "ready", "revision": cur.Revision})
		if e = c.Write(ctx, websocket.MessageText, b); e != nil {
			t.Fatal(e)
		}
		read(c, cur.Revision+1)
	}
	state, _ := s.Hub.Snapshot(v.Code, tokens[0])
	s.Broadcast(v.Code)
	for i, c := range cs {
		got := read(c, state.Revision)
		if wild && (got.WildTile != state.WildTile || got.Indicator != state.Indicator || got.Opening == nil) {
			t.Fatal("missing shared mixed opening")
		}
		if got.Seat != i {
			t.Fatal("wrong private recipient")
		}
		want := 13
		if got.Seat == got.Dealer {
			want = 14
		}
		if len(got.Hand) != want {
			t.Fatal("bad private deal")
		}
	}
	before, _ := s.Hub.Snapshot(v.Code, tokens[0])
	cs[0].CloseNow()
	again := dial(tokens[0])
	defer again.CloseNow()
	after := read(again, before.Revision)
	if strings.Join(before.Hand, ",") != strings.Join(after.Hand, ",") {
		t.Fatal("reconnect lost hand")
	}
}
