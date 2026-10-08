package game

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestClientMsgDecode(t *testing.T) {
	var m ClientMsg
	if err := json.Unmarshal([]byte(`{"t":"move","x":1.2,"z":-3.4,"r":1.57}`), &m); err != nil {
		t.Fatal(err)
	}
	if want := (ClientMsg{T: "move", X: 1.2, Z: -3.4, R: 1.57}); m != want {
		t.Errorf("decoded %+v, want %+v", m, want)
	}
}

func TestSnapshotJSON(t *testing.T) {
	w := NewWorld(3)
	for i := 0; i < 3; i++ {
		if _, err := w.AddPlayer(); err != nil {
			t.Fatal(err)
		}
	}
	w.Players[2].Score = 4
	msg := Snapshot(w)
	if msg.T != "state" || len(msg.Players) != 3 || len(msg.Stars) != StarCount {
		t.Fatalf("bad snapshot %+v", msg)
	}
	for i, p := range msg.Players {
		if p.ID != i+1 {
			t.Errorf("players[%d].id = %d, want %d", i, p.ID, i+1)
		}
	}
	for i, s := range msg.Stars {
		if s.ID != w.Stars[i].ID {
			t.Errorf("stars[%d].id = %d, want %d", i, s.ID, w.Stars[i].ID)
		}
	}
	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"t":"state"`, `"players":`, `"stars":`, `"id":`, `"animal":"fox"`, `"x":`, `"z":`, `"r":`, `"score":4`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("snapshot JSON %s lacks %s", b, key)
		}
	}
}

func TestWelcomeIncludesID(t *testing.T) {
	b, err := json.Marshal(ServerMsg{T: "welcome", ID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"t":"welcome","id":1}` {
		t.Errorf("welcome = %s", b)
	}
}
