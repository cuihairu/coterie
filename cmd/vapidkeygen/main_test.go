package main

// The generated pair must satisfy the server-side contract
// (notification.parseVapidKeys): a 65-byte uncompressed P-256 point, a
// 32-byte raw scalar, and the public point derived from the scalar
// matching the printed public key — a mismatched pair is a config
// error on the server, so the generator must never print one.

import (
	"bytes"
	"crypto/ecdh"
	"encoding/base64"
	"strings"
	"testing"
)

func TestRunPrintsUsablePair(t *testing.T) {
	var out bytes.Buffer
	if err := run(&out); err != nil {
		t.Fatal(err)
	}
	var publicB64, privateB64 string
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		v, ok := strings.CutPrefix(line, "VAPID_PUBLIC_KEY=")
		if ok {
			publicB64 = v
			continue
		}
		if v, ok := strings.CutPrefix(line, "VAPID_PRIVATE_KEY="); ok {
			privateB64 = v
		}
	}
	if publicB64 == "" || privateB64 == "" {
		t.Fatalf("missing key lines: %q", out.String())
	}

	dec := base64.RawURLEncoding.DecodeString
	pub, err := dec(publicB64)
	if err != nil || len(pub) != 65 || pub[0] != 4 {
		t.Fatalf("public key: %q decoded %d bytes, err %v", publicB64, len(pub), err)
	}
	privRaw, err := dec(privateB64)
	if err != nil || len(privRaw) != 32 {
		t.Fatalf("private key: %q decoded %d bytes, err %v", privateB64, len(privRaw), err)
	}

	priv, err := ecdh.P256().NewPrivateKey(privRaw)
	if err != nil {
		t.Fatalf("scalar rejected: %v", err)
	}
	if !bytes.Equal(priv.PublicKey().Bytes(), pub) {
		t.Fatal("printed public key does not match the printed private key")
	}
}
