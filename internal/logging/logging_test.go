package logging

import "testing"

func TestColorPolicy(t *testing.T) {
	tests := []struct {
		name, colors, noColor string
		force, disable        bool
	}{
		{name: "default", force: true},
		{name: "explicit on", colors: "true", force: true},
		{name: "explicit off", colors: "false", disable: true},
		{name: "no color standard", colors: "true", noColor: "1", disable: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			getenv := func(key string) string {
				if key == "LOG_COLORS" {
					return tc.colors
				}
				if key == "NO_COLOR" {
					return tc.noColor
				}
				return ""
			}
			force, disable := colorPolicy(getenv)
			if force != tc.force || disable != tc.disable {
				t.Fatalf("got force=%v disable=%v", force, disable)
			}
		})
	}
}
