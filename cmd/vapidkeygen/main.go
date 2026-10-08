// vapidkeygen prints a fresh Web Push VAPID key pair in the form the
// server expects (design D22): base64url, public key as the
// uncompressed P-256 point, private key as the raw 32-byte scalar.
package main

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
)

func main() {
	priv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "vapidkeygen:", err)
		os.Exit(1)
	}
	enc := base64.RawURLEncoding
	fmt.Println("VAPID_PUBLIC_KEY=" + enc.EncodeToString(priv.PublicKey().Bytes()))
	fmt.Println("VAPID_PRIVATE_KEY=" + enc.EncodeToString(priv.Bytes()))
}
