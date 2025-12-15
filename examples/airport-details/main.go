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
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

type AirportData struct {
	Latitude  float64
	Longitude float64
	Altitude  float64
	ICAO      [8]byte
	Name      [32]byte
	Name64    [64]byte
}

type ParkingPlace struct {
	Name             uint32
	Number           uint32
	Heading          float32
	Type             uint32
	BiasX            float32
	BiasZ            float32
	NumberOfAirlines uint32
}

type ParkingPlaces []ParkingPlace

func (pp *ParkingPlaces) Get(index int) *ParkingPlace {
	return &(*pp)[index]
}

type TaxiPath struct {
	Type      uint32
	Start     uint32
	End       uint32
	NameIndex uint32
}

type TaxiPaths []TaxiPath

func (tp *TaxiPaths) Get(index int) *TaxiPath {
	return &(*tp)[index]
}

type TaxiName struct {
	Name [32]byte
}

type TaxiNames []TaxiName

func (tn *TaxiNames) Get(index int) *TaxiName {
	return &(*tn)[index]
}

type TaxiPoint struct {
	Type        uint32
	Orientation uint32
	BiasX       float32
	BiasZ       float32
}

type TaxiPoints []TaxiPoint

func (tp *TaxiPoints) Get(index int) *TaxiPoint {
	return &(*tp)[index]
}

type Waypoint struct {
	Latitude   float64
	Longitude  float64
	Altitude   float64
	Type       uint32
	ICAO       [8]byte
	IsTerminal uint32
}

type Waypoints []Waypoint

func (wp *Waypoints) Get(index int) *Waypoint {
	return &(*wp)[index]
}

