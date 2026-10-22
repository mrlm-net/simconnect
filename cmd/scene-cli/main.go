// Command scene-cli helps write camera scenes against a running airport map
// (cmd/airport-map): spawn the aircraft and a car to frame, capture the
// simulator's camera as a composition (placed on the aircraft and the
// airport, with a screenshot), play shots, glides and scenes.
//
//	scene-cli spawn -tail DEMO009 -model FSLTL_WANTED_A320_Midnight [-stand -1] [-push-min 120]
//	scene-cli car [-title FSDT_FollowMe_Hilux] (-near DEMO009 -right 25 -forward 10 | -lat .. -lon .. | -at-camera) [-hdg 240]
//	scene-cli capture NAME [-tail DEMO009] [-dir compositions]
//	scene-cli shot -tail DEMO009 (-glide 4 | -from sideDolly_dist1_t50 [-to dolly] | -move engine [-at 0.5] [-back 1.6]) [-side -1]
//	scene-cli scene 4-showcase
//	scene-cli vehicle-radio [on|off]
//	scene-cli off | moves | glides
//
// -map is the map's address (http://127.0.0.1:8080), -icao the airport (LKPR).
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

var (
	mapURL = flag.String("map", "http://127.0.0.1:8080", "the airport map")
	icao   = flag.String("icao", "LKPR", "the airport")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: scene-cli [-map URL] [-icao ICAO] spawn|car|capture|shot|scene|vehicle-radio|off|moves|glides [flags]")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}
	cmd, args := flag.Arg(0), flag.Args()[1:]
	var err error
	switch cmd {
	case "spawn":
		err = spawn(args)
	case "car":
		err = car(args)
	case "capture":
		err = capture(args)
	case "shot":
		err = shot(args)
	case "scene":
		if len(args) == 0 {
			err = errors.New("scene NAME")
			break
		}
		err = show(call(http.MethodPost, "/api/camera/scene?name="+args[0]+"&icao="+*icao, nil))
	case "off":
		err = show(call(http.MethodPost, "/api/camera", map[string]string{"mode": "off"}))
	case "vehicle-radio":
		if len(args) == 0 || args[0] != "on" && args[0] != "off" {
			err = show(call(http.MethodGet, "/api/vehicles/radio", nil))
			break
		}
		err = show(call(http.MethodPost, "/api/vehicles/radio", map[string]bool{"on": args[0] == "on"}))
	case "moves":
		err = show(call(http.MethodGet, "/api/camera/moves", nil))
	case "glides":
		err = glides()
	default:
		flag.Usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "scene-cli:", err)
		os.Exit(1)
	}
}

// call asks the map; a non-2xx answer is an error with its text.
func call(method, path string, body any) ([]byte, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, *mapURL+path, r)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	out, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode/100 != 2 {
		return nil, fmt.Errorf("%s %s: %s", method, path, strings.TrimSpace(string(out)))
	}
	return out, nil
}

// show prints an answer, indented.
func show(b []byte, err error) error {
	if err != nil {
		return err
	}
	var v any
	if json.Unmarshal(b, &v) == nil {
		b, _ = json.MarshalIndent(v, "", "  ")
	}
	fmt.Println(string(b))
	return nil
}

func spawn(args []string) error {
	fs := flag.NewFlagSet("spawn", flag.ExitOnError)
	tail := fs.String("tail", "DEMO009", "call sign")
	model := fs.String("model", "FSLTL_WANTED_A320_Midnight", "model title")
	stand := fs.Int("stand", -1, "parking index (-1: one picked)")
	push := fs.Float64("push-min", 120, "minutes until it asks for pushback")
	fs.Parse(args)
	b, err := call(http.MethodPost, "/api/control", map[string]any{"kind": "departure", "icao": *icao, "stand": *stand,
		"pushInMin": *push, "model": *model, "tail": *tail})
	if err != nil {
		return err
	}
	var v struct{ Tail, Stand, State string }
	json.Unmarshal(b, &v)
	fmt.Printf("%s at %s (%s)\n", v.Tail, v.Stand, v.State)
	return nil
}

