package world

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Errors of a token check (JWKS.Verify).
var (
	ErrTokenMalformed = errors.New("token: malformed")
	ErrTokenSignature = errors.New("token: bad signature")
	ErrTokenExpired   = errors.New("token: expired or not yet valid")
	ErrTokenClaims    = errors.New("token: issuer or audience not accepted")
	ErrTokenKey       = errors.New("token: no key for it")
)

// Claims are a verified token's claims.
type Claims struct {
	Subject  string         `json:"sub"`
	Issuer   string         `json:"iss"`
	Audience []string       `json:"-"`
	Expires  time.Time      `json:"-"` // zero: none
	All      map[string]any `json:"-"` // every claim as decoded
}

// JWKS verifies signed session tokens (JWT, compact form) against the keys
// a server publishes at a JWKS address, or in a JWKS file (#792): the MyCrew API issues a
// player's token for the director, GET /v1/traffic/jwks its keys. RS256,
// RS384, RS512, PS256, ES256, ES384 and EdDSA (Ed25519); exp and nbf with
// a minute of leeway, iss and aud when set. Keys are fetched on first use,
// again every hour, and at once (at most every 30 s) for an unknown kid.
type JWKS struct {
	URL string
	// Issuer and Audience: the token's iss must be Issuer, and its aud
	// include Audience ("": not checked).
	Issuer, Audience string
	// Client fetches the keys (nil: a 10 s timeout client).
	Client *http.Client

	mu      sync.Mutex
	keys    map[string]crypto.PublicKey
	fetched time.Time
	tried   time.Time        // the last fetch, failed or not: at most one each jwksMinFetch (#58)
	now     func() time.Time // tests
}

// NewJWKS checks tokens against the keys at url.
func NewJWKS(url, issuer, audience string) *JWKS {
	return &JWKS{URL: url, Issuer: issuer, Audience: audience}
}

const (
	jwksRefresh  = time.Hour
	jwksMinFetch = 30 * time.Second
	tokenLeeway  = time.Minute
)

func (j *JWKS) clock() time.Time {
	if j.now != nil {
		return j.now()
	}
	return time.Now()
}

// Verify checks token's signature, times and issuer and audience, and
// returns its claims.
func (j *JWKS) Verify(ctx context.Context, token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, ErrTokenMalformed
	}
	// Scoped to this service: a key signs tokens for others too (#59).
	if j.Issuer == "" && j.Audience == "" {
		return Claims{}, fmt.Errorf("%w: neither issuer nor audience is set to check", ErrTokenClaims)
	}
	var head struct {
		Alg  string   `json:"alg"`
		Kid  string   `json:"kid"`
		Crit []string `json:"crit"`
	}
	if err := decodePart(parts[0], &head); err != nil {
		return Claims{}, err
	}
	if len(head.Crit) > 0 {
		return Claims{}, fmt.Errorf("%w: critical header %q not understood", ErrTokenMalformed, head.Crit) // RFC 7515 4.1.11
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Claims{}, ErrTokenMalformed
	}
	key, err := j.key(ctx, head.Kid)
	if err != nil {
		return Claims{}, err
	}
	if err := verifySignature(head.Alg, key, []byte(parts[0]+"."+parts[1]), sig); err != nil {
		return Claims{}, err
	}
	return j.claims(parts[1])
}

