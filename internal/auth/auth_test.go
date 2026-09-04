package auth

import "testing"

func TestMatchExactAndGlob(t *testing.T) {
	mask := "tew!~tew@home.example"

	cases := []struct {
		pattern string
		want    bool
	}{
		{"tew!~tew@home.example", true},
		{"TEW!~tew@home.example", true},
		{"*!~tew@home.example", true},
		{"tew!~tew@*.example", true},
		{"other!~tew@home.example", false},
		{"", false},
	}

	for _, tc := range cases {
		if got := Match(tc.pattern, mask); got != tc.want {
			t.Errorf("Match(%q, %q) = %v, want %v", tc.pattern, mask, got, tc.want)
		}
	}
}

func TestHostmasksRoles(t *testing.T) {
	h := New("owner!~o@host", []string{"admin!~a@host", "*!~mod@*.net"})

	if !h.IsOwner("owner!~o@host") {
		t.Fatal("expected owner match")
	}
	if h.IsAdmin("owner!~o@host") {
		t.Fatal("owner should not count as admin unless listed")
	}
	if !h.IsAdmin("admin!~a@host") {
		t.Fatal("expected admin match")
	}
	if !h.IsOwnerOrAdmin("mod!~mod@foo.net") {
		t.Fatal("expected glob admin via IsOwnerOrAdmin")
	}
}