// aircraftAt is where one of the map's aircraft is.
func aircraftAt(tail string) (lat, lon, hdg float64, err error) {
	b, err := call(http.MethodGet, "/api/control", nil)
	if err != nil {
		return 0, 0, 0, err
	}
	var list []struct {
		Tail     string
		Heading  float64
		Position struct{ Lat, Lon float64 }
	}
	if err := json.Unmarshal(b, &list); err != nil {
		return 0, 0, 0, err
	}
	for _, a := range list {
		if strings.EqualFold(a.Tail, tail) {
			return a.Position.Lat, a.Position.Lon, a.Heading, nil
		}
	}
	return 0, 0, 0, fmt.Errorf("no aircraft %s on the map", tail)
}

// car places a ground vehicle and keeps it there until Ctrl+C (the
// simulator removes it with this connection).
func car(args []string) error {
	fs := flag.NewFlagSet("car", flag.ExitOnError)
	title := fs.String("title", "FSDT_FollowMe_Hilux", "SimObject title")
	near := fs.String("near", "", "beside this aircraft (-right, -forward from it)")
	right := fs.Float64("right", 25, "meters to its right (with -near)")
	fwd := fs.Float64("forward", 0, "meters ahead of it (with -near)")
	atCam := fs.Bool("at-camera", false, "where the simulator's camera is")
	lat := fs.Float64("lat", 0, "latitude")
	lon := fs.Float64("lon", 0, "longitude")
	hdg := fs.Float64("hdg", -1, "heading, degrees true (-1: as the aircraft, or 0)")
	fs.Parse(args)
	switch {
	case *near != "":
		alat, alon, ahdg, err := aircraftAt(*near)
		if err != nil {
			return err
		}
		*lat, *lon = calc.DisplaceByHeading(alat, alon, ahdg, *fwd)
		*lat, *lon = calc.DisplaceByHeading(*lat, *lon, math.Mod(ahdg+90, 360), *right)
		if *hdg < 0 {
			*hdg = ahdg
		}
	case *atCam:
		b, err := call(http.MethodPost, "/api/camera/capture", map[string]string{"icao": *icao, "name": "car"})
		if err != nil {
			return err
		}
		var c struct {
			Camera struct {
				Lat, Lon   float64
				HeadingDeg float64
			}
		}
		json.Unmarshal(b, &c)
		*lat, *lon = c.Camera.Lat, c.Camera.Lon
		if *hdg < 0 {
			*hdg = c.Camera.HeadingDeg
		}
	case *lat == 0 && *lon == 0:
		return errors.New("car: -near TAIL, -at-camera or -lat/-lon")
	}
	if *hdg < 0 {
		*hdg = 0
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	client := simconnect.NewClient("scene-cli", engine.WithContext(ctx))
	if err := client.Connect(); err != nil {
		return err
	}
	defer client.Disconnect()
	pos := types.SIMCONNECT_DATA_INITPOSITION{Latitude: *lat, Longitude: *lon, Altitude: 0, Heading: *hdg, OnGround: 1}
	if err := client.AICreateSimulatedObject(*title, pos, 1); err != nil {
		return err
	}
	fmt.Printf("%s at %.6f, %.6f heading %.0f — Ctrl+C removes it\n", *title, *lat, *lon, *hdg)
	for {
		select {
		case <-ctx.Done():
			return nil
		case m, ok := <-client.Stream():
			if !ok {
				return errors.New("the simulator closed the connection")
			}
			switch types.SIMCONNECT_RECV_ID(m.SIMCONNECT_RECV.DwID) {
			case types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID:
				fmt.Println("placed, object", m.AsAssignedObjectID().DwObjectID)
			case types.SIMCONNECT_RECV_ID_EXCEPTION:
				return fmt.Errorf("the simulator refused it (exception %d): no SimObject %q?", m.AsException().DwException, *title)
			}
		}
	}
}

// capture saves the simulator's camera as NAME.json and its picture as
// NAME.jpg in -dir.
func capture(args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("capture NAME [-tail T] [-dir D]")
	}
	name := args[0]
	fs := flag.NewFlagSet("capture", flag.ExitOnError)
	tail := fs.String("tail", "", "the aircraft filmed (nearest the line of sight otherwise)")
	dir := fs.String("dir", "compositions", "where to save")
	fs.Parse(args[1:])
	b, err := call(http.MethodPost, "/api/camera/capture", map[string]string{"icao": *icao, "name": name, "tail": *tail})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(*dir, 0o755); err != nil {
		return err
	}
	var v any
	json.Unmarshal(b, &v)
	pretty, _ := json.MarshalIndent(v, "", "  ")
	if err := os.WriteFile(filepath.Join(*dir, name+".json"), pretty, 0o644); err != nil {
		return err
	}
	fmt.Println(string(pretty))
	jpg, _ := filepath.Abs(filepath.Join(*dir, name+".jpg"))
	if err := grab(jpg); err != nil {
		fmt.Fprintln(os.Stderr, "scene-cli: no picture:", err)
		return nil
	}
	fmt.Println("saved", filepath.Join(*dir, name+".json"), "and", jpg)
	return nil
}

