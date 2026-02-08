//go:build windows
// +build windows

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/datasets"
	"github.com/mrlm-net/simconnect/pkg/datasets/facilities"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Definition IDs for facility datasets
const (
	defAirport   uint32 = 5000
	defRunway    uint32 = 5001
	defParking   uint32 = 5002
	defFrequency uint32 = 5003
)

// Request IDs for facility data requests
const (
	reqAirport   uint32 = 6000
	reqRunway    uint32 = 6001
	reqParking   uint32 = 6002
	reqFrequency uint32 = 6003
)

// Target airport ICAO code
var targetICAO = "LKPR"

// AirportData matches a subset of fields from the airport facility definition.
// We read: LATITUDE, LONGITUDE, ALTITUDE, MAGVAR, NAME, NAME64, ICAO, REGION.
// Note: Fields after REGION involve mixed-size types (FLOAT64 after STRING8)
// which create Go struct alignment mismatches with SimConnect's packed binary layout.
// The remaining fields defined in the dataset are sent but not parsed here.
type AirportData struct {
	Latitude  float64
	Longitude float64
	Altitude  float64
	MagVar    float32
	Name      [32]byte
	Name64    [64]byte
	ICAO      [8]byte
	Region    [8]byte
}

// RunwayData matches the field order in NewRunwayFacilityDataset:
// LATITUDE, LONGITUDE, ALTITUDE, HEADING, LENGTH, WIDTH,
// PATTERN_ALTITUDE, SLOPE, TRUE_SLOPE, SURFACE,
// PRIMARY_ILS_ICAO, PRIMARY_ILS_REGION, PRIMARY_ILS_TYPE,
// PRIMARY_NUMBER, PRIMARY_DESIGNATOR, PRIMARY_THRESHOLD, PRIMARY_BLASTPAD,
// PRIMARY_OVERRUN, PRIMARY_APPROACH_LIGHTS, PRIMARY_LEFT_VASI, PRIMARY_RIGHT_VASI,
// SECONDARY_ILS_ICAO, SECONDARY_ILS_REGION, SECONDARY_ILS_TYPE,
// SECONDARY_NUMBER, SECONDARY_DESIGNATOR, SECONDARY_THRESHOLD, SECONDARY_BLASTPAD,
// SECONDARY_OVERRUN, SECONDARY_APPROACH_LIGHTS, SECONDARY_LEFT_VASI, SECONDARY_RIGHT_VASI
type RunwayData struct {
	Latitude   float64
	Longitude  float64
	Altitude   float64
	Heading    float32
	Length     float32
	Width      float32
	PatternAlt float32
	Slope      float32
	TrueSlope  float32
	Surface    int32
}

// ParkingData matches the field order in NewParkingFacilityDataset:
// TYPE, TAXI_POINT_TYPE, NAME, SUFFIX, NUMBER, ORIENTATION, HEADING, RADIUS, BIAS_X, BIAS_Z
type ParkingData struct {
	Type          int32
	TaxiPointType int32
	Name          int32
	Suffix        int32
	Number        uint32
	Orientation   float32
	Heading       float32
	Radius        float32
	BiasX         float32
	BiasZ         float32
}

// FrequencyData matches the field order in NewFrequencyFacilityDataset:
// TYPE, FREQUENCY, NAME
type FrequencyData struct {
	Type      int32
	Frequency int32
	Name      [64]byte
}

func frequencyTypeName(t int32) string {
	switch t {
	case 0:
		return "None"
	case 1:
		return "ATIS"
	case 2:
		return "Multicom"
	case 3:
		return "UNICOM"
	case 4:
		return "CTAF"
	case 5:
		return "Ground"
	case 6:
		return "Tower"
	case 7:
		return "Clearance"
	case 8:
		return "Approach"
	case 9:
		return "Departure"
	case 10:
		return "Center"
	case 11:
		return "FSS"
	case 12:
		return "AWOS"
	case 13:
		return "ASOS"
	case 14:
		return "Clearance Pre-Taxi"
	case 15:
		return "Remote Clearance Delivery"
	default:
		return fmt.Sprintf("Unknown(%d)", t)
	}
}

func surfaceTypeName(s int32) string {
	switch s {
	case 0:
		return "Concrete"
	case 1:
		return "Grass"
	case 2:
		return "Water"
	case 3:
		return "Grass Bumpy"
	case 4:
		return "Asphalt"
	case 7:
		return "Clay"
	case 8:
		return "Snow"
	case 9:
		return "Ice"
	case 10:
		return "Dirt"
	case 11:
		return "Coral"
	case 12:
		return "Gravel"
	case 13:
		return "Oil Treated"
	case 14:
		return "Steel Mats"
	case 15:
		return "Bituminous"
	case 16:
		return "Brick"
	case 17:
		return "Macadam"
	case 18:
		return "Planks"
	case 19:
		return "Sand"
	case 20:
		return "Shale"
	case 21:
		return "Tarmac"
	case 22:
		return "Wright Flyer Track"
	case 254:
		return "Unknown"
	default:
		return fmt.Sprintf("Surface(%d)", s)
	}
}

