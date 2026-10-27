package logger

import (
	"context"
	"testing"
)

func TestMaskEmail(t *testing.T) {
	cases := map[string]string{
		"mkesharwani125@gmail.com": "mke***@gmail.com",
		"ab@x.in":                  "ab***@x.in",
		"not-an-email":             "***",
	}
	for in, want := range cases {
		if got := MaskEmail(in); got != want {
			t.Errorf("MaskEmail(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFromContext(t *testing.T) {
	if FromContext(context.Background()) != GetLogger() {
		t.Fatal("plain context should use the process logger")
	}
	if FromContext(WithCorrelationID(context.Background(), "abc")) == GetLogger() {
		t.Fatal("correlation context should carry its own logger")
	}
}
