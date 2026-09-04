package automode

import (
	"testing"
	"time"

	"github.com/iamtew/t3b/internal/auth"
)

func TestShouldOp(t *testing.T) {
	tr := New(auth.New("owner!~o@h", []string{"admin!~a@h"}))
	if !tr.ShouldOp("#c", "owner", "owner!~o@h") {
		t.Fatal("owner should op")
	}
	if tr.ShouldOp("#c", "owner", "owner!~o@h") {
		t.Fatal("debounce should block immediate re-op")
	}
	if tr.ShouldOp("#c", "rando", "rando!~r@h") {
		t.Fatal("stranger must not op")
	}
	tr.last["#c\x00owner"] = time.Now().Add(-Debounce - time.Second)
	if !tr.ShouldOp("#c", "owner", "owner!~o@h") {
		t.Fatal("after debounce should allow again")
	}
}
