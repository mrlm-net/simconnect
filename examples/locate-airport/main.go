//go:build windows
// +build windows

package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/signal"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// CameraData represents the data structure for CAMERA STATE and CAMERA SUBSTATE
// The fields must match the order of AddToDataDefinition calls
type CameraData struct {
	CameraState    int32
	CameraSubstate int32
	GPSPositionAlt float64
	GPSPositionLat float64
	GPSPositionLon float64
}

// runConnection handles a single connection lifecycle to the simulator.
// Returns nil when the simulator disconnects (allowing reconnection),
// or an error if cancelled via context.
func runConnection(ctx context.Context) error {
	// Initialize client with context
	client := simconnect.NewClient("GO Example - SimConnect Read Messages and their data",
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

	// --------------------------------------------
	// - Define data structure for CAMERA STATE and CAMERA SUBSTATE
	//   and request updates every second
	// --------------------------------------------
	client.AddToDataDefinition(2000, "CAMERA STATE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 0)
	client.AddToDataDefinition(2000, "CAMERA SUBSTATE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 1)
	client.AddToDataDefinition(2000, "PLANE ALTITUDE", "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 2)
	client.AddToDataDefinition(2000, "PLANE LATITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 3)
	client.AddToDataDefinition(2000, "PLANE LONGITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 4)
	client.RequestDataOnSimObject(2001, 2000, types.SIMCONNECT_OBJECT_ID_USER, types.SIMCONNECT_PERIOD_SECOND, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0)

	client.AddToDataDefinition(3000, "TITLE", "", types.SIMCONNECT_DATATYPE_STRING128, 0, 0)
	client.AddToDataDefinition(3000, "LIVERY NAME", "", types.SIMCONNECT_DATATYPE_STRING128, 0, 1)
	client.AddToDataDefinition(3000, "LIVERY FOLDER", "", types.SIMCONNECT_DATATYPE_STRING128, 0, 2)
	client.AddToDataDefinition(3000, "PLANE LATITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 3)
	client.AddToDataDefinition(3000, "PLANE LONGITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 4)
	client.AddToDataDefinition(3000, "PLANE ALTITUDE", "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 5)
	client.AddToDataDefinition(3000, "PLANE HEADING DEGREES TRUE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 6)
	client.AddToDataDefinition(3000, "PLANE HEADING DEGREES MAGNETIC", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 7)
	client.AddToDataDefinition(3000, "VERTICAL SPEED", "feet per second", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 8)
	client.AddToDataDefinition(3000, "PLANE PITCH DEGREES", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 9)
	client.AddToDataDefinition(3000, "PLANE BANK DEGREES", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 10)
	client.AddToDataDefinition(3000, "GROUND VELOCITY", "knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 11)
	client.AddToDataDefinition(3000, "AIRSPEED INDICATED", "knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 12)
	client.AddToDataDefinition(3000, "AIRSPEED TRUE", "knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 13)
	client.AddToDataDefinition(3000, "ON ANY RUNWAY", "bool", types.SIMCONNECT_DATATYPE_INT32, 0, 14)
	client.AddToDataDefinition(3000, "SURFACE TYPE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 15)
	client.AddToDataDefinition(3000, "SIM ON GROUND", "bool", types.SIMCONNECT_DATATYPE_INT32, 0, 16)
	client.AddToDataDefinition(3000, "ATC ID", "", types.SIMCONNECT_DATATYPE_STRING32, 0, 17)
	client.AddToDataDefinition(3000, "ATC AIRLINE", "", types.SIMCONNECT_DATATYPE_STRING32, 0, 18)

	// Request data for all aircraft within 10km radius
	client.RequestDataOnSimObjectType(4001, 3000, 10000, types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT)

	// Wait for SIMCONNECT_RECV_ID_OPEN message to confirm connection is ready
	stream := client.Stream()
	// Track user position and ensure we only request the list once
	var myLatitude float64
	var myLongitude float64
	requestedClosest := false

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
			case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
				fmt.Println("  => Received SimObject data event")
				simObjData := msg.AsSimObjectData()
				fmt.Printf("     Request ID: %d, Define ID: %d, Object ID: %d, Flags: %d, Out of: %d, DefineCount: %d\n",
					simObjData.DwRequestID,
					simObjData.DwDefineID,
					simObjData.DwObjectID,
					simObjData.DwFlags,
					simObjData.DwOutOf,
					simObjData.DwDefineCount,
				)
				// Cast the data pointer to CameraData struct
				// The DwData field is the start of the actual data block
				if simObjData.DwDefineID == 2000 {

					cameraData := engine.CastDataAs[CameraData](&simObjData.DwData)
					fmt.Printf("     Camera State: %d, Camera Substate: %d, GPS Position Alt: %f, GPS Position Lat: %f, GPS Position Lon: %f \n",
						cameraData.CameraState,
						cameraData.CameraSubstate,
						cameraData.GPSPositionAlt,
						cameraData.GPSPositionLat,
						cameraData.GPSPositionLon,
					)

					// store user position and request airport list once
					myLatitude = cameraData.GPSPositionLat
					myLongitude = cameraData.GPSPositionLon
					if !requestedClosest {
						// Request airports in reality bubble; use definition id 5000
						if err := client.RequestFacilitiesListEX1(5000, types.SIMCONNECT_FACILITY_LIST_AIRPORT); err != nil {
							fmt.Fprintf(os.Stderr, "❌ RequestFacilitiesListEX1 failed: %v\n", err)
						} else {
							fmt.Println("🔎 Requested airport list (for closest-airport search)")
							requestedClosest = true
						}
					}
				}
			case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA_BYTYPE:
				simObjData := msg.AsSimObjectDataBType()
				fmt.Printf("     Request ID: %d, Define ID: %d, Object ID: %d, Flags: %d, Out of: %d, DefineCount: %d\n",
					simObjData.DwRequestID,
					simObjData.DwDefineID,
					simObjData.DwObjectID,
					simObjData.DwFlags,
					simObjData.DwOutOf,
					simObjData.DwDefineCount,
				)
				if simObjData.DwDefineID == 3000 {
					aircraftData := engine.CastDataAs[struct {
						Title             [128]byte
						LiveryName        [128]byte
						LiveryFolder      [128]byte
						Lat               float64
						Lon               float64
						Alt               float64
						Head              float64
						HeadMag           float64
						Vs                float64
						Pitch             float64
						Bank              float64
						GroundSpeed       float64
						AirspeedIndicated float64
						AirspeedTrue      float64
						OnAnyRunway       int32
						SurfaceType       int32
						SimOnGround       int32
						AtcID             [32]byte
						AtcAirline        [32]byte
					}](&simObjData.DwData)
					fmt.Printf("     Aircraft Title: %s, Livery Name: %s, Livery Folder: %s, Lat: %f, Lon: %f, Alt: %f, Head: %f, HeadMag: %f, VS: %f, Pitch: %f, Bank: %f, GroundSpeed: %f, AirspeedIndicated: %f, AirspeedTrue: %f, OnAnyRunway: %d, SurfaceType: %d, SimOnGround: %d, AtcID: %s\n",
						engine.BytesToString(aircraftData.Title[:]),
						engine.BytesToString(aircraftData.LiveryName[:]),
						engine.BytesToString(aircraftData.LiveryFolder[:]),
						aircraftData.Lat,
						aircraftData.Lon,
						aircraftData.Alt,
						aircraftData.Head,
						aircraftData.HeadMag,
						aircraftData.Vs,
						aircraftData.Pitch,
						aircraftData.Bank,
						aircraftData.GroundSpeed,
						aircraftData.AirspeedIndicated,
						aircraftData.AirspeedTrue,
						aircraftData.OnAnyRunway,
						aircraftData.SurfaceType,
						aircraftData.SimOnGround,
						engine.BytesToString(aircraftData.AtcID[:]),
					)
				}
			case types.SIMCONNECT_RECV_ID_AIRPORT_LIST:
				list := msg.AsAirportList()

				fmt.Printf("🏢 Received facility list (airport list): RequestID=%d, ArraySize=%d, Packet=%d of %d\n",
					list.DwRequestID, list.DwArraySize, list.DwEntryNumber, list.DwOutOf)

				if list.DwArraySize == 0 {
					fmt.Println("  No airports in this message")
					continue
				}

				// Calculate actual entry size from the message
				headerSize := unsafe.Sizeof(types.SIMCONNECT_RECV_FACILITIES_LIST{})
				actualDataSize := uintptr(msg.DwSize) - headerSize
				actualEntrySize := actualDataSize / uintptr(list.DwArraySize)

				// dataStart points to the beginning of the array data (after the header)
				dataStart := unsafe.Pointer(uintptr(unsafe.Pointer(list)) + headerSize)

				closestDistance := math.MaxFloat64
				var closestIdent string

				for i := uint32(0); i < uint32(list.DwArraySize); i++ {
					entryOffset := uintptr(i) * actualEntrySize
					entryPtr := unsafe.Pointer(uintptr(dataStart) + entryOffset)

					var ident [6]byte
					copy(ident[:], (*[6]byte)(entryPtr)[:])

					lat := *(*float64)(unsafe.Pointer(uintptr(entryPtr) + 12))
					lon := *(*float64)(unsafe.Pointer(uintptr(entryPtr) + 20))

					// Euclidean on lat/lon degrees (sufficient for nearby search)
					distance := math.Sqrt(math.Pow(lat-myLatitude, 2) + math.Pow(lon-myLongitude, 2))
					if distance < closestDistance {
						closestDistance = distance
						closestIdent = engine.BytesToString(ident[:])
					}
				}

				if closestIdent != "" {
					fmt.Printf("✅ Closest airport Ident: %s (approx distance: %f degrees)\n", closestIdent, closestDistance)
				}
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
