package crypto

import (
	"testing"
)

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef" // 32 chars
	c, err := New(key)
	if err != nil {
		t.Fatal(err)
	}
	plain := "sk-abcdef123456"
	enc, err := c.Encrypt(plain)
	if err != nil {
		t.Fatal(err)
	}
	if enc == plain {
		t.Error("ciphertext == plaintext")
	}
	dec, err := c.Decrypt(enc)
	if err != nil {
		t.Fatal(err)
	}
	if dec != plain {
		t.Errorf("got %q want %q", dec, plain)
	}
}

func TestMaskAPIKey(t *testing.T) {
	got := Mask("sk-abcdef123456")
	if got != "****3456" {
		t.Errorf("got %q", got)
	}
	if Mask("short") != "****" {
		t.Errorf("short key mask = %q", Mask("short"))
	}
}