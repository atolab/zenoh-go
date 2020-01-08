package core

/*
#cgo CFLAGS: -DZENOH_MACOS
#cgo LDFLAGS: -lzenohc

#define ZENOH_MACOS 1

#include <zenoh.h>
*/
import "C"
import (
	"encoding/hex"
	"time"
	"unsafe"
)


// Timestamp is a data structure representing a unique timestamp.
type Timestamp = C.z_timestamp_t


// GenerateTimestamp creates a new timestamp with current time but with 0x00 as clock_id.
// WARN: Don't use it, this is a temporary workaround.
// @TODO: remove this when we're sure Data always come with a Timestamp.
func GenerateTimestamp() *Timestamp {
	ns := time.Now().UnixNano()
	sec := C.ulong((ns / 1000000000) << 32)
	frac := C.ulong(float32((ns % 1000000000) / 1000000000) * 0x100000000)
	ts := new(Timestamp)
	ts.time = sec + frac
	ts.clock_id = [16]C.uchar{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	return ts
}

// Time returns the  time as a 64-bit long, where:
//   - The higher 32-bit represent the number of seconds
//       since midnight, January 1, 1970 UTC
//   - The lower 32-bit represent a fraction of 1 second.
func (ts *Timestamp) Time() uint64 {
	return uint64(ts.time)
}

// ClockID returns the unique identifier of the clock that generated this timestamp.
func (ts *Timestamp) ClockID() [16]byte {
	return *(*[16]byte)(unsafe.Pointer(&ts.clock_id))
}

// number of NTP fraction per second (2^32)
const fracPerSec = 0x100000000

// number of nanoseconds per second (10^9)
const nanoPerSec = 1000000000

// GoTime returns the time of a Timestamp as a Go time.Time
func (ts *Timestamp) GoTime() time.Time {
	sec := ts.time >> 32
	frac := ts.time & 0xffffffff
	ns := (frac * nanoPerSec) / fracPerSec
	return time.Unix(int64(sec), int64(ns))
}

// Before reports whether the Timestamp ts was created before ots.
// This function can be used for sorting.
func (ts *Timestamp) Before(ots *Timestamp) bool {
	if ts.time < ots.time {
		return true
	} else if ts.time > ots.time {
		return false
	} else {
		for i, b := range ts.clock_id {
			if b < ots.clock_id[i] {
				return true
			} else if b > ots.clock_id[i] {
				return false
			}
		}
	}
	return false
}

// ToString returns the Timestamp as a string
func (ts *Timestamp) ToString() string {
	clk := ts.ClockID()
	s := ts.GoTime().In(time.UTC).Format(time.RFC3339Nano) + "/" + hex.EncodeToString(clk[:])
	return s
}

