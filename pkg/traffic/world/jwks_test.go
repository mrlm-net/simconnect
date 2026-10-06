package world

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

var b64 = base64.RawURLEncoding

// testKeys are an RSA, a P-256 and an Ed25519 key and their JWKS.
type testKeys struct {
	rsa *rsa.PrivateKey
	ec  *ecdsa.PrivateKey
	ed  ed25519.PrivateKey
}

func newTestKeys(t *testing.T) testKeys {
	t.Helper()
	r, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	e, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, d, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return testKeys{r, e, d}
}

func (k testKeys) jwks() []byte {
	pad := func(b []byte, n int) []byte { return append(make([]byte, n-len(b)), b...) }
	set := map[string]any{"keys": []map[string]string{
		{"kty": "RSA", "kid": "r1", "use": "sig", "n": b64.EncodeToString(k.rsa.N.Bytes()), "e": b64.EncodeToString(big.NewInt(int64(k.rsa.E)).Bytes())},
		{"kty": "EC", "kid": "e1", "crv": "P-256", "x": b64.EncodeToString(pad(k.ec.X.Bytes(), 32)), "y": b64.EncodeToString(pad(k.ec.Y.Bytes(), 32))},
		{"kty": "OKP", "kid": "d1", "crv": "Ed25519", "x": b64.EncodeToString(k.ed.Public().(ed25519.PublicKey))},
	}}
	b, _ := json.Marshal(set)
	return b
}

// sign makes a token of claims signed by alg with the key kid.
func (k testKeys) sign(t *testing.T, alg, kid string, claims map[string]any) string {
	t.Helper()
	h, _ := json.Marshal(map[string]string{"alg": alg, "kid": kid, "typ": "JWT"})
	c, _ := json.Marshal(claims)
	signed := b64.EncodeToString(h) + "." + b64.EncodeToString(c)
	sum := sha256.Sum256([]byte(signed))
	var sig []byte
	var err error
	switch alg {
	case "RS256":
		sig, err = rsa.SignPKCS1v15(rand.Reader, k.rsa, crypto.SHA256, sum[:])
	case "ES256":
		var r, s *big.Int
		r, s, err = ecdsa.Sign(rand.Reader, k.ec, sum[:])
		if err == nil {
			sig = append(r.FillBytes(make([]byte, 32)), s.FillBytes(make([]byte, 32))...)
		}
	case "EdDSA":
		sig = ed25519.Sign(k.ed, []byte(signed))
	case "none":
	}
	if err != nil {
		t.Fatal(err)
	}
	return signed + "." + b64.EncodeToString(sig)
}

// TestJWKSVerify: the MyCrew API's session tokens (#792): each algorithm
// verified by its kid; expired, wrong issuer or audience, a forged or an
// unsigned token refused; the keys fetched once, again for an unknown kid.
func TestJWKSVerify(t *testing.T) {
	keys := newTestKeys(t)
	var fetches atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetches.Add(1)
		w.Write(keys.jwks())
	}))
	defer srv.Close()
	j := NewJWKS(srv.URL, "https://mycrew.outlays.dev", "traffic-director")
	ctx := context.Background()
	exp := time.Now().Add(10 * time.Minute)
	good := map[string]any{"sub": "player-7", "iss": "https://mycrew.outlays.dev", "aud": []string{"traffic-director"}, "exp": exp.Unix()}
	for alg, kid := range map[string]string{"RS256": "r1", "ES256": "e1", "EdDSA": "d1"} {
		c, err := j.Verify(ctx, keys.sign(t, alg, kid, good))
		if err != nil {
			t.Fatalf("%s: %v", alg, err)
		}
		if c.Subject != "player-7" || c.Expires.Unix() != exp.Unix() {
			t.Errorf("%s: claims %+v", alg, c)
		}
	}
	if n := fetches.Load(); n != 1 {
		t.Errorf("keys fetched %d times, want once", n)
	}
	bad := func(name string, token string, want error) {
		t.Helper()
		if _, err := j.Verify(ctx, token); !errors.Is(err, want) {
			t.Errorf("%s: %v, want %v", name, err, want)
		}
	}
	with := func(k string, v any) map[string]any {
		m := map[string]any{}
		for a, b := range good {
			m[a] = b
		}
		m[k] = v
		return m
	}
	bad("expired", keys.sign(t, "RS256", "r1", with("exp", time.Now().Add(-2*time.Minute).Unix())), ErrTokenExpired)
	bad("not yet valid", keys.sign(t, "RS256", "r1", with("nbf", time.Now().Add(5*time.Minute).Unix())), ErrTokenExpired)
	bad("other issuer", keys.sign(t, "RS256", "r1", with("iss", "https://evil")), ErrTokenClaims)
	bad("other audience", keys.sign(t, "RS256", "r1", with("aud", "someone-else")), ErrTokenClaims)
	bad("unsigned", keys.sign(t, "none", "r1", good), ErrTokenSignature)
	bad("key of another alg", keys.sign(t, "ES256", "r1", good), ErrTokenSignature)
	tok := keys.sign(t, "RS256", "r1", good)
	bad("forged claims", tok[:len(tok)-10]+"AAAAAAAAAA", ErrTokenSignature)
	bad("not a token", "abc", ErrTokenMalformed)
	before := fetches.Load()
	j.mu.Lock()
	j.fetched = time.Now().Add(-time.Minute) // past jwksMinFetch
	j.mu.Unlock()
	bad("unknown kid", keys.sign(t, "RS256", "r9", good), ErrTokenKey)
	if fetches.Load() != before+1 {
		t.Error("an unknown kid did not fetch the keys again")
	}
}

