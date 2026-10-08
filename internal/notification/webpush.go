package notification

// Web Push outbound delivery (design D22, RFC 8291 + RFC 8292).
// Encryption and the VAPID JWT are hand-written on stdlib +
// x/crypto/hkdf — both are fully specified, so no third-party push
// library is pulled in.

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/crypto/hkdf"

	"github.com/cuihairu/coterie/internal/push"
)

// pushTTL tells the push service how long an undeliverable message may
// wait for the client (a week).
const pushTTL = "604800"

// vapidTTL bounds the JWT lifetime well under the RFC's 24h ceiling.
const vapidTTL = 12 * time.Hour

// pushPayloadMax is the payload budget common push services accept.
const pushPayloadMax = 4096

// vapidKeys carries the server's VAPID identity. Both parts arrive
// base64url: the public key as the uncompressed P-256 point (65 bytes,
// the format browser push tooling generates), the private key as the
// raw 32-byte scalar.
type vapidKeys struct {
	signer   *ecdsa.PrivateKey
	Public65 []byte
	Subject  string // mailto: contact sent as the JWT sub claim
}

// parseVapidKeys decodes the configured pair; nil means the channel is
// unconfigured (any part missing or malformed).
func parseVapidKeys(publicB64, privateB64, subject string) *vapidKeys {
	if publicB64 == "" || privateB64 == "" || subject == "" {
		return nil
	}
	pub, err := base64.RawURLEncoding.DecodeString(publicB64)
	if err != nil || len(pub) != 65 || pub[0] != 4 {
		return nil
	}
	priv, err := base64.RawURLEncoding.DecodeString(privateB64)
	if err != nil || len(priv) != 32 {
		return nil
	}
	d := new(big.Int).SetBytes(priv)
	x, y := elliptic.Unmarshal(elliptic.P256(), pub)
	if x == nil {
		return nil
	}
	return &vapidKeys{
		signer:   &ecdsa.PrivateKey{D: d, PublicKey: ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}},
		Public65: pub,
		Subject:  subject,
	}
}

// b64url is unpadded base64url, the encoding every Web Push string uses.
var b64url = base64.RawURLEncoding

// encrypt seals the payload for one subscription's key pair under RFC
// 8291 (aes128gcm content coding, single record): an ephemeral P-256
// ECDH with the client key feeds HKDF chains bound to the client's
// auth secret; the record header carries the salt and the ephemeral
// public key the receiver needs.
func encrypt(payload []byte, p256dh, auth string) ([]byte, error) {
	authSecret, err := b64url.DecodeString(auth)
	if err != nil {
		return nil, fmt.Errorf("decode auth key: %w", err)
	}
	uaPubRaw, err := b64url.DecodeString(p256dh)
	if err != nil {
		return nil, fmt.Errorf("decode p256dh key: %w", err)
	}
	uaPub, err := ecdh.P256().NewPublicKey(uaPubRaw)
	if err != nil {
		return nil, fmt.Errorf("bad p256dh key: %w", err)
	}
	asPriv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	shared, err := asPriv.ECDH(uaPub)
	if err != nil {
		return nil, err
	}
	asPub := asPriv.PublicKey().Bytes()

	// PRK_key = HKDF(ikm=ecdh, salt=auth_secret,
	//                info="WebPush: info" NUL ua_pub as_pub)
	info := append([]byte("WebPush: info\x00"), uaPubRaw...)
	info = append(info, asPub...)
	prkKey := hkdfExtract(shared, authSecret, info, 32)

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	// CEK and nonce chain from PRK_key with the record salt as HKDF
	// salt (the auth secret is only the salt of the first chain).
	ikm := hkdfExtract(prkKey, salt, []byte("Content-Encoding: aes128gcm\x00"), 16)
	nonce := hkdfExtract(prkKey, salt, []byte("Content-Encoding: nonce\x00"), 12)
	// Record framing: payload + the 0x02 delimiter (no padding bytes in
	// the final record), one AES-GCM tag, one record total.
	pt := make([]byte, 0, len(payload)+1)
	pt = append(pt, payload...)
	pt = append(pt, 0x02)

	block, err := aes.NewCipher(ikm)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ct := gcm.Seal(nil, nonce, pt, nil)

	header := make([]byte, 0, 21+len(asPub)) // salt+rs+idlen+keyid
	header = append(header, salt...)
	var rs [4]byte
	// rs covers the whole body: header + record plaintext + GCM tag.
	binary.BigEndian.PutUint32(rs[:], uint32(len(header)+4+1+len(asPub)+len(pt)+16))
	header = append(header, rs[:]...)
	header = append(header, byte(len(asPub)))
	header = append(header, asPub...)
	return append(header, ct...), nil
}

