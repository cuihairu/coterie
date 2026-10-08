package notification

// Tests for the hand-written Web Push primitives (D22): the encrypt
// path is checked against an independent receiver-side decryption (the
// RFC 8291 derivation run in reverse), the VAPID JWT against a fresh
// ECDSA verification of its claims.

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/crypto/hkdf"
)

// decrypt runs the receiver side of RFC 8291 aes128gcm: parse the
// record header, redo the HKDF chains with the client's private key and
// auth secret, open the single record, strip the delimiter.
func decrypt(t *testing.T, body []byte, priv *ecdh.PrivateKey, authSecret []byte) []byte {
	t.Helper()
	if len(body) < 21 {
		t.Fatalf("body too short: %d", len(body))
	}
	salt, rest := body[:16], body[16:]
	rs := int(big.NewInt(0).SetBytes(rest[:4]).Uint64())
	idlen := int(rest[4])
	keyid := rest[5 : 5+idlen]
	ct := rest[5+idlen:]
	if rs != len(body) {
		t.Fatalf("rs = %d, body = %d (want single record)", rs, len(body))
	}

	shared, err := priv.ECDH(mustPub(t, keyid))
	if err != nil {
		t.Fatalf("receiver ecdh: %v", err)
	}
	info := append([]byte("WebPush: info\x00"), priv.PublicKey().Bytes()...)
	info = append(info, keyid...)
	prkKey := hkdfRead(t, shared, authSecret, info, 32)
	ikm := hkdfRead(t, prkKey, salt, []byte("Content-Encoding: aes128gcm\x00"), 16)
	nonce := hkdfRead(t, prkKey, salt, []byte("Content-Encoding: nonce\x00"), 12)

	block, err := aes.NewCipher(ikm)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if pt[len(pt)-1] != 0x02 {
		t.Fatalf("record delimiter = %#x, want 0x02", pt[len(pt)-1])
	}
	return pt[:len(pt)-1]
}

func mustPub(t *testing.T, raw []byte) *ecdh.PublicKey {
	t.Helper()
	p, err := ecdh.P256().NewPublicKey(raw)
	if err != nil {
		t.Fatalf("bad public key: %v", err)
	}
	return p
}

func hkdfRead(t *testing.T, secret, salt, info []byte, n int) []byte {
	t.Helper()
	out := make([]byte, n)
	r := hkdf.New(sha256.New, secret, salt, info)
	if _, err := r.Read(out); err != nil {
		t.Fatal(err)
	}
	return out
}

func clientKeys(t *testing.T) (*ecdh.PrivateKey, string, string) {
	t.Helper()
	priv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := make([]byte, 16)
	if _, err := rand.Read(auth); err != nil {
		t.Fatal(err)
	}
	enc := base64.RawURLEncoding
	return priv, enc.EncodeToString(priv.PublicKey().Bytes()), enc.EncodeToString(auth)
}

func TestEncryptRoundTrip(t *testing.T) {
	priv, p256dh, auth := clientKeys(t)
	authSecret, err := base64.RawURLEncoding.DecodeString(auth)
	if err != nil {
		t.Fatal(err)
	}
	for _, payload := range []string{"hello", "", strings.Repeat("x", 3000)} {
		body, err := encrypt([]byte(payload), p256dh, auth)
		if err != nil {
			t.Fatalf("encrypt(%d bytes): %v", len(payload), err)
		}
		if got := string(decrypt(t, body, priv, authSecret)); got != payload {
			t.Fatalf("round trip: got %q, want %q", got, payload)
		}
	}
}

// Two encryptions of the same input must differ: fresh ephemeral key
// and salt every time.
func TestEncryptNotDeterministic(t *testing.T) {
	_, p256dh, auth := clientKeys(t)
	a, err := encrypt([]byte("same"), p256dh, auth)
	if err != nil {
		t.Fatal(err)
	}
	b, err := encrypt([]byte("same"), p256dh, auth)
	if err != nil {
		t.Fatal(err)
	}
	if string(a) == string(b) {
		t.Fatal("two encryptions of the same payload are identical")
	}
}

func TestEncryptRejectsBadKeys(t *testing.T) {
	if _, err := encrypt([]byte("x"), "!!not-base64!!", "YWJj"); err == nil {
		t.Fatal("bad p256dh accepted")
	}
	if _, err := encrypt([]byte("x"), base64.RawURLEncoding.EncodeToString(make([]byte, 65)), "!!"); err == nil {
		t.Fatal("bad auth accepted")
	}
	// A valid-length but all-zero point fails ECDH.
	if _, err := encrypt([]byte("x"), base64.RawURLEncoding.EncodeToString(make([]byte, 65)), "YWJj"); err == nil {
		t.Fatal("degenerate p256dh key accepted")
	}
}

