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


// Timestamp is a Zenoh timestamp
type Timestamp = C.z_timestamp_t

// ClockID returns the clock id of a Timestamp
func (ts *Timestamp) ClockID() [16]byte {
	return *(*[16]byte)(unsafe.Pointer(&ts.clock_id))
}

// Time returns the time of a Timestamp
func (ts *Timestamp) Time() uint64 {
	return uint64(ts.time)
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

// String returns the Timestamp as a string
func (ts *Timestamp) String() string {
	clk := ts.ClockID()
	s := ts.GoTime().In(time.UTC).Format(time.RFC3339Nano) + "/" + hex.EncodeToString(clk[:])
	return s
}

