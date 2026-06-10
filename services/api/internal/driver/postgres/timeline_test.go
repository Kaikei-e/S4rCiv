package postgres

import "testing"

// escapeLike must neutralize every LIKE/ILIKE metacharacter (\ % _) so the
// keyword the handler passes down matches literally under the queries' ESCAPE '\'.
// Backslash is escaped first by the replacer construction, so already-"escaped"
// input is itself treated as literal text (no double-unescape).
func TestEscapeLike(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"予算", "予算"},
		{"100%", `100\%`},
		{"a_b", `a\_b`},
		{`a\b`, `a\\b`},
		{`\%_`, `\\\%\_`},
		{"令和%年_月", `令和\%年\_月`},
	}
	for _, tc := range cases {
		if got := escapeLike(tc.in); got != tc.want {
			t.Errorf("escapeLike(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
