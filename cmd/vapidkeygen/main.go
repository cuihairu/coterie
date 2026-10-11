// vapidkeygen prints a fresh Web Push VAPID key pair in the form the
// server expects (design D22): base64url, public key as the
// uncompressed P-256 point, private key as the raw 32-byte scalar.
package main

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
)

func run(w io.Writer) error {
	priv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	enc := base64.RawURLEncoding
	fmt.Fprintln(w, "VAPID_PUBLIC_KEY="+enc.EncodeToString(priv.PublicKey().Bytes()))
	fmt.Fprintln(w, "VAPID_PRIVATE_KEY="+enc.EncodeToString(priv.Bytes()))
	return nil
}

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "vapidkeygen:", err)
		os.Exit(1)
	}
}