// selfSigned is a certificate for 127.0.0.1 and the pool that trusts it.
func selfSigned(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "director"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, _ := x509.ParseCertificate(der)
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}, pool
}

// TestSecureLink: the actuator dials the director over TLS with a fresh
// session token every time (TokenFunc); the director verifies it (JWKS),
// closes the link when it expires, and the actuator greets again with a
// new one. An actuator not trusting the certificate never attaches.
func TestSecureLink(t *testing.T) {
	redialEvery = 100 * time.Millisecond
	defer func() { redialEvery = 5 * time.Second }()
	keys := newTestKeys(t)
	jw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(keys.jwks()) }))
	defer jw.Close()
	j := NewJWKS(jw.URL, "", "")
	cert, pool := selfSigned(t)
	server := LinkOptions{Verify: j.LinkVerify, TLS: &tls.Config{Certificates: []tls.Certificate{cert}}}
	ln, err := server.listen("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	var issued atomic.Int32
	life := 2 * time.Second // exp is whole seconds: at least 1 s of life
	client := LinkOptions{TLS: &tls.Config{RootCAs: pool}, TokenFunc: func(context.Context) (string, error) {
		issued.Add(1)
		// Expired a minute ago plus the leeway: refused at once, unless
		// life is added.
		return keys.sign(t, "EdDSA", "d1", map[string]any{"sub": "p1", "exp": time.Now().Add(-tokenLeeway + life).Unix()}), nil
	}}
	hub := newHubLink()
	defer hub.Close()
	srv := newWireServer()
	srv.add("dep/1", &fakeDep{})
	go serve(hub, srv)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.dialOut(ctx, ln.Addr().String(), client, func(string, ...any) {})

	accept := func() *wireClient {
		t.Helper()
		for {
			c, err := ln.Accept()
			if err != nil {
				t.Fatal(err)
			}
			if l, ok := server.greeted(ctx, c, t.Logf); ok {
				return newWireClient(l, func(wireMsg) {})
			}
		}
	}
	c1 := accept()
	if s := (&remoteDep{c: c1, t: "dep/1"}).State(); s != traffic.TaxiHoldingShort {
		t.Fatalf("state over TLS: %v", s)
	}
	// The token expires: the director closes the link, the actuator
	// greets again with a new token.
	c2 := accept()
	if s := (&remoteDep{c: c2, t: "dep/1"}).State(); s != traffic.TaxiHoldingShort {
		t.Fatalf("state after the new token: %v", s)
	}
	if n := issued.Load(); n < 2 {
		t.Errorf("tokens issued %d, want a fresh one per greeting", n)
	}

	// Not trusting the certificate: the handshake fails, nothing attaches.
	cancel() // the actuator stops dialling
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.(*tls.Conn).Handshake()
			c.Close()
		}
	}()
	untrusting := LinkOptions{TLS: &tls.Config{}, Token: "x"}
	dctx, dcancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer dcancel()
	if c, err := untrusting.dial(dctx, ln.Addr().String()); err == nil {
		c.Close()
		t.Error("dialled a director whose certificate it does not trust")
	}
}
