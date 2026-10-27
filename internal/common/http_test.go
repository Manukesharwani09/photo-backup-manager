package common

import "testing"

func TestFormatGB(t *testing.T) {
	cases := map[int64]string{-1: "unlimited", 0: "0 GB", 12970973361: "12.08 GB", 15032385536: "14 GB"}
	for in, want := range cases {
		if got := FormatGB(in); got != want {
			t.Errorf("FormatGB(%d) = %q, want %q", in, got, want)
		}
	}
}
