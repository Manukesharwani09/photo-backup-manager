package crypto

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func key(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		t.Fatal(err)
	}
	return k
}

func TestSealOpenRoundTrip(t *testing.T) {
	k := key(t)
	plain := []byte("1//refresh-token-value")
	sealed, err := Seal(k, plain)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, plain) {
		t.Fatal("sealed output contains the plaintext")
	}
	got, err := Open(k, sealed)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("got %q, want %q", got, plain)
	}
}

func TestSealUsesFreshNonce(t *testing.T) {
	k := key(t)
	a, _ := Seal(k, []byte("same"))
	b, _ := Seal(k, []byte("same"))
	if bytes.Equal(a, b) {
		t.Fatal("two seals of the same plaintext are identical")
	}
}

func TestOpenRejectsTamperedData(t *testing.T) {
	k := key(t)
	sealed, _ := Seal(k, []byte("secret"))
	sealed[len(sealed)-1] ^= 0xff
	if _, err := Open(k, sealed); err == nil {
		t.Fatal("tampered ciphertext was accepted")
	}
}

func TestOpenRejectsWrongKey(t *testing.T) {
	sealed, _ := Seal(key(t), []byte("secret"))
	if _, err := Open(key(t), sealed); err == nil {
		t.Fatal("wrong key was accepted")
	}
}

func TestOpenRejectsShortInput(t *testing.T) {
	if _, err := Open(key(t), []byte("short")); err == nil {
		t.Fatal("short input was accepted")
	}
}

func TestBadKeyLength(t *testing.T) {
	if _, err := Seal([]byte("too-short"), []byte("x")); err == nil {
		t.Fatal("invalid key length was accepted")
	}
}
