package daemon

import "testing"

func TestIsRouterCommand(t *testing.T) {
	if !IsRouterCommand([]string{"status"}) {
		t.Fatal("status")
	}
	if IsRouterCommand(nil) || IsRouterCommand([]string{"run"}) {
		t.Fatal("non-router")
	}
}