func TestVapidAuthHeader(t *testing.T) {
	_, p256dh, _ := clientKeys(t)
	_ = p256dh
	privRaw := make([]byte, 32)
	if _, err := rand.Read(privRaw); err != nil {
		t.Fatal(err)
	}
	priv, err := ecdh.P256().NewPrivateKey(privRaw)
	if err != nil {
		t.Fatal(err)
	}
	pub65 := priv.PublicKey().Bytes()
	x, y := elliptic.Unmarshal(elliptic.P256(), pub65)
	keys := &vapidKeys{
		signer:   &ecdsa.PrivateKey{D: new(big.Int).SetBytes(privRaw), PublicKey: ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}},
		Public65: pub65,
		Subject:  "mailto:ops@example.com",
	}
	hdr, err := keys.authHeader("https://push.example.com/send/abc")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hdr, "vapid t=") || !strings.Contains(hdr, ", k=") {
		t.Fatalf("header shape: %q", hdr)
	}
	jwt := strings.TrimPrefix(hdr[:strings.Index(hdr, ",")], "vapid t=")
	parts := strings.SplitN(jwt, ".", 3)
	if len(parts) != 3 {
		t.Fatalf("jwt split: %q", hdr)
	}
	dec := base64.RawURLEncoding.DecodeString
	var header struct {
		Typ string `json:"typ"`
		Alg string `json:"alg"`
	}
	var claims struct {
		Aud string `json:"aud"`
		Exp int64  `json:"exp"`
		Sub string `json:"sub"`
	}
	hj, err := dec(parts[0])
	if err != nil || json.Unmarshal(hj, &header) != nil || header.Alg != "ES256" {
		t.Fatalf("jwt header: %q %v", parts[0], err)
	}
	cj, err := dec(parts[1])
	if err != nil || json.Unmarshal(cj, &claims) != nil {
		t.Fatalf("jwt claims: %q %v", parts[1], err)
	}
	u, _ := url.Parse("https://push.example.com/send/abc")
	if claims.Aud != u.Scheme+"://"+u.Host || claims.Sub != "mailto:ops@example.com" || claims.Exp == 0 {
		t.Fatalf("claims: %+v", claims)
	}
	// The k= parameter must be the uncompressed public point.
	kv := strings.TrimPrefix(hdr[strings.Index(hdr, ", ")+2:], "k=")
	if kb, err := dec(kv); err != nil || len(kb) != 65 || kb[0] != 4 || string(kb) != string(pub65) {
		t.Fatalf("k parameter: %v %d", err, len(kb))
	}
	// Verify the signature with the public key.
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	sig, err := dec(parts[2])
	if err != nil || len(sig) != 64 {
		t.Fatalf("signature: %v %d", err, len(sig))
	}
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:])
	if !ecdsa.Verify(&keys.signer.PublicKey, digest[:], r, s) {
		t.Fatal("jwt signature does not verify")
	}
}

func TestParseVapidKeys(t *testing.T) {
	if parseVapidKeys("", "", "") != nil {
		t.Fatal("empty config accepted")
	}
	if parseVapidKeys("abc", "def", "mailto:x") != nil {
		t.Fatal("garbage config accepted")
	}
	privRaw := make([]byte, 32)
	priv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	copy(privRaw, priv.Bytes())
	enc := base64.RawURLEncoding
	k := parseVapidKeys(enc.EncodeToString(priv.PublicKey().Bytes()), enc.EncodeToString(privRaw), "mailto:x")
	if k == nil {
		t.Fatal("valid config rejected")
	}
	// Truncated private scalar rejected.
	if parseVapidKeys(enc.EncodeToString(priv.PublicKey().Bytes()), enc.EncodeToString(privRaw[:16]), "mailto:x") != nil {
		t.Fatal("short private key accepted")
	}
}

func TestTrimPayload(t *testing.T) {
	full := []byte(`{"title":"t","body":"` + strings.Repeat("b", 5000) + `"}`)
	trimmed := string(trimPayload(full))
	if len(trimmed) > pushPayloadMax+64 { // escaping headroom
		t.Fatalf("trim left %d bytes", len(trimmed))
	}
	var p map[string]string
	if err := json.Unmarshal([]byte(trimmed), &p); err != nil {
		t.Fatalf("trimmed not json: %v", err)
	}
	if p["title"] != "t" {
		t.Fatalf("title lost: %v", p)
	}
	small := []byte(`{"title":"t","body":"tiny"}`)
	if string(trimPayload(small)) != string(small) {
		t.Fatal("small payload modified")
	}
}
