package handler

import "testing"

func TestSemverNewer(t *testing.T) {
	tests := []struct {
		latest, current string
		want            bool
	}{
		// Basic comparisons
		{"1.0.1", "1.0.0", true},
		{"1.1.0", "1.0.0", true},
		{"2.0.0", "1.0.0", true},
		{"1.0.0", "1.0.0", false}, // same version
		{"1.0.0", "1.0.1", false}, // current is newer
		{"1.0.0", "2.0.0", false},

		// Multi-digit
		{"1.10.0", "1.9.0", true},
		{"1.0.10", "1.0.9", true},
		{"10.0.0", "9.0.0", true},

		// Pre-release comparisons per SemVer 2.0
		{"1.1.0-beta", "1.0.0", true},
		{"1.0.0", "1.0.0-beta", true}, // stable release ranks above pre-release
		{"1.0.0-rc.2", "1.0.0-rc.1", true},
		{"1.0.0-rc.1", "1.0.0-rc.2", false},

		// Build metadata ignored for precedence
		{"1.0.1+build", "1.0.0", true},
		{"1.0.1", "1.0.1+build", false},
		{"1.0.1+build2", "1.0.1+build1", false},

		// Edge cases
		{"", "1.0.0", false},
		{"1.0.0", "", false},
		{"1.0.0", "dev", false},
		{"", "", false},

		// Partial versions
		{"1.1", "1.0", true},
		{"1", "0", true},
	}

	for _, tt := range tests {
		got := semverNewer(tt.latest, tt.current)
		if got != tt.want {
			t.Errorf("semverNewer(%q, %q) = %v, want %v", tt.latest, tt.current, got, tt.want)
		}
	}
}

func TestStripV(t *testing.T) {
	tests := []struct{ in, want string }{
		{"v1.0.0", "1.0.0"},
		{"1.0.0", "1.0.0"},
		{"v", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := stripV(tt.in); got != tt.want {
			t.Errorf("stripV(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseSemver(t *testing.T) {
	tests := []struct {
		in   string
		want [3]int
	}{
		{"1.2.3", [3]int{1, 2, 3}},
		{"v1.2.3", [3]int{1, 2, 3}},
		{"1.2.3-beta.1", [3]int{1, 2, 3}},
		{"1.2.3+build456", [3]int{1, 2, 3}},
		{"1.2", [3]int{1, 2, 0}},
		{"1", [3]int{1, 0, 0}},
		{"invalid", [3]int{0, 0, 0}},
	}
	for _, tt := range tests {
		if got := parseSemver(tt.in); got != tt.want {
			t.Errorf("parseSemver(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
