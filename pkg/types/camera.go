//go:build windows
// +build windows

package types

import (
	"encoding/binary"
	"math"
)

// The add-on camera API (MSFS 2024 SDK 1.7): an add-on acquires the camera,
// then places it — by position and rotation, or by position and a point to
// look at — relative to the world, a SimObject or the pilot's eyepoint.

// SIMCONNECT_POSITION_REFERENTIAL is what a camera position or rotation is
// relative to.
//
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_POSITION_REFERENTIAL.htm
type SIMCONNECT_POSITION_REFERENTIAL DWORD

const (
	SIMCONNECT_POSITION_REFERENTIAL_NONE            SIMCONNECT_POSITION_REFERENTIAL = iota // not to be used
	SIMCONNECT_POSITION_REFERENTIAL_SIMOBJECT                                              // offset from a SimObject, meters
	SIMCONNECT_POSITION_REFERENTIAL_WORLD                                                  // latitude, longitude and altitude
	SIMCONNECT_POSITION_REFERENTIAL_EYEPOINT                                               // offset from the pilot eyepoint, meters
	SIMCONNECT_POSITION_REFERENTIAL_SIMOBJECT_DATUM                                        // offset from a SimObject's datum reference point, meters
)

// SIMCONNECT_CAMERA_DATA_MASK says which members of a SIMCONNECT_DATA_CAMERA
// SimConnect_CameraSet applies.
type SIMCONNECT_CAMERA_DATA_MASK DWORD

const (
	SIMCONNECT_CAMERA_DATA_MASK_NONE         SIMCONNECT_CAMERA_DATA_MASK = 0
	SIMCONNECT_CAMERA_DATA_MASK_POSITION     SIMCONNECT_CAMERA_DATA_MASK = 1 << 0
	SIMCONNECT_CAMERA_DATA_MASK_ROTATION     SIMCONNECT_CAMERA_DATA_MASK = 1 << 1
	SIMCONNECT_CAMERA_DATA_MASK_TARGETED     SIMCONNECT_CAMERA_DATA_MASK = 1 << 2
	SIMCONNECT_CAMERA_DATA_MASK_FOV          SIMCONNECT_CAMERA_DATA_MASK = 1 << 3
	SIMCONNECT_CAMERA_DATA_MASK_ALL_ROTATION                             = SIMCONNECT_CAMERA_DATA_MASK_POSITION | SIMCONNECT_CAMERA_DATA_MASK_ROTATION | SIMCONNECT_CAMERA_DATA_MASK_FOV
	SIMCONNECT_CAMERA_DATA_MASK_ALL_TARGETED                             = SIMCONNECT_CAMERA_DATA_MASK_POSITION | SIMCONNECT_CAMERA_DATA_MASK_TARGETED | SIMCONNECT_CAMERA_DATA_MASK_FOV
)

// SIMCONNECT_CAMERA_AVAILABILITY is the add-on camera's state in a
// SIMCONNECT_RECV_CAMERA_STATUS.
type SIMCONNECT_CAMERA_AVAILABILITY DWORD

const (
	SIMCONNECT_CAMERA_NOT_ACQUIRED SIMCONNECT_CAMERA_AVAILABILITY = iota
	SIMCONNECT_CAMERA_ACQUIRED
	SIMCONNECT_CAMERA_ACQUIRED_BY_OTHER
	SIMCONNECT_CAMERA_USER_DISABLED
)

// SIMCONNECT_CAMERA_FLAG are the flags of SimConnect_CameraEnableFlag and
// SimConnect_CameraDisableFlag.
type SIMCONNECT_CAMERA_FLAG DWORD

const (
	SIMCONNECT_CAMERA_FLAG_INTERACTION  SIMCONNECT_CAMERA_FLAG = 0x01
	SIMCONNECT_CAMERA_FLAG_ABOVE_GROUND SIMCONNECT_CAMERA_FLAG = 0x02
)

// SIMCONNECT_CAMERA_WORLD_LOCKER_STATUS is the state of a camera world
// locker request.
type SIMCONNECT_CAMERA_WORLD_LOCKER_STATUS DWORD

const (
	SIMCONNECT_CAMERA_WORLD_LOCKER_STATUS_NONE SIMCONNECT_CAMERA_WORLD_LOCKER_STATUS = iota
	SIMCONNECT_CAMERA_WORLD_LOCKER_STATUS_START
	SIMCONNECT_CAMERA_WORLD_LOCKER_STATUS_SUCCESS
	SIMCONNECT_CAMERA_WORLD_LOCKER_STATUS_CANCEL
	SIMCONNECT_CAMERA_WORLD_LOCKER_STATUS_FAIL
)