func parkingTypeName(t int32) string {
	switch t {
	case 0:
		return "None"
	case 1:
		return "Ramp GA"
	case 2:
		return "Ramp GA Small"
	case 3:
		return "Ramp GA Medium"
	case 4:
		return "Ramp GA Large"
	case 5:
		return "Ramp Cargo"
	case 6:
		return "Ramp Mil Cargo"
	case 7:
		return "Ramp Mil Combat"
	case 8:
		return "Gate Small"
	case 9:
		return "Gate Medium"
	case 10:
		return "Gate Heavy"
	case 11:
		return "Dock GA"
	case 12:
		return "Fuel"
	case 13:
		return "Vehicle"
	case 14:
		return "Ramp GA Extra"
	case 15:
		return "Gate Extra"
	default:
		return fmt.Sprintf("Type(%d)", t)
	}
}

// runConnection handles a single connection lifecycle to the simulator.
func runConnection(ctx context.Context) error {
	client := simconnect.NewClient("GO Example - SimConnect Facilities",
		engine.WithContext(ctx),
	)

	fmt.Println("Waiting for simulator to start...")
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := client.Connect(); err != nil {
				time.Sleep(2 * time.Second)
				continue
			}
			goto connected
		}
	}

connected:
	fmt.Println("Connected to SimConnect")

	// Register facility datasets using the RegisterFacilityDataset helper.
	// Each dataset wraps OPEN/CLOSE brackets and field definitions,
	// so we don't need manual AddToFacilityDefinition calls.

	// Airport dataset - top-level airport info (defID 5000)
	// Nested datasets must be wrapped within the airport's OPEN/CLOSE.
	// We use separate definition IDs for each nesting pattern.
	if err := client.RegisterFacilityDataset(defAirport, facilities.NewAirportFacilityDataset()); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to register airport dataset: %v\n", err)
		return err
	}

	// Runway dataset nested under airport (defID 5001)
	// We use a custom dataset with only scalar fields (no STRUCT fields like
	// PRIMARY_THRESHOLD, PRIMARY_APPROACH_LIGHTS, etc. which require nested handling).
	// The full NewRunwayFacilityDataset() includes all fields for advanced use.
	runwayDataset := &datasets.FacilityDataSet{
		Definitions: []datasets.FacilityDataDefinition{
			"OPEN RUNWAY",
			"LATITUDE", "LONGITUDE", "ALTITUDE",
			"HEADING", "LENGTH", "WIDTH",
			"PATTERN_ALTITUDE", "SLOPE", "TRUE_SLOPE",
			"SURFACE",
			"CLOSE RUNWAY",
		},
	}
	client.AddToFacilityDefinition(defRunway, "OPEN AIRPORT")
	if err := client.RegisterFacilityDataset(defRunway, runwayDataset); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to register runway dataset: %v\n", err)
		return err
	}
	client.AddToFacilityDefinition(defRunway, "CLOSE AIRPORT")

	// Parking dataset nested under airport (defID 5002)
	client.AddToFacilityDefinition(defParking, "OPEN AIRPORT")
	if err := client.RegisterFacilityDataset(defParking, facilities.NewParkingFacilityDataset()); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to register parking dataset: %v\n", err)
		return err
	}
	client.AddToFacilityDefinition(defParking, "CLOSE AIRPORT")

	// Frequency dataset nested under airport (defID 5003)
	client.AddToFacilityDefinition(defFrequency, "OPEN AIRPORT")
	if err := client.RegisterFacilityDataset(defFrequency, facilities.NewFrequencyFacilityDataset()); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to register frequency dataset: %v\n", err)
		return err
	}
	client.AddToFacilityDefinition(defFrequency, "CLOSE AIRPORT")

	// Request facility data for the target airport
	client.RequestFacilityData(defAirport, reqAirport, targetICAO, "")
	client.RequestFacilityData(defRunway, reqRunway, targetICAO, "")
	client.RequestFacilityData(defParking, reqParking, targetICAO, "")
	client.RequestFacilityData(defFrequency, reqFrequency, targetICAO, "")

	fmt.Printf("Requested facility data for %s\n\n", targetICAO)

	var runwayCount, parkingCount, frequencyCount int

	stream := client.Stream()
	for {
		select {
		case <-ctx.Done():
			fmt.Println("Disconnecting...")
			client.Disconnect()
			return ctx.Err()
		case msg, ok := <-stream:
			if !ok {
				fmt.Println("Stream closed (simulator disconnected)")
				return nil
			}
			if msg.Err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", msg.Err)
				continue
			}

			switch types.SIMCONNECT_RECV_ID(msg.DwID) {
			case types.SIMCONNECT_RECV_ID_OPEN:
				open := msg.AsOpen()
				fmt.Printf("SimConnect: %s v%d.%d\n\n",
					engine.BytesToString(open.SzApplicationName[:]),
					open.DwApplicationVersionMajor, open.DwApplicationVersionMinor)

			case types.SIMCONNECT_RECV_ID_FACILITY_DATA:
				fd := msg.AsFacilityData()

				switch uint32(fd.UserRequestId) {
				case reqAirport:
					if fd.Type == types.SIMCONNECT_FACILITY_DATA_AIRPORT {
						data := engine.CastDataAs[AirportData](&fd.Data)
						fmt.Printf("=== Airport: %s ===\n", engine.BytesToString(data.Name64[:]))
						fmt.Printf("  ICAO:     %s\n", engine.BytesToString(data.ICAO[:]))
						fmt.Printf("  Region:   %s\n", engine.BytesToString(data.Region[:]))
						fmt.Printf("  Position: %.6f, %.6f @ %.1f ft\n", data.Latitude, data.Longitude, data.Altitude)
						fmt.Printf("  MagVar:   %.2f\n\n", data.MagVar)
					}

				case reqRunway:
					if fd.Type == types.SIMCONNECT_FACILITY_DATA_RUNWAY {
						data := engine.CastDataAs[RunwayData](&fd.Data)
						runwayCount++
						fmt.Printf("--- Runway %d ---\n", runwayCount)
						fmt.Printf("  Position: %.6f, %.6f @ %.1f ft\n", data.Latitude, data.Longitude, data.Altitude)
						fmt.Printf("  Heading:  %.1f\n", data.Heading)
						fmt.Printf("  Size:     %.0f x %.0f ft\n", data.Length, data.Width)
						fmt.Printf("  Surface:  %s\n", surfaceTypeName(data.Surface))
						fmt.Printf("  Pattern:  %.0f ft\n", data.PatternAlt)
						fmt.Printf("  Slope:    %.2f / True: %.2f\n\n", data.Slope, data.TrueSlope)
					}

				case reqParking:
					if fd.Type == types.SIMCONNECT_FACILITY_DATA_TAXI_PARKING {
						data := engine.CastDataAs[ParkingData](&fd.Data)
						if data.Number != 0 {
							parkingCount++
							fmt.Printf("  Parking %d: %s #%d  Heading=%.1f  Radius=%.1f\n",
								parkingCount, parkingTypeName(data.Type), data.Number, data.Heading, data.Radius)
						}
					}

				case reqFrequency:
					if fd.Type == types.SIMCONNECT_FACILITY_DATA_FREQUENCY {
						data := engine.CastDataAs[FrequencyData](&fd.Data)
						frequencyCount++
						// Frequency is stored as Hz integer, convert to MHz
						freqMHz := float64(data.Frequency) / 1_000_000.0
						fmt.Printf("  Frequency %d: %-12s %8.3f MHz  %s\n",
							frequencyCount, frequencyTypeName(data.Type), freqMHz,
							engine.BytesToString(data.Name[:]))
					}
				}

			case types.SIMCONNECT_RECV_ID_FACILITY_DATA_END:
				fmt.Printf("\n=== Facility data complete ===\n")
				fmt.Printf("  Runways:     %d\n", runwayCount)
				fmt.Printf("  Parking:     %d\n", parkingCount)
				fmt.Printf("  Frequencies: %d\n", frequencyCount)
				fmt.Println("\nDone. Press Ctrl+C to exit or wait for reconnection.")
			}
		}
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		fmt.Println("\nShutting down...")
		cancel()
	}()

	// Allow overriding ICAO via command-line argument
	if len(os.Args) > 1 {
		targetICAO = os.Args[1]
	}

	fmt.Printf("SimConnect Facilities Example - Querying %s\n", targetICAO)
	fmt.Println("(Press Ctrl+C to exit)")
	fmt.Println()

	for {
		err := runConnection(ctx)
		if err != nil {
			fmt.Printf("Connection ended: %v\n", err)
			return
		}
		fmt.Println("Waiting 5 seconds before reconnecting...")
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}
