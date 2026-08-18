package storeedgemanager

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.15.8.1", "0.15.8", 1},
		{"0.15.8", "0.15.8", 0},
		{"0.15.7.1", "0.15.8", -1},
		{"v1.2.3", "1.2.2", 1},
	}
	for _, tc := range cases {
		got := compareVersions(tc.a, tc.b)
		if got != tc.want {
			t.Fatalf("compareVersions(%q,%q)=%d want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestSignatureMessageBindsReleaseMetadata(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	msg := signatureMessage("0.15.8.1", "linux-amd64", "worker", "abcd")
	sig := ed25519.Sign(priv, []byte(msg))
	if !ed25519.Verify(pub, []byte(msg), sig) {
		t.Fatal("valid signature rejected")
	}
	if ed25519.Verify(pub, []byte(signatureMessage("0.15.9", "linux-amd64", "worker", "abcd")), sig) {
		t.Fatal("signature must bind version")
	}
	if ed25519.Verify(pub, []byte(signatureMessage("0.15.8.1", "windows-amd64", "worker", "abcd")), sig) {
		t.Fatal("signature must bind platform")
	}
}
