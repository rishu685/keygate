package main

import "testing"

func TestStripHTMLTags(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"Alice", "Alice"},
		{"<b>Alice</b>", "Alice"},
		{"<i><b>Bold Italic</b></i>", "Bold Italic"},
		{"<script>alert(1)</script>", "alert(1)"},
		{"<scr<script>ipt>nested</script>", "nested"},
		{"<a href=\"https://example.com\">Link</a>", "Link"},
		{"Alice <!-- comment --> Bob", "Alice  Bob"},

		// Does not truncate on unclosed or relational '<'
		{"Alice < Bob", "Alice < Bob"},
		{"Alex <3", "Alex <3"},
		{"Score < 5", "Score < 5"},
		{"x < 5 and y > 2", "x < 5 and y > 2"},
		{"John <", "John <"},
		{"", ""},
	}

	for _, tt := range tests {
		if got := stripHTMLTags(tt.in); got != tt.want {
			t.Errorf("stripHTMLTags(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