// grabScript draws the simulator's window into a JPEG (PrintWindow: the
// window must be on screen, a covered one gives a stale picture).
const grabScript = `param([string]$Out)
Add-Type -AssemblyName System.Drawing
Add-Type @"
using System; using System.Runtime.InteropServices;
public static class W {
  [DllImport("user32.dll")] public static extern bool PrintWindow(IntPtr h, IntPtr dc, uint f);
  [DllImport("user32.dll")] public static extern bool GetClientRect(IntPtr h, out R r);
  [StructLayout(LayoutKind.Sequential)] public struct R { public int L, T, Ri, B; }
}
"@
$p = Get-Process FlightSimulator2024, FlightSimulator -ErrorAction SilentlyContinue | Select-Object -First 1
if (-not $p) { throw "the simulator is not running" }
$h = $p.MainWindowHandle
$r = New-Object W+R; [W]::GetClientRect($h, [ref]$r) | Out-Null
$bmp = New-Object System.Drawing.Bitmap ($r.Ri - $r.L), ($r.B - $r.T)
$g = [System.Drawing.Graphics]::FromImage($bmp)
$dc = $g.GetHdc(); [W]::PrintWindow($h, $dc, 3) | Out-Null; $g.ReleaseHdc($dc)
$small = New-Object System.Drawing.Bitmap $bmp, 1280, ([int](1280 * $bmp.Height / $bmp.Width))
$small.Save($Out, [System.Drawing.Imaging.ImageFormat]::Jpeg)
`

func grab(out string) error {
	f, err := os.CreateTemp("", "scene-cli-*.ps1")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(grabScript); err != nil {
		f.Close()
		return err
	}
	f.Close()
	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", f.Name(), "-Out", out)
	if b, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(b)))
	}
	return nil
}

func shot(args []string) error {
	fs := flag.NewFlagSet("shot", flag.ExitOnError)
	tail := fs.String("tail", "DEMO009", "the aircraft")
	glide := fs.Int("glide", 0, "glide number (scene-cli glides)")
	from := fs.String("from", "", "frame to glide from")
	to := fs.String("to", "", "frame to glide to (dolly: slide along it; empty: hold)")
	move := fs.String("move", "", "a move (scene-cli moves)")
	at := fs.Float64("at", -1, "hold the move at 0..1")
	back := fs.Float64("back", 1, "distance factor")
	side := fs.Float64("side", 1, "its right (1) or left (-1)")
	fs.Parse(args)
	req := map[string]any{"icao": *icao, "tail": *tail, "glide": *glide, "from": *from, "to": *to, "move": *move, "back": *back, "side": *side}
	if *at >= 0 {
		req["at"] = *at
	}
	return show(call(http.MethodPost, "/api/camera/shot", req))
}

func glides() error {
	b, err := call(http.MethodGet, "/api/camera/glides", nil)
	if err != nil {
		return err
	}
	var list [][2]string
	if err := json.Unmarshal(b, &list); err != nil {
		return err
	}
	for i, g := range list {
		fmt.Printf("%3d  %s → %s\n", i+1, g[0], g[1])
	}
	return nil
}
