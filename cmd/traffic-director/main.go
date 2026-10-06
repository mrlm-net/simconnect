// Command traffic-director takes a split traffic World's decisions (#710):
// schedule, ATC, sequencing, separation, conflicts. It needs no simulator —
// it runs anywhere, Linux included — and drives a traffic-actuator beside
// the simulator over the network. It serves the World's HTTP API (the
// airport map's), and the map's page with -web.
//
//	traffic-director -actuator simpc:7710 -token s3cret -addr :8080 -web cmd/airport-map/web
//
// For multiplayer it waits for the players' actuators to dial in, over
// TLS, each greeting with its MyCrew API session token (#792):
//
//	traffic-director -listen :7710 -jwks https://mycrew.outlays.dev/v1/traffic/jwks \
//	  -tls-cert director.crt -tls-key director.key
package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

func main() {
	actuator := flag.String("actuator", "127.0.0.1:7710", "the traffic-actuator to drive")
	token := flag.String("token", "", "the actuator's token")
	listen := flag.String("listen", "", "wait on this address (\":7710\") for an actuator that dials in, instead of dialling -actuator (#774)")
	addr := flag.String("addr", "127.0.0.1:8080", "HTTP address of the World's API")
	web := flag.String("web", "", "directory of the airport map's page to serve (cmd/airport-map/web); \"\": the API only")
	airways := flag.String("airways", "", "airway graph for flight plans; \"\": direct routes")
	logDir := flag.String("log-dir", ".", "directory for the traffic log")
	dataDir := flag.String("data-dir", ".", "directory for local settings")
	apiToken := flag.String("api-token", "", "token a client needs to control the traffic over the HTTP API (\"auto\": a random one; \"\": open, trusted networks only)")
	viewToken := flag.String("view-token", "", "token a client needs to read the HTTP API (\"auto\": a random one)")
	jwksURL := flag.String("jwks", "", "with -listen: verify each actuator's session token against the keys at this JWKS address (the MyCrew API's) or in this JWKS file, instead of -token")
	jwksIss := flag.String("jwks-iss", "", "with -jwks: the issuer a token must name")
	jwksAud := flag.String("jwks-aud", "", "with -jwks: the audience a token must include")
	jwksAPI := flag.String("jwks-api", "control", "with -jwks: what a session token gives on the HTTP API: control, view or none")
	tlsCert := flag.String("tls-cert", "", "with -listen: serve the link over TLS with this certificate (PEM)")
	tlsKey := flag.String("tls-key", "", "with -listen: the certificate's private key (PEM)")
	flag.Parse()
	if (*jwksURL != "" || *tlsCert != "") && *listen == "" {
		fmt.Fprintln(os.Stderr, "❌ -jwks and -tls-cert need -listen")
		os.Exit(2)
	}
	link := world.LinkOptions{Token: *token}
	var jwks *world.JWKS
	if *jwksURL != "" {
		jwks = world.NewJWKS(*jwksURL, *jwksIss, *jwksAud)
		link.Verify = jwks.LinkVerify
		// Checked now: a JWKS that cannot be read is said at start, not
		// found out from refused players (an empty set may be published
		// later, so it only warns).
		lctx, done := context.WithTimeout(context.Background(), 15*time.Second)
		n, err := jwks.Load(lctx)
		done()
		switch {
		case err != nil && !strings.HasPrefix(*jwksURL, "http"):
			fmt.Fprintln(os.Stderr, "❌ -jwks:", err)
			os.Exit(2)
		case err != nil:
			fmt.Fprintf(os.Stderr, "⚠️  -jwks: %v (tried again on the first token)\n", err)
		case n == 0:
			fmt.Fprintf(os.Stderr, "⚠️  -jwks %s has no keys yet: every token is refused until it has\n", *jwksURL)
		default:
			fmt.Printf("JWKS %s: %d key(s)\n", *jwksURL, n)
		}
	}
	if *tlsCert != "" || *tlsKey != "" {
		cert, err := tls.LoadX509KeyPair(*tlsCert, *tlsKey)
		if err != nil {
			fmt.Fprintln(os.Stderr, "❌ TLS:", err)
			os.Exit(2)
		}
		link.TLS = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	var graph *nav.AirwayGraph
	if *airways != "" {
		g, err := nav.LoadAirwayGraph(*airways)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  airways: %v (flight plans fly direct)\n", err)
		} else {
			graph = g
		}
	}
	w := world.New(world.Options{LogDir: *logDir, Airways: graph, DataDir: *dataDir})
	if *listen != "" {
		// Actuators behind routers dial in (#774).
		go func() {
			if err := world.ListenDirectorWith(ctx, w, *listen, link); err != nil {
				fmt.Fprintln(os.Stderr, "❌", err)
				cancel()
			}
		}()
	} else {
		go world.DialDirector(ctx, w, *actuator, *token)
	}

	mux := http.NewServeMux()
	if *web != "" {
		mux.Handle("GET /", http.FileServer(http.Dir(*web)))
	}
	w.Register(mux)
	// The HTTP API behind tokens on a public server (#781): a bearer token
	// (Authorization: Bearer …) or ?token=… once; this computer always in.
	for _, t := range []*string{apiToken, viewToken} {
		if *t == "auto" {
			b := make([]byte, 16)
			rand.Read(b)
			*t = hex.EncodeToString(b)
		}
	}
	if *apiToken != "" {
		fmt.Printf("API control token: %s\n", *apiToken)
	}
	if *viewToken != "" {
		fmt.Printf("API view token: %s\n", *viewToken)
	}
	world.SetTokens(*apiToken, *viewToken)
	if jwks != nil && *jwksAPI != "none" {
		refused := newRefusalLog()
		world.SetTokenVerifier(func(ctx context.Context, t string) error {
			c, err := jwks.Verify(ctx, t)
			if err != nil {
				refused.log(t, c.Subject, err)
			}
			return err
		}, *jwksAPI == "control")
	}
	srv := &http.Server{Addr: *addr, Handler: world.Guard(mux), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shut, done := context.WithTimeout(context.Background(), 2*time.Second)
		defer done()
		srv.Shutdown(shut)
	}()
	of := *actuator
	if *listen != "" {
		of = "actuators dialling " + *listen
	}
	fmt.Printf("director of %s, API on http://%s\n", of, *addr)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, "❌", err)
		os.Exit(1)
	}
}

// refusalLog prints why an HTTP API token was refused, once a minute per
// token and reason: a page polls with the same cookie every second.
type refusalLog struct {
	mu   sync.Mutex
	last map[string]time.Time
}

func newRefusalLog() *refusalLog { return &refusalLog{last: map[string]time.Time{}} }

func (r *refusalLog) log(token, subject string, err error) {
	k := token + "\x00" + err.Error()
	r.mu.Lock()
	defer r.mu.Unlock()
	if at, ok := r.last[k]; ok && time.Since(at) < time.Minute {
		return
	}
	if len(r.last) > 1000 {
		r.last = map[string]time.Time{}
	}
	r.last[k] = time.Now()
	who := subject
	if who == "" {
		who = "?"
	}
	fmt.Fprintf(os.Stderr, "%s  api: token of %s refused: %v\n", time.Now().Format("15:04:05.000"), who, err)
}
