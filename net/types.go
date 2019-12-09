package net

/*
#include <zenoh.h>

// Indirection since Go cannot call a C function pointer (z_replies_sender_t)
inline void call_replies_sender(z_replies_sender_t send_replies, void *query_handle, z_array_p_resource_t *replies) {
	send_replies(query_handle, *replies);
}
*/
import "C"
import (
	"strconv"
	"time"
	"encoding/hex"
	"unsafe"
)

const (
	// InfoPidKey is the key for the PID value in the properties
	// map returned by the Info() operation.
	InfoPidKey = C.Z_INFO_PID_KEY

	// InfoPeerKey is the key for the peer value in the properties
	// map returned by the Info() operation.
	InfoPeerKey = C.Z_INFO_PEER_KEY

	// InfoPeerPidKey is the key for the peer's PID value in the properties
	// map returned by the Info() operation.
	InfoPeerPidKey = C.Z_INFO_PEER_PID_KEY

	// UserKey is the key for the (optional) user's name in the properties
	// map passed to the Login() operation.
	UserKey = C.Z_USER_KEY

	// PasswdKey is the key for the (optional) user's password in the properties
	// map passed to the Login() operation.
	PasswdKey = C.Z_PASSWD_KEY
)

// ZNError reports an error that occurred in the zenoh-c library
type ZNError struct {
	msg  string
	code int
}

// Error returns the message associated to a ZNError
func (e *ZNError) Error() string {
	return e.msg + " (error code:" + strconv.Itoa(e.code) + ")"
}


//
// Types and helpers
//

// Session is the C session type
type Session = C.z_zenoh_t

// Subscriber is a Zenoh subscriber
type Subscriber struct {
	regIndex int
	zsub     *C.z_sub_t
}

// Publisher is a Zenoh publisher
type Publisher = C.z_pub_t

// Storage is a Zenoh storage
type Storage struct {
	regIndex int
	zsto     *C.z_sto_t
}

// Eval is a Zenoh eval
type Eval struct {
	regIndex int
	zeval    *C.z_eva_t
}

// Resource is a Zenoh resource with a name and a value (data).
type Resource struct {
	RName    string
	Data     []byte
	Encoding uint8
	Kind     uint8
}

// RepliesSender is used in a storage's and eval's QueryHandler() implementation to send back replies to a query.
type RepliesSender struct {
	sendRepliesFunc C.z_replies_sender_t
	queryHandle     unsafe.Pointer
}

var sizeofUintptr = int(unsafe.Sizeof(uintptr(0)))

// SendReplies sends the replies to a query in a storage or eval.
// This operation should be called in the implementation of a QueryHandler
func (rs *RepliesSender) SendReplies(replies []Resource) {
	// Convert []Resource into z_array_p_resource_t
	array := new(C.z_array_p_resource_t)
	if replies == nil {
		array.length = 0
		array.elem = nil
	} else {
		nbRes := len(replies)

		var (
			cResources     = (*C.z_resource_t)(C.malloc(C.size_t(C.sizeof_z_resource_t * nbRes)))
			goResources    = (*[1 << 30]C.z_resource_t)(unsafe.Pointer(cResources))[:nbRes:nbRes]
			cResourcesPtr  = (*uintptr)(C.malloc(C.size_t(sizeofUintptr * nbRes)))
			goResourcesPtr = (*[1 << 30]uintptr)(unsafe.Pointer(cResourcesPtr))[:nbRes:nbRes]
		)
		defer C.free(unsafe.Pointer(cResources))
		defer C.free(unsafe.Pointer(cResourcesPtr))
		for i, r := range replies {
			goResources[i].rname = C.CString(r.RName)
			defer C.free(unsafe.Pointer(goResources[i].rname))
			goResources[i].data = (*C.uchar)(C.CBytes(r.Data))
			defer C.free(unsafe.Pointer(goResources[i].data))
			goResources[i].length = C.ulong(len(r.Data))
			goResources[i].encoding = C.ushort(r.Encoding)
			goResources[i].kind = C.ushort(r.Kind)

			goResourcesPtr[i] = (uintptr)(unsafe.Pointer(&goResources[i]))
		}
		array.length = C.uint(nbRes)
		array.elem = (**C.z_resource_t)(unsafe.Pointer(cResourcesPtr))

	}
	C.call_replies_sender(rs.sendRepliesFunc, rs.queryHandle, array)
}

// DataHandler will be called on reception of data matching the subscribed/stored resource.
type DataHandler func(rid string, data []byte, info *DataInfo)

