package config

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var validKey = base64.StdEncoding.EncodeToString(make([]byte, 32))

func clearEnv(t *testing.T) {
	for _, k := range []string{"GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "GOOGLE_REDIRECT_URL", "TOKEN_ENC_KEY", "DATABASE_URL", "LISTEN_ADDR"} {
		t.Setenv(k, "") // registers restore on cleanup
		os.Unsetenv(k)
	}
}

func writeEnv(t *testing.T, body string) string {
	p := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadFromDotEnv(t *testing.T) {
	clearEnv(t)
	p := writeEnv(t, "# comment\n\nGOOGLE_CLIENT_ID=id.apps\nGOOGLE_CLIENT_SECRET = sec \nTOKEN_ENC_KEY="+validKey+"\nnot-a-pair\n")
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GoogleClientID != "id.apps" || cfg.GoogleClientSecret != "sec" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if len(cfg.TokenKey) != 32 {
		t.Fatalf("key length %d", len(cfg.TokenKey))
	}
	if cfg.ListenAddr != "localhost:8080" || !strings.Contains(cfg.DatabaseURL, "photobackup") {
		t.Fatalf("defaults not applied: %+v", cfg)
	}
}

// Base64 keys end in "=" — the value must keep everything after the first "=".
func TestValueContainingEquals(t *testing.T) {
	clearEnv(t)
	k := base64.StdEncoding.EncodeToString(make([]byte, 32)) // ends with "="
	if !strings.HasSuffix(k, "=") {
		t.Skip("key has no padding")
	}
	p := writeEnv(t, "GOOGLE_CLIENT_ID=a\nGOOGLE_CLIENT_SECRET=b\nTOKEN_ENC_KEY="+k+"\n")
	if _, err := Load(p); err != nil {
		t.Fatalf("padded key rejected: %v", err)
	}
}

func TestRealEnvWinsOverDotEnv(t *testing.T) {
	clearEnv(t)
	t.Setenv("GOOGLE_CLIENT_ID", "from-env")
	p := writeEnv(t, "GOOGLE_CLIENT_ID=from-file\nGOOGLE_CLIENT_SECRET=b\nTOKEN_ENC_KEY="+validKey+"\n")
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GoogleClientID != "from-env" {
		t.Fatalf("got %q", cfg.GoogleClientID)
	}
}

func TestMissingFileIsFineWhenEnvSet(t *testing.T) {
	clearEnv(t)
	t.Setenv("GOOGLE_CLIENT_ID", "a")
	t.Setenv("GOOGLE_CLIENT_SECRET", "b")
	t.Setenv("TOKEN_ENC_KEY", validKey)
	if _, err := Load(filepath.Join(t.TempDir(), "nope.env")); err != nil {
		t.Fatal(err)
	}
}

func TestErrors(t *testing.T) {
	cases := map[string]string{
		"missing client": "TOKEN_ENC_KEY=" + validKey + "\n",
		"missing key":    "GOOGLE_CLIENT_ID=a\nGOOGLE_CLIENT_SECRET=b\n",
		"short key":      "GOOGLE_CLIENT_ID=a\nGOOGLE_CLIENT_SECRET=b\nTOKEN_ENC_KEY=" + base64.StdEncoding.EncodeToString(make([]byte, 16)) + "\n",
		"not base64 key": "GOOGLE_CLIENT_ID=a\nGOOGLE_CLIENT_SECRET=b\nTOKEN_ENC_KEY=%%%\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			clearEnv(t)
			if _, err := Load(writeEnv(t, body)); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}