// claims decodes and checks the payload.
func (j *JWKS) claims(part string) (Claims, error) {
	var raw map[string]any
	if err := decodePart(part, &raw); err != nil {
		return Claims{}, err
	}
	c := Claims{All: raw}
	c.Subject, _ = raw["sub"].(string)
	c.Issuer, _ = raw["iss"].(string)
	switch a := raw["aud"].(type) {
	case string:
		c.Audience = []string{a}
	case []any:
		for _, s := range a {
			if s, ok := s.(string); ok {
				c.Audience = append(c.Audience, s)
			}
		}
	}
	now := j.clock()
	exp, ok := raw["exp"].(float64)
	if !ok {
		return c, fmt.Errorf("%w: no expiry (exp)", ErrTokenClaims) // never good for ever (#59)
	}
	c.Expires = time.Unix(int64(exp), 0)
	if now.After(c.Expires.Add(tokenLeeway)) {
		return c, fmt.Errorf("%w: expired at %s", ErrTokenExpired, c.Expires.UTC().Format(time.RFC3339))
	}
	if nbf, ok := raw["nbf"].(float64); ok && now.Add(tokenLeeway).Before(time.Unix(int64(nbf), 0)) {
		return c, fmt.Errorf("%w: not before %s", ErrTokenExpired, time.Unix(int64(nbf), 0).UTC().Format(time.RFC3339))
	}
	if j.Issuer != "" && c.Issuer != j.Issuer {
		return c, fmt.Errorf("%w: issuer %q, want %q", ErrTokenClaims, c.Issuer, j.Issuer)
	}
	if j.Audience != "" && !containsString(c.Audience, j.Audience) {
		return c, fmt.Errorf("%w: audience %q, want %q", ErrTokenClaims, c.Audience, j.Audience)
	}
	return c, nil
}

func containsString(list []string, s string) bool {
	for _, l := range list {
		if l == s {
			return true
		}
	}
	return false
}

func decodePart(part string, v any) error {
	b, err := base64.RawURLEncoding.DecodeString(part)
	if err != nil {
		return ErrTokenMalformed
	}
	if json.Unmarshal(b, v) != nil {
		return ErrTokenMalformed
	}
	return nil
}

// key is the key of kid, fetched again when it is unknown (or the keys are
// an hour old). A token without kid takes the only key there is.
func (j *JWKS) key(ctx context.Context, kid string) (crypto.PublicKey, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	pick := func() (crypto.PublicKey, bool) {
		if kid == "" && len(j.keys) == 1 {
			for _, k := range j.keys {
				return k, true
			}
		}
		k, ok := j.keys[kid]
		return k, ok
	}
	now := j.clock()
	if k, ok := pick(); ok && now.Sub(j.fetched) < jwksRefresh {
		return k, nil
	}
	if now.Sub(j.tried) >= jwksMinFetch {
		j.tried = now
		keys, err := j.fetch(ctx)
		if err != nil {
			if k, ok := pick(); ok {
				return k, nil // the keys we have, while the server is away
			}
			return nil, fmt.Errorf("%w: %v", ErrTokenKey, err)
		}
		j.keys, j.fetched = keys, now
	}
	if k, ok := pick(); ok {
		return k, nil
	}
	if len(j.keys) == 0 {
		return nil, fmt.Errorf("%w: no keys at %s", ErrTokenKey, j.URL)
	}
	return nil, fmt.Errorf("%w: unknown kid %q (%d keys at %s)", ErrTokenKey, kid, len(j.keys), j.URL)
}

// Load reads the key set now, as Verify would: how many keys it has, or
// why it could not be read (a director checks its -jwks at start).
func (j *JWKS) Load(ctx context.Context) (int, error) {
	keys, err := j.fetch(ctx)
	if err != nil {
		return 0, err
	}
	j.mu.Lock()
	j.keys, j.fetched = keys, j.clock()
	j.mu.Unlock()
	return len(keys), nil
}

// fetch reads the key set at j.URL.
func (j *JWKS) fetch(ctx context.Context) (map[string]crypto.PublicKey, error) {
	if !strings.HasPrefix(j.URL, "http://") && !strings.HasPrefix(j.URL, "https://") {
		// A file (a path, or file://path): read again on every fetch.
		f, err := os.Open(strings.TrimPrefix(j.URL, "file://"))
		if err != nil {
			return nil, fmt.Errorf("jwks: %w", err)
		}
		defer f.Close()
		return decodeKeySet(f, j.URL)
	}
	client := j.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, j.URL, nil)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jwks %s: %s", j.URL, res.Status)
	}
	return decodeKeySet(res.Body, j.URL)
}