// QueryHandler will be called on reception of query matching the stored/evaluated resource.
// The QueryHandler must provide the data matching the resource selection 'rname' by calling
// the 'sendReplies' function of the 'RepliesSender'. The 'sendReplies'
// function MUST be called but accepts empty data array.
type QueryHandler func(rname string, predicate string, sendReplies *RepliesSender)

// ReplyHandler will be called on reception of replies to a query.
type ReplyHandler func(reply *ReplyValue)

// SubMode is a Subscriber mode
type SubMode = C.z_sub_mode_t

// SubModeKind is the kind of a Subscriber mode
type SubModeKind = C.uint8_t

const (
	// ZPushMode : push mode for subscriber
	ZPushMode SubModeKind = iota + 1
	// ZPullMode : pull mode for subscriber
	ZPullMode SubModeKind = iota + 1
	// ZPeriodicPushMode : periodic push mode for subscriber
	ZPeriodicPushMode SubModeKind = iota + 1
	// ZPeriodicPullMode : periodic pull mode for subscriber
	ZPeriodicPullMode SubModeKind = iota + 1
)

// NewSubMode returns a SubMode with the specified kind
func NewSubMode(kind SubModeKind) SubMode {
	return SubMode{kind, C.z_temporal_property_t{0, 0, 0}}
}

// NewSubModeWithTime returns a SubMode with the specified kind and temporal properties
func NewSubModeWithTime(kind SubModeKind, origin C.ulong, period C.ulong, duration C.ulong) SubMode {
	return SubMode{kind, C.z_temporal_property_t{origin, period, duration}}
}

// QueryDest is a Query destination
type QueryDest = C.z_query_dest_t

// QueryDestKind is the kind of a Query destination
type QueryDestKind = C.uint8_t

const (
	// ZBestMatch : the nearest complete storage/eval if there is one, all storages/evals if not.
	ZBestMatch QueryDestKind = iota
	// ZComplete : only complete storages/evals.
	ZComplete QueryDestKind = iota
	// ZAll : all storages/evals.
	ZAll QueryDestKind = iota
	// ZNone : no storages/evals.
	ZNone QueryDestKind = iota
)

// NewQueryDest returns a QueryDest with the specified kind
func NewQueryDest(kind QueryDestKind) QueryDest {
	return QueryDest{kind, 1}
}

// NewQueryDestWithNb returns a QueryDest with the specified kind and nb
func NewQueryDestWithNb(kind QueryDestKind, nb C.uint8_t) QueryDest {
	return QueryDest{kind, nb}
}

// DataInfo is the information associated to a received data.
type DataInfo = C.z_data_info_t

// Flags returns the flags from a DataInfo
func (info *DataInfo) Flags() uint {
	return uint(info.flags)
}

// Tstamp returns the timestamp from a DataInfo
func (info *DataInfo) Tstamp() Timestamp {
	return info.tstamp
}

// Encoding returns the encoding from a DataInfo
func (info *DataInfo) Encoding() uint8 {
	return uint8(info.encoding)
}

// Kind returns the kind from a DataInfo
func (info *DataInfo) Kind() uint8 {
	return uint8(info.kind)
}

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

// ReplyKind is the kind of a ReplyValue
type ReplyKind = C.char

const (
	// ZStorageData : a reply with data from a storage
	ZStorageData ReplyKind = iota
	// ZStorageFinal : a final reply from a storage (without data)
	ZStorageFinal ReplyKind = iota
	// ZEvalData : a reply with data from an eval
	ZEvalData ReplyKind = iota
	// ZEvalFinal : a final reply from an eval (without data)
	ZEvalFinal ReplyKind = iota
	// ZReplyFinal : the final reply (without data)
	ZReplyFinal ReplyKind = iota
)

// ReplyValue is a reply to a query
type ReplyValue = C.z_reply_value_t

// Kind returns the Reply message kind
func (r *ReplyValue) Kind() ReplyKind {
	return ReplyKind(r.kind)
}

// SrcID returns the unique id of the storage or eval that sent this reply
func (r *ReplyValue) SrcID() []byte {
	return C.GoBytes(unsafe.Pointer(r.srcid), C.int(r.srcid_length))
}

// RSN returns the request sequence number
func (r *ReplyValue) RSN() uint64 {
	return uint64(r.rsn)
}

// RName returns the resource name of this reply
func (r *ReplyValue) RName() string {
	return C.GoString(r.rname)
}

// Data returns the data of this reply, if the Reply message kind is Z_STORAGE_DATA.
// Otherwise, it returns null
func (r *ReplyValue) Data() []byte {
	return C.GoBytes(unsafe.Pointer(r.data), C.int(r.data_length))
}

// Info returns the DataInfo associated to this reply
func (r *ReplyValue) Info() DataInfo {
	return r.info
}