// hkdfExtract reads length bytes from an HKDF chain.
func hkdfExtract(secret, salt, info []byte, length int) []byte {
	out := make([]byte, length)
	r := hkdf.New(sha256.New, secret, salt, info)
	if _, err := io.ReadFull(r, out); err != nil {
		panic("hkdf: " + err.Error()) // cannot happen for these lengths
	}
	return out
}

// vapidAuthHeader builds the RFC 8292 Authorization value: an ES256 JWT
// audited to the endpoint's origin.
func (k *vapidKeys) authHeader(endpoint string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("bad push endpoint %q", endpoint)
	}
	header, err := json.Marshal(map[string]string{"typ": "JWT", "alg": "ES256"})
	if err != nil {
		return "", err
	}
	claims, err := json.Marshal(map[string]any{
		"aud": u.Scheme + "://" + u.Host,
		"exp": time.Now().Add(vapidTTL).Unix(),
		"sub": k.Subject,
	})
	if err != nil {
		return "", err
	}
	signingInput := b64url.EncodeToString(header) + "." + b64url.EncodeToString(claims)
	digest := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, k.signer, digest[:])
	if err != nil {
		return "", err
	}
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return "vapid t=" + signingInput + "." + b64url.EncodeToString(sig) +
		", k=" + b64url.EncodeToString(k.Public65), nil
}

// PushChannel delivers notifications to a user's registered browser
// push endpoints (D22). Each payload is encrypted per endpoint; 404/410
// from the push service prunes the registration. Best-effort like every
// channel: failures are logged, never fatal.
type PushChannel struct {
	store   *push.Store
	keys    *vapidKeys
	client  *http.Client
	timeout time.Duration
}

// NewPushChannel builds the channel; any missing VAPID part leaves it
// disabled (Enabled false), mirroring the email/webhook convention.
func NewPushChannel(store *push.Store, publicB64, privateB64, subject string) *PushChannel {
	return &PushChannel{
		store:   store,
		keys:    parseVapidKeys(publicB64, privateB64, subject),
		timeout: 10 * time.Second,
	}
}

// Name identifies the channel in delivery logs.
func (c *PushChannel) Name() string { return "push" }

// Enabled reports whether configuration is present.
func (c *PushChannel) Enabled() bool { return c.store != nil && c.keys != nil }

// Deliver pushes the notification to every endpoint the user has
// registered, pruning the ones the push service reports gone.
func (c *PushChannel) Deliver(ctx context.Context, n *Notification) error {
	subs, err := c.store.ListByUser(ctx, n.UserID)
	if err != nil {
		return err
	}
	if len(subs) == 0 {
		return nil
	}
	client := c.client
	if client == nil {
		client = &http.Client{Timeout: c.timeout}
	}
	var gone []string
	for _, sub := range subs {
		status, err := c.pushOne(ctx, client, &sub, n)
		if err != nil {
			// Endpoint-level failures are quiet best-effort; the
			// registration may recover on a later notification.
			_ = err
			continue
		}
		if status == http.StatusNotFound || status == http.StatusGone {
			gone = append(gone, sub.Endpoint)
		}
	}
	return c.store.DeleteByEndpoints(ctx, gone)
}

// pushOne sends one encrypted payload and returns the push service's
// status; the caller prunes 404/410 endpoints.
func (c *PushChannel) pushOne(ctx context.Context, client *http.Client, sub *push.Subscription, n *Notification) (int, error) {
	authz, err := c.keys.authHeader(sub.Endpoint)
	if err != nil {
		return 0, err
	}
	payload, err := json.Marshal(pushPayload(n))
	if err != nil {
		return 0, err
	}
	sealed, err := encrypt(trimPayload(payload), sub.P256dh, sub.Auth)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.Endpoint, bytes.NewReader(sealed))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("TTL", pushTTL)
	req.Header.Set("Urgency", "normal")
	req.Header.Set("Authorization", authz)
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 &&
		resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusGone {
		return resp.StatusCode, fmt.Errorf("push service status %d", resp.StatusCode)
	}
	return resp.StatusCode, nil
}

// pushPayload is what rides to the browser: the notification's display
// fields only — the push service is a third party, so no ledger detail.
func pushPayload(n *Notification) map[string]string {
	return map[string]string{"title": n.Title, "body": n.Body}
}

// trimPayload keeps the JSON within the service payload budget by
// cutting the body first; titles are short by construction.
func trimPayload(payload []byte) []byte {
	if len(payload) <= pushPayloadMax {
		return payload
	}
	var p struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return payload
	}
	over := len(payload) - pushPayloadMax
	if keep := len(p.Body) - over; keep > 0 {
		p.Body = strings.TrimSpace(p.Body[:keep]) + "…"
	} else {
		p.Body = ""
	}
	out, err := json.Marshal(p)
	if err != nil {
		return payload
	}
	return out
}
