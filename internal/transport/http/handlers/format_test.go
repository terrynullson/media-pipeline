package handlers

import "testing"

func TestHumanSize(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		input int64
		want  string
	}{
		{name: "bytes", input: 999, want: "999 B"},
		{name: "kilobytes", input: 1536, want: "1.5 KB"},
		{name: "megabytes", input: 5 * 1024 * 1024, want: "5.0 MB"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := HumanSize(tc.input); got != tc.want {
				t.Fatalf("HumanSize(%d) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