// CameraPBH is the camera rotation: pitch, bank and heading in degrees, as
// the SDK header has SIMCONNECT_DATA_PBH (floats; SIMCONNECT_DATA_PBH in
// this package has doubles).
type CameraPBH struct {
	Pitch, Bank, Heading float32
}

// SIMCONNECT_DATA_CAMERA places the add-on camera. SimConnect packs it to
// one byte (84 bytes), so it crosses the API as bytes: Bytes and
// CameraDataFrom.
//
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_DATA_CAMERA.htm
type SIMCONNECT_DATA_CAMERA struct {
	Position                    SIMCONNECT_DATA_XYZ
	PositionReferential         SIMCONNECT_POSITION_REFERENTIAL
	PositionReferentialObjectID DWORD // 0: the user aircraft; ignored for the world
	TargetedPos                 SIMCONNECT_DATA_XYZ
	Pbh                         CameraPBH
	RotationReferential         SIMCONNECT_POSITION_REFERENTIAL
	RotationReferentialObjectID DWORD
	Fov                         float64 // radians
}

// SimConnectCameraDataSize is the packed size of SIMCONNECT_DATA_CAMERA.
const SimConnectCameraDataSize = 84

// SimConnectCameraIgnoreField leaves a member of the camera as it is
// (SIMCONNECT_CAMERA_IGNORE_FIELD, FLT_MAX).
const SimConnectCameraIgnoreField = math.MaxFloat32

// Bytes is the camera as SimConnect lays it out (packed, little-endian).
func (c SIMCONNECT_DATA_CAMERA) Bytes() [SimConnectCameraDataSize]byte {
	var b [SimConnectCameraDataSize]byte
	le := binary.LittleEndian
	f64 := func(at int, v float64) { le.PutUint64(b[at:], math.Float64bits(v)) }
	f32 := func(at int, v float32) { le.PutUint32(b[at:], math.Float32bits(v)) }
	f64(0, c.Position.X)
	f64(8, c.Position.Y)
	f64(16, c.Position.Z)
	le.PutUint32(b[24:], uint32(c.PositionReferential))
	le.PutUint32(b[28:], uint32(c.PositionReferentialObjectID))
	f64(32, c.TargetedPos.X)
	f64(40, c.TargetedPos.Y)
	f64(48, c.TargetedPos.Z)
	f32(56, c.Pbh.Pitch)
	f32(60, c.Pbh.Bank)
	f32(64, c.Pbh.Heading)
	le.PutUint32(b[68:], uint32(c.RotationReferential))
	le.PutUint32(b[72:], uint32(c.RotationReferentialObjectID))
	f64(76, c.Fov)
	return b
}

// CameraDataFrom reads a packed SIMCONNECT_DATA_CAMERA.
func CameraDataFrom(b []byte) (SIMCONNECT_DATA_CAMERA, bool) {
	if len(b) < SimConnectCameraDataSize {
		return SIMCONNECT_DATA_CAMERA{}, false
	}
	le := binary.LittleEndian
	f64 := func(at int) float64 { return math.Float64frombits(le.Uint64(b[at:])) }
	f32 := func(at int) float32 { return math.Float32frombits(le.Uint32(b[at:])) }
	return SIMCONNECT_DATA_CAMERA{
		Position:                    SIMCONNECT_DATA_XYZ{f64(0), f64(8), f64(16)},
		PositionReferential:         SIMCONNECT_POSITION_REFERENTIAL(le.Uint32(b[24:])),
		PositionReferentialObjectID: DWORD(le.Uint32(b[28:])),
		TargetedPos:                 SIMCONNECT_DATA_XYZ{f64(32), f64(40), f64(48)},
		Pbh:                         CameraPBH{f32(56), f32(60), f32(64)},
		RotationReferential:         SIMCONNECT_POSITION_REFERENTIAL(le.Uint32(b[68:])),
		RotationReferentialObjectID: DWORD(le.Uint32(b[72:])),
		Fov:                         f64(76),
	}, true
}

// SIMCONNECT_RECV_CAMERA_STATUS is the add-on camera's state, after
// SimConnect_CameraAcquire, SimConnect_CameraGetStatus or while subscribed.
type SIMCONNECT_RECV_CAMERA_STATUS struct {
	SIMCONNECT_RECV
	AcquiredState  SIMCONNECT_CAMERA_AVAILABILITY
	GameControlled int32 // BOOL: the simulation still controls it
}

// SIMCONNECT_RECV_CAMERA_WORLD_LOCKER is the state of a world locker
// request.
type SIMCONNECT_RECV_CAMERA_WORLD_LOCKER struct {
	SIMCONNECT_RECV
	Status SIMCONNECT_CAMERA_WORLD_LOCKER_STATUS
}

// SimConnectCameraDefinitionSize is the size of one camera definition name
// in a SIMCONNECT_RECV_CAMERA_DEFINITION_LIST.
const SimConnectCameraDefinitionSize = 256
