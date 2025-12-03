package goatcounter

import "testing"

func TestFilterMatch(t *testing.T) {
	tests := []struct {
		query, path, title string
		event, want        bool
	}{
		{"", "/hello", "Hello, world", false, true},
		{"e", "/hello", "Hello, world", false, true},
		{"/h in:path", "/hello", "Hello, world", false, true},
		{"/h in:title", "/hello", "Hello, world", false, false},
		{", world   in:path", "/hello", "Hello, world", false, false},
		{", world   in:title", "/hello", "Hello, world", false, true},
		{"/h in:path at:end", "/hello", "Hello, world", false, false},
		{"/h in:path at:start", "/hello", "Hello, world", false, true},
		{"/h in:path at:start at:end", "/hello", "Hello, world", false, false},
		{"/hello in:path at:start at:end", "/hello", "Hello, world", false, true},

		{"HELLO", "/hello", "Hello, world", false, true},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			have := Filter{Query: tt.query}.Match(tt.path, tt.title, tt.event)
			if have != tt.want {
				t.Error(tt.query)
			}

			have = Filter{Query: tt.query + " :not"}.Match(tt.path, tt.title, tt.event)
			if have == tt.want {
				t.Error(":NOT →", tt.query)
			}
		})
	}
}
