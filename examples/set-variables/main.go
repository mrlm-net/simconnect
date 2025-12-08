//go:build windows
// +build windows

package main

import (
	"context"
	"fmt"
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
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	// Setup signal handler for Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		fmt.Println("🛑 Received interrupt signal, shutting down...")
		cancel()
	}()
	// This is a placeholder main function.
	// The actual implementation would go here.
	client := simconnect.New("GO Example - SimConnect Basic Connection")

	if err := client.Connect(); err != nil {
		fmt.Fprintln(os.Stderr, "❌ Error:", err)
		return
	}
	defer func() {
		if err := client.Disconnect(); err != nil {
			fmt.Fprintln(os.Stderr, "❌ Disconnect error:", err)
			return
		}

		fmt.Println("👋 Disconnected from SimConnect...")

	}()

	// Application logic would go here.
	fmt.Println("✅ Connected to SimConnect...")
	fmt.Println("⏳ Sleeping for 2 seconds...")
	time.Sleep(2 * time.Second)

	// Let's check camera state as an example
	client.AddToDataDefinition(1000, "CAMERA STATE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 0)
	client.AddToDataDefinition(1000, "CAMERA SUBSTATE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 1)
	client.RequestDataOnSimObject(1000, 1000, types.SIMCONNECT_OBJECT_ID_USER, types.SIMCONNECT_PERIOD_SECOND, 0, 0, 0, 0)

	queue := client.Stream()
	fmt.Println("✈️  Ready for takeoff!")

	client.AddToDataDefinition(2000, "CAMERA STATE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 0)

	// Before main event loop create timeout async functions to set camera state
	go func() {
		// After 5 seconds set camera to external view (3)
		time.Sleep(15 * time.Second)
		fmt.Println("🔄 Setting camera state to EXTERNAL VIEW (3)")
		value := int32(3)
		_ = client.SetDataOnSimObject(
			2000,                            // definitionID (CAMERA STATE)
			types.SIMCONNECT_OBJECT_ID_USER, // objectID
			0,                               // flags (SIMCONNECT_DATA_SET_FLAG_DEFAULT)
			1,                               // arrayCount (one int32)
			uint32(unsafe.Sizeof(value)),    // cbUnitSize (size of one int32)
			unsafe.Pointer(&value),          // pDataSet (pointer to value)
		)

		time.Sleep(5 * time.Second)
		fmt.Println("🔄 Setting camera state to COCKPIT VIEW (2)")
		value = int32(2)
		_ = client.SetDataOnSimObject(
			2000,                            // definitionID (CAMERA STATE)
			types.SIMCONNECT_OBJECT_ID_USER, // objectID
			0,                               // flags (SIMCONNECT_DATA_SET_FLAG_DEFAULT)
			1,                               // arrayCount (one int32)
			uint32(unsafe.Sizeof(value)),    // cbUnitSize (size of one int32)
			unsafe.Pointer(&value),          // pDataSet (pointer to value)
		)

		time.Sleep(5 * time.Second)
		fmt.Println("🔄 Setting camera state to DRONE VIEW (4)")
		value = int32(4)
		_ = client.SetDataOnSimObject(
			2000,                            // definitionID (CAMERA STATE)
			types.SIMCONNECT_OBJECT_ID_USER, // objectID
			0,                               // flags (SIMCONNECT_DATA_SET_FLAG_DEFAULT)
			1,                               // arrayCount (one int32)
			uint32(unsafe.Sizeof(value)),    // cbUnitSize (size of one int32)
			unsafe.Pointer(&value),          // pDataSet (pointer to value)
		)

		time.Sleep(5 * time.Second)
		fmt.Println("🔄 Setting camera state to COCKPIT VIEW (2)")
		value = int32(2)
		_ = client.SetDataOnSimObject(
			2000,                            // definitionID (CAMERA STATE)
			types.SIMCONNECT_OBJECT_ID_USER, // objectID
			0,                               // flags (SIMCONNECT_DATA_SET_FLAG_DEFAULT)
			1,                               // arrayCount (one int32)
			uint32(unsafe.Sizeof(value)),    // cbUnitSize (size of one int32)
			unsafe.Pointer(&value),          // pDataSet (pointer to value)
		)

		time.Sleep(5 * time.Second)
		fmt.Println("🔄 Setting camera state to EXTERNAL VIEW (3)")
		value = int32(3)
		_ = client.SetDataOnSimObject(
			2000,                            // definitionID (CAMERA STATE)
			types.SIMCONNECT_OBJECT_ID_USER, // objectID
			0,                               // flags (SIMCONNECT_DATA_SET_FLAG_DEFAULT)
			1,                               // arrayCount (one int32)
			uint32(unsafe.Sizeof(value)),    // cbUnitSize (size of one int32)
			unsafe.Pointer(&value),          // pDataSet (pointer to value)
		)

		time.Sleep(15 * time.Second)
		fmt.Println("🛑 Finished setting camera states, exiting...")
		cancel()
	}()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("🔌 Context cancelled, disconnecting...")
			if err := client.Disconnect(); err != nil {
				fmt.Fprintf(os.Stderr, "❌ Disconnect error: %v\n", err)
			}
			return
		case msg, ok := <-queue:
			if !ok {
				fmt.Println("📴 Stream closed (simulator disconnected)")
				return
			}

			if msg.Err != nil {
				fmt.Fprintf(os.Stderr, "❌ Error: %v\n", msg.Err)
				continue
			}

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
				cameraData := engine.CastDataAs[CameraData](&simObjData.DwData)
				fmt.Printf("%+v\n", cameraData)
			}

		}

	}
}