// runConnection handles a single connection lifecycle to the simulator.
// Returns nil when the simulator disconnects (allowing reconnection),
// or an error if cancelled via context.
func runConnection(ctx context.Context) error {
	// Initialize client with context
	client := simconnect.NewClient("GO Example - SimConnect Read facility and its data",
		engine.WithContext(ctx),
	)

	// Retry connection until simulator is running
	fmt.Println("⏳ Waiting for simulator to start...")
	for {
		select {
		case <-ctx.Done():
			fmt.Println("🛑 Cancelled while waiting for simulator")
			return ctx.Err()
		default:
			if err := client.Connect(); err != nil {
				fmt.Printf("🔄 Connection attempt failed: %v, retrying in 2 seconds...\n", err)
				time.Sleep(2 * time.Second)
				continue
			}
			goto connected
		}
	}

connected:
	fmt.Println("✅ Connected to SimConnect, listening for messages...")
	// We can already register data definitions and requests here

	// See remarks in docs for open/close usage
	// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Facilities/SimConnect_AddToFacilityDefinition.htm#remarks
	client.AddToFacilityDefinition(3000, "OPEN AIRPORT")
	client.AddToFacilityDefinition(3000, "LATITUDE")
	client.AddToFacilityDefinition(3000, "LONGITUDE")
	client.AddToFacilityDefinition(3000, "ALTITUDE")
	client.AddToFacilityDefinition(3000, "ICAO")
	client.AddToFacilityDefinition(3000, "NAME")
	client.AddToFacilityDefinition(3000, "NAME64")
	client.AddToFacilityDefinition(3000, "CLOSE AIRPORT")

	client.AddToFacilityDefinition(3001, "OPEN AIRPORT")
	client.AddToFacilityDefinition(3001, "OPEN TAXI_PARKING")
	client.AddToFacilityDefinition(3001, "NAME")
	client.AddToFacilityDefinition(3001, "NUMBER")
	client.AddToFacilityDefinition(3001, "HEADING")
	client.AddToFacilityDefinition(3001, "TYPE")
	client.AddToFacilityDefinition(3001, "BIAS_X")
	client.AddToFacilityDefinition(3001, "BIAS_Z")
	client.AddToFacilityDefinition(3001, "N_AIRLINES")
	client.AddToFacilityDefinition(3001, "CLOSE TAXI_PARKING")
	client.AddToFacilityDefinition(3001, "CLOSE AIRPORT")

	client.AddToFacilityDefinition(3002, "OPEN AIRPORT")
	client.AddToFacilityDefinition(3002, "OPEN TAXI_PATH")
	client.AddToFacilityDefinition(3002, "TYPE")
	client.AddToFacilityDefinition(3002, "START")
	client.AddToFacilityDefinition(3002, "END")
	client.AddToFacilityDefinition(3002, "NAME_INDEX")
	client.AddToFacilityDefinition(3002, "CLOSE TAXI_PATH")
	client.AddToFacilityDefinition(3002, "CLOSE AIRPORT")

	client.AddToFacilityDefinition(3003, "OPEN AIRPORT")
	client.AddToFacilityDefinition(3003, "OPEN TAXI_NAME")
	client.AddToFacilityDefinition(3003, "NAME")
	client.AddToFacilityDefinition(3003, "CLOSE TAXI_NAME")
	client.AddToFacilityDefinition(3003, "CLOSE AIRPORT")

	client.AddToFacilityDefinition(3004, "OPEN AIRPORT")
	client.AddToFacilityDefinition(3004, "OPEN TAXI_POINT")
	client.AddToFacilityDefinition(3004, "TYPE")
	client.AddToFacilityDefinition(3004, "ORIENTATION")
	client.AddToFacilityDefinition(3004, "BIAS_X")
	client.AddToFacilityDefinition(3004, "BIAS_Z")
	client.AddToFacilityDefinition(3004, "CLOSE TAXI_POINT")
	client.AddToFacilityDefinition(3004, "CLOSE AIRPORT")

	client.AddToFacilityDefinition(3005, "OPEN WAYPOINT")
	client.AddToFacilityDefinition(3005, "LATITUDE")
	client.AddToFacilityDefinition(3005, "LONGITUDE")
	client.AddToFacilityDefinition(3005, "ALTITUDE")
	client.AddToFacilityDefinition(3005, "TYPE")
	client.AddToFacilityDefinition(3005, "ICAO")
	//client.AddToFacilityDefinition(3005, "IS_TERMINAL_WPT")
	client.AddToFacilityDefinition(3005, "CLOSE WAYPOINT")

	//client.RequestFacilityData(3005, 128, "ED5V6", "")

	client.RequestFacilityData(3000, 123, "LKPR", "")
	client.RequestFacilityData(3001, 124, "LKPR", "")
	client.RequestFacilityData(3002, 125, "LKPR", "")
	client.RequestFacilityData(3003, 126, "LKPR", "")
	client.RequestFacilityData(3004, 127, "LKPR", "")

	var airport AirportData
	// Container for storing parking places
	var parkingPlaces ParkingPlaces

	var taxiPaths TaxiPaths

	var taxiNames TaxiNames

	var taxiPoints TaxiPoints

	var waypoints Waypoints

	// Wait for SIMCONNECT_RECV_ID_OPEN message to confirm connection is ready
	stream := client.Stream()
	// Main message processing loop
	for {
		select {
		case <-ctx.Done():
			fmt.Println("🔌 Context cancelled, disconnecting...")
			if err := client.Disconnect(); err != nil {
				fmt.Fprintf(os.Stderr, "❌ Disconnect error: %v\n", err)
			}
			//fmt.Println("Disconnected from SimConnect")
			return ctx.Err()
		case msg, ok := <-stream:
			if !ok {
				fmt.Println("📴 Stream closed (simulator disconnected)")
				return nil // Return nil to allow reconnection
			}

			if msg.Err != nil {
				fmt.Fprintf(os.Stderr, "❌ Error: %v\n", msg.Err)
				continue
			}

			fmt.Println("📨 Message received - ", types.SIMCONNECT_RECV_ID(msg.SIMCONNECT_RECV.DwID))

			//fmt.Printf("📨 Message received - ID: %d, Size: %d bytes\n", msg, msg.Size)

			// Handle specific messages
			// This could be done based on type and also if needed request IDs
			switch types.SIMCONNECT_RECV_ID(msg.DwID) {
			case types.SIMCONNECT_RECV_ID_OPEN:
				fmt.Println("🟢 Connection ready (SIMCONNECT_RECV_ID_OPEN received)")
				msg := msg.AsOpen()
				fmt.Println("📡 Received SIMCONNECT_RECV_OPEN message!")
				fmt.Printf("  Application Name: '%s'\n", engine.BytesToString(msg.SzApplicationName[:]))
				fmt.Printf("  Application Version: %d.%d\n", msg.DwApplicationVersionMajor, msg.DwApplicationVersionMinor)
				fmt.Printf("  Application Build: %d.%d\n", msg.DwApplicationBuildMajor, msg.DwApplicationBuildMinor)
				fmt.Printf("  SimConnect Version: %d.%d\n", msg.DwSimConnectVersionMajor, msg.DwSimConnectVersionMinor)
				fmt.Printf("  SimConnect Build: %d.%d\n", msg.DwSimConnectBuildMajor, msg.DwSimConnectBuildMinor)

			case types.SIMCONNECT_RECV_ID_FACILITY_DATA:
				fmt.Println("🏗️  Received SIMCONNECT_RECV_ID_FACILITY_DATA message!")
				msg := msg.AsFacilityData()

				fmt.Printf("  UserRequestId: %d\n", msg.UserRequestId)
				fmt.Printf("  UniqueRequestId: %d\n", msg.UniqueRequestId)
				fmt.Printf("  ParentUniqueRequestId: %d\n", msg.ParentUniqueRequestId)
				fmt.Printf("  Type: %d\n", msg.Type)
				fmt.Printf("  IsListItem: %v\n", msg.IsListItem)
				fmt.Printf("  ItemIndex: %d\n", msg.ItemIndex)
				fmt.Printf("  ListSize: %d\n", msg.ListSize)
				// Buffer of data. Have to cast it to a struct which matches the definition.
				switch msg.UserRequestId {
				case 123:
					fmt.Println("  Facility Data Type: Airport")
					data := engine.CastDataAs[AirportData](&msg.Data)
					fmt.Printf("  Data:\n")
					fmt.Printf("    Latitude: %f\n", data.Latitude)
					fmt.Printf("    Longitude: %f\n", data.Longitude)
					fmt.Printf("    Altitude: %f\n", data.Altitude)
					fmt.Printf("    ICAO: '%s'\n", engine.BytesToString(data.ICAO[:]))
					fmt.Printf("    Name: '%s'\n", engine.BytesToString(data.Name[:]))
					fmt.Printf("    Name64: '%s'\n", engine.BytesToString(data.Name64[:]))
					airport = *data
				case 124:
					fmt.Println("  Facility Data Type: Parking Place")
					data := engine.CastDataAs[ParkingPlace](&msg.Data)
					// We don't want empty parking places (Number==0)
					if data.Number != 0 {
						parkingPlaces = append(parkingPlaces, *data)
					}
				case 125:
					fmt.Println("  Facility Data Type: Taxi Path")
					data := engine.CastDataAs[TaxiPath](&msg.Data)
					// Handle taxi path data if needed
					taxiPaths = append(taxiPaths, *data)
				case 126:
					fmt.Println("  Facility Data Type: Taxi Name")
					data := engine.CastDataAs[TaxiName](&msg.Data)
					// Handle taxi name data if needed
					taxiNames = append(taxiNames, *data)
				case 127:
					fmt.Println("  Facility Data Type: Taxi Point")
					data := engine.CastDataAs[TaxiPoint](&msg.Data)
					// Handle taxi point data if needed
					taxiPoints = append(taxiPoints, *data)
				case 128:
					fmt.Println("  Facility Data Type: Waypoint")
					data := engine.CastDataAs[Waypoint](&msg.Data)
					// Handle waypoint data if needed
					waypoints = append(waypoints, *data)
				}

			case types.SIMCONNECT_RECV_ID_FACILITY_DATA_END:
				fmt.Println("🏁 Received SIMCONNECT_RECV_ID_FACILITY_DATA_END message!")
				fmt.Printf("Airport ref: lat=%f lon=%f\n", airport.Latitude, airport.Longitude)

				// Display count of received resources
				fmt.Printf("Total Parking Places received: %d\n", len(parkingPlaces))
				fmt.Printf("Total Taxi Paths received: %d\n", len(taxiPaths))
				fmt.Printf("Total Taxi Names received: %d\n", len(taxiNames))
				fmt.Printf("Total Taxi Points received: %d\n", len(taxiPoints))
				fmt.Printf("Total Waypoints received: %d\n", len(waypoints))

				/*for i, place := range parkingPlaces {
					//heading := float64(place.Heading)

					lat, lon := convert.OffsetToLatLon(airport.Latitude, airport.Longitude, float64(place.BiasX), float64(place.BiasZ))

					fmt.Printf("🅿️  Parking %d: Name=%d, Number=%d, Heading=%f, BiasX=%f, BiasZ=%f, N_Airlines=%d, Lat=%f, Lon=%f\n",
						i+1, int32(place.Name), place.Number, place.Heading, place.BiasX, place.BiasZ, place.NumberOfAirlines, lat, lon)

				}*/

				/*for i, path := range taxiPaths {
					fmt.Printf("🚖 Taxi Path %d: Type=%d, Start=%d, End=%d\n",
						i+1, path.Type, path.Start, path.End)
				}*/

				/*for i, tname := range taxiNames {
					fmt.Printf("🚖 Taxi Name %d: Name='%s'\n",
						i+1, engine.BytesToString(tname.Name[:]))
				}*/

			default:
				// Other message types can be handled here
			}
		}
	}
}

func main() {
	// Create cancellable context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())

	// Setup signal handler for Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		fmt.Println("🛑 Received interrupt signal, shutting down...")
		cancel()
	}()

	fmt.Println("ℹ️  (Press Ctrl+C to exit)")

	// Reconnection loop - keeps trying to connect when simulator disconnects
	for {
		err := runConnection(ctx)
		if err != nil {
			// Context cancelled (Ctrl+C) - exit completely
			fmt.Printf("⚠️  Connection ended: %v\n", err)
			return
		}

		// Simulator disconnected (err == nil) - wait and retry
		fmt.Println("⏳ Waiting 5 seconds before reconnecting...")
		select {
		case <-ctx.Done():
			fmt.Println("🛑 Shutdown requested, not reconnecting")
			return
		case <-time.After(5 * time.Second):
			fmt.Println("🔄 Attempting to reconnect...")
		}
	}
}