// decodeKeySet reads a JWKS document: its signing keys by kid.
func decodeKeySet(r io.Reader, from string) (map[string]crypto.PublicKey, error) {
	var set struct {
		Keys []jwk `json:"keys"`
	}
	if err := json.NewDecoder(r).Decode(&set); err != nil {
		return nil, fmt.Errorf("jwks %s: %w", from, err)
	}
	keys := map[string]crypto.PublicKey{}
	for _, k := range set.Keys {
		if k.Use != "" && k.Use != "sig" {
			continue
		}
		if pk, err := k.public(); err == nil {
			keys[k.Kid] = pk
		}
	}
	return keys, nil
}

// jwk is a published key (RFC 7517).
type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func (k jwk) public() (crypto.PublicKey, error) {
	num := func(s string) (*big.Int, error) {
		b, err := base64.RawURLEncoding.DecodeString(s)
		if err != nil || len(b) == 0 {
			return nil, ErrTokenKey
		}
		return new(big.Int).SetBytes(b), nil
	}
	switch k.Kty {
	case "RSA":
		n, err := num(k.N)
		if err != nil {
			return nil, err
		}
		e, err := num(k.E)
		if err != nil || !e.IsInt64() {
			return nil, ErrTokenKey
		}
		return &rsa.PublicKey{N: n, E: int(e.Int64())}, nil
	case "EC":
		var curve elliptic.Curve
		size := 32
		switch k.Crv {
		case "P-256":
			curve = elliptic.P256()
		case "P-384":
			curve, size = elliptic.P384(), 48
		default:
			return nil, ErrTokenKey
		}
		x, errX := base64.RawURLEncoding.DecodeString(k.X)
		y, errY := base64.RawURLEncoding.DecodeString(k.Y)
		if errX != nil || errY != nil || len(x) != size || len(y) != size {
			return nil, ErrTokenKey
		}
		pk, err := ecdsa.ParseUncompressedPublicKey(curve, append(append([]byte{4}, x...), y...))
		if err != nil {
			return nil, ErrTokenKey
		}
		return pk, nil
	case "OKP":
		b, err := base64.RawURLEncoding.DecodeString(k.X)
		if k.Crv != "Ed25519" || err != nil || len(b) != ed25519.PublicKeySize {
			return nil, ErrTokenKey
		}
		return ed25519.PublicKey(b), nil
	}
	return nil, ErrTokenKey
}

// verifySignature checks sig over signed with key by alg.
func verifySignature(alg string, key crypto.PublicKey, signed, sig []byte) error {
	digest := func(h hash.Hash) []byte { h.Write(signed); return h.Sum(nil) }
	switch alg {
	case "RS256", "RS384", "RS512", "PS256":
		pk, ok := key.(*rsa.PublicKey)
		if !ok {
			return ErrTokenSignature
		}
		var err error
		switch alg {
		case "RS256":
			err = rsa.VerifyPKCS1v15(pk, crypto.SHA256, digest(sha256.New()), sig)
		case "RS384":
			err = rsa.VerifyPKCS1v15(pk, crypto.SHA384, digest(sha512.New384()), sig)
		case "RS512":
			err = rsa.VerifyPKCS1v15(pk, crypto.SHA512, digest(sha512.New()), sig)
		case "PS256":
			err = rsa.VerifyPSS(pk, crypto.SHA256, digest(sha256.New()), sig, nil)
		}
		if err != nil {
			return ErrTokenSignature
		}
		return nil
	case "ES256", "ES384":
		pk, ok := key.(*ecdsa.PublicKey)
		size, h := 32, hash.Hash(sha256.New())
		if alg == "ES384" {
			size, h = 48, sha512.New384()
		}
		if !ok || len(sig) != 2*size || pk.Curve.Params().BitSize != size*8 {
			return ErrTokenSignature
		}
		r, s := new(big.Int).SetBytes(sig[:size]), new(big.Int).SetBytes(sig[size:])
		if !ecdsa.Verify(pk, digest(h), r, s) {
			return ErrTokenSignature
		}
		return nil
	case "EdDSA":
		pk, ok := key.(ed25519.PublicKey)
		if !ok || !ed25519.Verify(pk, signed, sig) {
			return ErrTokenSignature
		}
		return nil
	}
	return fmt.Errorf("%w: alg %q", ErrTokenSignature, alg) // none, HS*: never
}
