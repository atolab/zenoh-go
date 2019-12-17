package net

/*
#include <zenoh.h>

// Indirection since Go cannot call a C function pointer (zn_replies_sender_t)
inline void call_replies_sender(zn_replies_sender_t send_replies, void *query_handle, zn_resource_p_array_t *replies) {
	send_replies(query_handle, *replies);
}
*/
import "C"
import (
	"unsafe"

	zcore "github.com/atolab/zenoh-go/core"
)

const (
	// InfoPidKey is the key for the PID value in the properties
	// map returned by the Info() operation.
	InfoPidKey = C.ZN_INFO_PID_KEY

	// InfoPeerKey is the key for the peer value in the properties
	// map returned by the Info() operation.
	InfoPeerKey = C.ZN_INFO_PEER_KEY

	// InfoPeerPidKey is the key for the peer's PID value in the properties
	// map returned by the Info() operation.
	InfoPeerPidKey = C.ZN_INFO_PEER_PID_KEY

	// UserKey is the key for the (optional) user's name in the properties
	// map passed to the Login() operation.
	UserKey = C.ZN_USER_KEY

	// PasswdKey is the key for the (optional) user's password in the properties
	// map passed to the Login() operation.
	PasswdKey = C.ZN_PASSWD_KEY
)

// ZError reports an error that occurred in zenoh.
type ZError = zcore.ZError

//
// Types and helpers
//

// Timestamp is a Zenoh timestamp
type Timestamp = zcore.Timestamp

// Session is the C session type
type Session = C.zn_session_t

// Subscriber is a Zenoh subscriber
type Subscriber struct {
	regIndex int
	zsub     *C.zn_sub_t
}

// Publisher is a Zenoh publisher
type Publisher = C.zn_pub_t

// Storage is a Zenoh storage
type Storage struct {
	regIndex int
	zsto     *C.zn_sto_t
}

// Eval is a Zenoh eval
type Eval struct {
	regIndex int
	zeval    *C.zn_eva_t
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
	sendRepliesFunc C.zn_replies_sender_t
	queryHandle     unsafe.Pointer
}

var sizeofUintptr = int(unsafe.Sizeof(uintptr(0)))

// SendReplies sends the replies to a query in a storage or eval.
// This operation should be called in the implementation of a QueryHandler
func (rs *RepliesSender) SendReplies(replies []Resource) {
	// Convert []Resource into zn_resource_p_array_t
	array := new(C.zn_resource_p_array_t)
	if replies == nil {
		array.length = 0
		array.elem = nil
	} else {
		nbRes := len(replies)

		var (
			cResources     = (*C.zn_resource_t)(C.malloc(C.size_t(C.sizeof_zn_resource_t * nbRes)))
			goResources    = (*[1 << 30]C.zn_resource_t)(unsafe.Pointer(cResources))[:nbRes:nbRes]
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
		array.elem = (**C.zn_resource_t)(unsafe.Pointer(cResourcesPtr))

	}
	C.call_replies_sender(rs.sendRepliesFunc, rs.queryHandle, array)
}

// DataHandler will be called on reception of data matching the subscribed/stored resource.
type DataHandler func(rkey string, data []byte, info *DataInfo)

// QueryHandler will be called on reception of query matching the stored/evaluated resource.
// The QueryHandler must provide the data matching the resource selection 'rname' by calling
// the 'sendReplies' function of the 'RepliesSender'. The 'sendReplies'
// function MUST be called but accepts empty data array.
type QueryHandler func(rname string, predicate string, sendReplies *RepliesSender)

// ReplyHandler will be called on reception of replies to a query.
type ReplyHandler func(reply *ReplyValue)

// SubMode is a Subscriber mode
type SubMode = C.zn_sub_mode_t

// SubModeKind is the kind of a Subscriber mode
type SubModeKind = C.uint8_t

const (
	// ZNPushMode : push mode for subscriber
	ZNPushMode SubModeKind = iota + 1
	// ZNPullMode : pull mode for subscriber
	ZNPullMode SubModeKind = iota + 1
	// ZNPeriodicPushMode : periodic push mode for subscriber
	ZNPeriodicPushMode SubModeKind = iota + 1
	// ZNPeriodicPullMode : periodic pull mode for subscriber
	ZNPeriodicPullMode SubModeKind = iota + 1
)

// NewSubMode returns a SubMode with the specified kind
func NewSubMode(kind SubModeKind) SubMode {
	return SubMode{kind, C.zn_temporal_property_t{0, 0, 0}}
}

// NewSubModeWithTime returns a SubMode with the specified kind and temporal properties
func NewSubModeWithTime(kind SubModeKind, origin C.ulong, period C.ulong, duration C.ulong) SubMode {
	return SubMode{kind, C.zn_temporal_property_t{origin, period, duration}}
}

// QueryDest is a Query destination
type QueryDest = C.zn_query_dest_t

// QueryDestKind is the kind of a Query destination
type QueryDestKind = C.uint8_t

const (
	// ZNBestMatch : the nearest complete storage/eval if there is one, all storages/evals if not.
	ZNBestMatch QueryDestKind = iota
	// ZNComplete : only complete storages/evals.
	ZNComplete QueryDestKind = iota
	// ZNAll : all storages/evals.
	ZNAll QueryDestKind = iota
	// ZNNone : no storages/evals.
	ZNNone QueryDestKind = iota
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
type DataInfo = C.zn_data_info_t

// Flags returns the flags from a DataInfo
func (info *DataInfo) Flags() uint {
	return uint(info.flags)
}

// Tstamp returns the timestamp from a DataInfo
func (info *DataInfo) Tstamp() *Timestamp {
	// As CGO generates 2 different structs for C.z_timestamp_t in zenoh/core
	// and zenoh/net packages, we need to do this unsafe.Pointer conversion.
	// See https://github.com/golang/go/issues/13467
	return (*Timestamp)(unsafe.Pointer(&info.tstamp))
}

// Encoding returns the encoding from a DataInfo
func (info *DataInfo) Encoding() uint8 {
	return uint8(info.encoding)
}

// Kind returns the kind from a DataInfo
func (info *DataInfo) Kind() uint8 {
	return uint8(info.kind)
}

// ReplyKind is the kind of a ReplyValue
type ReplyKind = C.char

const (
	// ZNStorageData : a reply with data from a storage
	ZNStorageData ReplyKind = iota
	// ZNStorageFinal : a final reply from a storage (without data)
	ZNStorageFinal ReplyKind = iota
	// ZNEvalData : a reply with data from an eval
	ZNEvalData ReplyKind = iota
	// ZNEvalFinal : a final reply from an eval (without data)
	ZNEvalFinal ReplyKind = iota
	// ZNReplyFinal : the final reply (without data)
	ZNReplyFinal ReplyKind = iota
)

// ReplyValue is a reply to a query
type ReplyValue = C.zn_reply_value_t

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

// Data returns the data of this reply, if the Reply message kind is ZN_STORAGE_DATA.
// Otherwise, it returns null
func (r *ReplyValue) Data() []byte {
	return C.GoBytes(unsafe.Pointer(r.data), C.int(r.data_length))
}

// Info returns the DataInfo associated to this reply
func (r *ReplyValue) Info() DataInfo {
	return r.info
}

