// Package zenoh provides the Zenoh client API in Go.
package zenoh

/*
#cgo CFLAGS: -DZENOH_MACOS
#cgo LDFLAGS: -lzenohc

#define ZENOH_MACOS 1

#include <stdlib.h>
#include <stdio.h>
#include <zenoh.h>
#include <zenoh/recv_loop.h>
#include <zenoh/rname.h>

// Forward declarations of callbacks (see callbacks.go)
extern void subscriber_callback_cgo(const z_resource_id_t *rid, const unsigned char *data, size_t length, const z_data_info_t *info, void *arg);
extern void storage_subscriber_callback_cgo(const z_resource_id_t *rid, const unsigned char *data, size_t length, const z_data_info_t *info, void *arg);
extern void storage_query_handler_cgo(const char *rname, const char *predicate, replies_sender_t send_replies, void *query_handle, void *arg);
extern void eval_query_handler_cgo(const char *rname, const char *predicate, replies_sender_t send_replies, void *query_handle, void *arg);
extern void reply_callback_cgo(const z_reply_value_t *reply, void *arg);

// Indirection since Go cannot call a C function pointer (replies_sender_t)
inline void call_replies_sender(replies_sender_t send_replies, void *query_handle, z_array_resource_t *replies) {
	send_replies(query_handle, *replies);
}

*/
import "C"
import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	log "github.com/sirupsen/logrus"
)

// ZError reports an error that occured in the zenoh-c library
type ZError struct {
	msg  string
	code int
}

func (e *ZError) Error() string {
	return e.msg + " (error code:" + strconv.Itoa(e.code) + ")"
}

var logger = log.WithFields(log.Fields{" pkg": "zenoh"})

// ZOpen opens a connection to a Zenoh broker specified by its locator.
// The returned Zenoh can be used for requests on the Zenoh broker.
func ZOpen(locator string) (*Zenoh, error) {
	logger.WithField("locator", locator).Debug("ZOpen")

	l := C.CString(locator)
	defer C.free(unsafe.Pointer(l))

	result := C.z_open(l, nil, nil)
	if result.tag == C.Z_ERROR_TAG {
		return nil, &ZError{"z_open on " + locator + " failed", resultValueToErrorCode(result.value)}
	}
	z := resultValueToZenoh(result.value)

	logger.WithField("locator", locator).Debug("Run z_recv_loop")
	go C.z_recv_loop(z)

	return z, nil
}

// ZOpenWUP opens a connection to a Zenoh broker specified by its locator, using a username and a password.
// The returned Zenoh can be used for requests on the Zenoh broker.
func ZOpenWUP(locator string, uname string, passwd string) (*Zenoh, error) {
	logger.WithFields(log.Fields{
		"locator": locator,
		"uname":   uname,
	}).Debug("ZOpenWUP")

	l := C.CString(locator)
	defer C.free(unsafe.Pointer(l))
	u := C.CString(uname)
	defer C.free(unsafe.Pointer(u))
	p := C.CString(passwd)
	defer C.free(unsafe.Pointer(p))

	result := C.z_open_wup(l, u, p)
	if result.tag == C.Z_ERROR_TAG {
		return nil, &ZError{"z_open on " + locator + " failed", resultValueToErrorCode(result.value)}
	}
	z := resultValueToZenoh(result.value)

	logger.WithField("locator", locator).Debug("Run z_recv_loop")
	go C.z_recv_loop(z)

	return z, nil
}

// Close closes the connection to the Zenoh broker.
func (z *Zenoh) Close() error {
	logger.Debug("Close")
	errcode := C.z_stop_recv_loop(z)
	if errcode != 0 {
		return &ZError{"z_stop_recv_loop failed", int(errcode)}
	}
	errcode = C.z_close(z)
	if errcode != 0 {
		return &ZError{"z_close failed", int(errcode)}
	}
	return nil
}

// Info returns information about the Zenoh configuration and status.
func (z *Zenoh) Info() map[string]string {
	// TODO: copy all properties from z_vec_t
	info := map[string]string{}
	cprops := C.z_info(z)

	cpeerPid := ((*C.z_property_t)(C.z_vec_get(&cprops, C.Z_INFO_PEER_PID_KEY))).value
	buf := C.GoBytes(unsafe.Pointer(cpeerPid.elem), C.int(cpeerPid.length))
	var peerPid strings.Builder
	for _, x := range buf {
		peerPid.WriteString(fmt.Sprintf("%02x", x))
	}
	info["peer_pid"] = peerPid.String()

	return info
}

type subscriberCallbackRegistry struct {
	mu    *sync.Mutex
	index int
	fns   map[int]SubscriberCallback
}

var subReg = subscriberCallbackRegistry{new(sync.Mutex), 0, make(map[int]SubscriberCallback)}

//export callSubscriberCallback
func callSubscriberCallback(rid *C.z_resource_id_t, data unsafe.Pointer, length C.size_t, info *C.z_data_info_t, arg unsafe.Pointer) {
	var rname string
	if rid.kind == C.Z_STR_RES_ID {
		rname = resIDToRName(rid.id)
	} else {
		fmt.Printf("INTERNAL ERROR: SubscriberCallback received a non-string z_resource_id_t with kind=%d\n", rid.kind)
		return
	}

	dataSlice := C.GoBytes(data, C.int(length))

	// Note: 'arg' parameter is used to store the index of callback in subReg.fns. Don't use it as a real C memory address !!
	index := uintptr(arg)
	goCallback := subReg.fns[int(index)]
	goCallback(rname, dataSlice, info)
}

// DeclareSubscriber declares a Subscriber on a resource
func (z *Zenoh) DeclareSubscriber(resource string, mode SubMode, callback SubscriberCallback) (*Subscriber, error) {
	logger.WithField("resource", resource).Debug("DeclareSubscriber")

	r := C.CString(resource)
	defer C.free(unsafe.Pointer(r))

	subReg.mu.Lock()
	defer subReg.mu.Unlock()
	subReg.index++
	for subReg.fns[subReg.index] != nil {
		subReg.index++
	}
	subReg.fns[subReg.index] = callback

	// Note: 'arg' parameter is used to store the index of callback in subReg.fns. Don't use it as a real C memory address !!
	result := C.z_declare_subscriber(z, r, &mode,
		(C.subscriber_callback_t)(unsafe.Pointer(C.subscriber_callback_cgo)),
		unsafe.Pointer(uintptr(subReg.index)))
	if result.tag == C.Z_ERROR_TAG {
		delete(subReg.fns, subReg.index)
		return nil, &ZError{"z_declare_subscriber for " + resource + " failed", resultValueToErrorCode(result.value)}
	}

	sub := new(Subscriber)
	sub.zsub = resultValueToSubscriber(result.value)
	sub.regIndex = subReg.index

	return sub, nil
}

// DeclarePublisher declares a Publisher on a resource
func (z *Zenoh) DeclarePublisher(resource string) (*Publisher, error) {
	logger.WithField("resource", resource).Debug("DeclarePublisher")

	r := C.CString(resource)
	defer C.free(unsafe.Pointer(r))

	result := C.z_declare_publisher(z, r)
	if result.tag == C.Z_ERROR_TAG {
		return nil, &ZError{"z_declare_publisher for " + resource + " failed", resultValueToErrorCode(result.value)}
	}

	return resultValueToPublisher(result.value), nil
}

type storageCallbacksRegistry struct {
	mu       *sync.Mutex
	index    int
	subCb    map[int]SubscriberCallback
	qHandler map[int]QueryHandler
}

var stoCbReg = storageCallbacksRegistry{new(sync.Mutex), 0, make(map[int]SubscriberCallback), make(map[int]QueryHandler)}

//export callStorageSubscriberCallback
func callStorageSubscriberCallback(rid *C.z_resource_id_t, data unsafe.Pointer, length C.size_t, info *C.z_data_info_t, arg unsafe.Pointer) {
	var rname string
	if rid.kind == C.Z_STR_RES_ID {
		rname = resIDToRName(rid.id)
	} else {
		fmt.Printf("INTERNAL ERROR: StorageSubscriberCallback received a non-string z_resource_id_t with kind=%d\n", rid.kind)
		return
	}

	dataSlice := C.GoBytes(data, C.int(length))

	// Note: 'arg' parameter is used to store the index of callback in stoCbReg.subCb. Don't use it as a real C memory address !!
	index := uintptr(arg)
	goCallback := stoCbReg.subCb[int(index)]
	goCallback(rname, dataSlice, info)
}

//export callStorageQueryHandler
func callStorageQueryHandler(rname *C.char, predicate *C.char, sendReplies unsafe.Pointer, queryHandle unsafe.Pointer, arg unsafe.Pointer) {
	goRname := C.GoString(rname)
	goPredicate := C.GoString(predicate)
	goRepliesSender := new(RepliesSender)
	goRepliesSender.sendRepliesFunc = C.replies_sender_t(sendReplies)
	goRepliesSender.queryHandle = queryHandle

	// Note: 'arg' parameter is used to store the index of callback in stoCbReg.qHandler. Don't use it as a real C memory address !!
	index := uintptr(arg)
	goCallback := stoCbReg.qHandler[int(index)]
	goCallback(goRname, goPredicate, goRepliesSender)
}

// DeclareStorage declares a Storage on a resource
func (z *Zenoh) DeclareStorage(resource string, callback SubscriberCallback, handler QueryHandler) (*Storage, error) {
	logger.WithField("resource", resource).Debug("DeclareStorage")

	r := C.CString(resource)
	defer C.free(unsafe.Pointer(r))

	stoCbReg.mu.Lock()
	defer stoCbReg.mu.Unlock()

	stoCbReg.index++
	for stoCbReg.subCb[stoCbReg.index] != nil {
		stoCbReg.index++
	}
	stoCbReg.subCb[stoCbReg.index] = callback
	stoCbReg.qHandler[stoCbReg.index] = handler

	// Note: 'arg' parameter is used to store the index of callbacks in stoCbReg. Don't use it as a real C memory address !!
	result := C.z_declare_storage(z, r,
		(C.subscriber_callback_t)(unsafe.Pointer(C.storage_subscriber_callback_cgo)),
		(C.query_handler_t)(unsafe.Pointer(C.storage_query_handler_cgo)),
		unsafe.Pointer(uintptr(stoCbReg.index)))
	if result.tag == C.Z_ERROR_TAG {
		delete(stoCbReg.subCb, stoCbReg.index)
		delete(stoCbReg.qHandler, stoCbReg.index)
		return nil, &ZError{"z_declare_storage for " + resource + " failed", resultValueToErrorCode(result.value)}
	}

	storage := new(Storage)
	storage.zsto = resultValueToStorage(result.value)
	storage.regIndex = subReg.index

	return storage, nil
}

type evalCallbacksRegistry struct {
	mu       *sync.Mutex
	index    int
	qHandler map[int]QueryHandler
}

var evalCbReg = evalCallbacksRegistry{new(sync.Mutex), 0, make(map[int]QueryHandler)}

//export callEvalQueryHandler
func callEvalQueryHandler(rname *C.char, predicate *C.char, sendReplies unsafe.Pointer, queryHandle unsafe.Pointer, arg unsafe.Pointer) {
	goRname := C.GoString(rname)
	goPredicate := C.GoString(predicate)
	goRepliesSender := new(RepliesSender)
	goRepliesSender.sendRepliesFunc = C.replies_sender_t(sendReplies)
	goRepliesSender.queryHandle = queryHandle

	// Note: 'arg' parameter is used to store the index of callback in evalCbReg.qHandler. Don't use it as a real C memory address !!
	index := uintptr(arg)
	goCallback := evalCbReg.qHandler[int(index)]
	goCallback(goRname, goPredicate, goRepliesSender)
}

// DeclareEval declares a Eval on a resource
func (z *Zenoh) DeclareEval(resource string, handler QueryHandler) (*Eval, error) {
	logger.WithField("resource", resource).Debug("DeclareEval")

	r := C.CString(resource)
	defer C.free(unsafe.Pointer(r))

	evalCbReg.mu.Lock()
	defer evalCbReg.mu.Unlock()

	evalCbReg.index++
	for evalCbReg.qHandler[evalCbReg.index] != nil {
		evalCbReg.index++
	}
	evalCbReg.qHandler[evalCbReg.index] = handler

	// Note: 'arg' parameter is used to store the index of callbacks in evalCbReg. Don't use it as a real C memory address !!
	result := C.z_declare_eval(z, r,
		(C.query_handler_t)(unsafe.Pointer(C.eval_query_handler_cgo)),
		unsafe.Pointer(uintptr(evalCbReg.index)))
	if result.tag == C.Z_ERROR_TAG {
		delete(evalCbReg.qHandler, evalCbReg.index)
		return nil, &ZError{"z_declare_eval for " + resource + " failed", resultValueToErrorCode(result.value)}
	}

	eval := new(Eval)
	eval.zeval = resultValueToEval(result.value)
	eval.regIndex = evalCbReg.index

	return eval, nil
}

// StreamCompactData writes a payload for the resource with which the Publisher is declared
func (p *Publisher) StreamCompactData(payload []byte) error {
	b, l := bufferToC(payload)
	result := C.z_stream_compact_data(p, b, l)
	if result != 0 {
		return &ZError{"z_stream_compact_data of " + strconv.Itoa(len(payload)) + " bytes buffer failed", int(result)}
	}
	return nil
}

// StreamData writes a payload for the resource with which the Publisher is declared
func (p *Publisher) StreamData(payload []byte) error {
	b, l := bufferToC(payload)
	result := C.z_stream_data(p, b, l)
	if result != 0 {
		return &ZError{"z_stream_data of " + strconv.Itoa(len(payload)) + " bytes buffer failed", int(result)}
	}
	return nil
}

// WriteData writes a payload for a resource in Zenoh
func (z *Zenoh) WriteData(resource string, payload []byte) error {
	r := C.CString(resource)
	defer C.free(unsafe.Pointer(r))

	b, l := bufferToC(payload)
	result := C.z_write_data(z, r, b, l)
	if result != 0 {
		return &ZError{"z_write_data of " + strconv.Itoa(len(payload)) + " bytes buffer on " + resource + "failed", int(result)}
	}
	return nil
}

// StreamDataWO writes a payload for the resource with which the Publisher is declared
func (p *Publisher) StreamDataWO(payload []byte, encoding uint8, kind uint8) error {
	b, l := bufferToC(payload)
	result := C.z_stream_data_wo(p, b, l, C.uchar(encoding), C.uchar(kind))
	if result != 0 {
		return &ZError{"z_stream_data_wo of " + strconv.Itoa(len(payload)) + " bytes buffer failed", int(result)}
	}
	return nil
}

// WriteDataWO writes a payload for a resource in Zenoh
func (z *Zenoh) WriteDataWO(resource string, payload []byte, encoding uint8, kind uint8) error {
	r := C.CString(resource)
	defer C.free(unsafe.Pointer(r))

	b, l := bufferToC(payload)
	result := C.z_write_data_wo(z, r, b, l, C.uchar(encoding), C.uchar(kind))
	if result != 0 {
		return &ZError{"z_write_data_wo of " + strconv.Itoa(len(payload)) + " bytes buffer on " + resource + "failed", int(result)}
	}
	return nil
}

// Pull retrives data for a given pull-mode subscription
func (s *Subscriber) Pull() error {
	result := C.z_pull(s.zsub)
	if result != 0 {
		return &ZError{"z_pull failed", int(result)}
	}
	return nil
}

var nullCPtr = (*C.uchar)(unsafe.Pointer(nil))

func bufferToC(buf []byte) (*C.uchar, C.ulong) {
	if buf == nil {
		return nullCPtr, C.ulong(0)
	}
	return (*C.uchar)(unsafe.Pointer(&buf[0])), C.ulong(len(buf))
}

// RNameIntersect returns true if the resource name 'rname1' intersect with the resrouce name 'rname2'.
func RNameIntersect(rname1 string, rname2 string) bool {
	r1 := C.CString(rname1)
	defer C.free(unsafe.Pointer(r1))
	r2 := C.CString(rname2)
	defer C.free(unsafe.Pointer(r2))

	return C.intersect(r1, r2) != 0
}

type replyCallbackRegistry struct {
	mu    *sync.Mutex
	index int
	fns   map[int]ReplyCallback
}

var replyReg = replyCallbackRegistry{new(sync.Mutex), 0, make(map[int]ReplyCallback)}

//export callReplyCallback
func callReplyCallback(reply *C.z_reply_value_t, arg unsafe.Pointer) {
	index := uintptr(arg)
	goCallback := replyReg.fns[int(index)]
	goCallback(reply)
}

// Query a resource with a predicate
func (z *Zenoh) Query(resource string, predicate string, callback ReplyCallback) error {
	r := C.CString(resource)
	defer C.free(unsafe.Pointer(r))
	p := C.CString(predicate)
	defer C.free(unsafe.Pointer(p))

	replyReg.mu.Lock()
	defer replyReg.mu.Unlock()
	replyReg.index++
	for replyReg.fns[replyReg.index] != nil {
		replyReg.index++
	}
	replyReg.fns[replyReg.index] = callback

	result := C.z_query(z, r, p,
		(C.z_reply_callback_t)(unsafe.Pointer(C.reply_callback_cgo)),
		unsafe.Pointer(uintptr(replyReg.index)))
	if result != 0 {
		return &ZError{"z_query on " + resource + "failed", int(result)}
	}
	return nil
}

// Query a resource with a predicate
func (z *Zenoh) QueryWO(resource string, predicate string, callback ReplyCallback, dest_storages QueryDest, dest_evals QueryDest) error {
	r := C.CString(resource)
	defer C.free(unsafe.Pointer(r))
	p := C.CString(predicate)
	defer C.free(unsafe.Pointer(p))

	replyReg.mu.Lock()
	defer replyReg.mu.Unlock()
	replyReg.index++
	for replyReg.fns[replyReg.index] != nil {
		replyReg.index++
	}
	replyReg.fns[replyReg.index] = callback

	result := C.z_query_wo(z, r, p,
		(C.z_reply_callback_t)(unsafe.Pointer(C.reply_callback_cgo)),
		unsafe.Pointer(uintptr(replyReg.index)),
		dest_storages, dest_evals)
	if result != 0 {
		return &ZError{"z_query on " + resource + "failed", int(result)}
	}
	return nil
}

// UndeclareSubscriber undeclares a Subscriber
func (z *Zenoh) UndeclareSubscriber(s *Subscriber) error {
	result := C.z_undeclare_subscriber(s.zsub)
	if result != 0 {
		return &ZError{"z_undeclare_subscriber failed", int(result)}
	}
	subReg.mu.Lock()
	delete(subReg.fns, s.regIndex)
	subReg.mu.Unlock()

	return nil
}

// UndeclarePublisher undeclares a Publisher
func (z *Zenoh) UndeclarePublisher(p *Publisher) error {
	result := C.z_undeclare_publisher(p)
	if result != 0 {
		return &ZError{"z_undeclare_publisher failed", int(result)}
	}
	return nil
}

// UndeclareStorage undeclares a Storage
func (z *Zenoh) UndeclareStorage(s *Storage) error {
	result := C.z_undeclare_storage(s.zsto)
	if result != 0 {
		return &ZError{"z_undeclare_storage failed", int(result)}
	}
	stoCbReg.mu.Lock()
	delete(stoCbReg.subCb, s.regIndex)
	delete(stoCbReg.qHandler, s.regIndex)
	stoCbReg.mu.Unlock()

	return nil
}

// UndeclareEval undeclares an Eval
func (z *Zenoh) UndeclareEval(e *Eval) error {
	result := C.z_undeclare_eval(e.zeval)
	if result != 0 {
		return &ZError{"z_undeclare_eval failed", int(result)}
	}
	evalCbReg.mu.Lock()
	delete(evalCbReg.qHandler, e.regIndex)
	evalCbReg.mu.Unlock()

	return nil
}

//
// Types and helpers
//

// Zenoh API
type Zenoh = C.z_zenoh_t

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

// RepliesSender is used in a storage's and eval's QueryHandler() implementation when sending back replies to a query.
type RepliesSender struct {
	sendRepliesFunc C.replies_sender_t
	queryHandle     unsafe.Pointer
}

var sizeofUintptr = int(unsafe.Sizeof(uintptr(0)))

// SendReplies sends the replies to a query on a storage.
// This operation should be called in the implementation of a QueryHandler
func (rs *RepliesSender) SendReplies(replies []Resource) {
	// Convert []Resource into z_array_resource_t
	array := new(C.z_array_resource_t)
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

// Resource is a Zenoh resource with a name and a value (data).
type Resource struct {
	RName    string
	Data     []byte
	Encoding uint8
	Kind     uint8
}

// SubscriberCallback is the callback to be implemented for the reception of subscribed resources (subscriber or storage)
type SubscriberCallback func(rid string, data []byte, info *DataInfo)

// QueryHandler is the callback to be implemented for the reception of a query by a storage
type QueryHandler func(rname string, predicate string, sendReplies *RepliesSender)

// ReplyCallback is the callback to be implemented for the reception of a query results
type ReplyCallback func(reply *ReplyValue)

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
	// ZBestMatch : best matching storage or eval
	ZBestMatch QueryDestKind = iota
	// ZComplete : complete storage or eval
	ZComplete QueryDestKind = iota
	// ZAll : all storages or evals
	ZAll QueryDestKind = iota
	// ZNone : no storage or eval
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

// Stoid returns the StorageId of the storage that sent this reply
func (r *ReplyValue) Stoid() []byte {
	return C.GoBytes(unsafe.Pointer(r.stoid), C.int(r.stoid_length))
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

func resultValueToErrorCode(cbytes [8]byte) int {
	buf := bytes.NewBuffer(cbytes[:])
	var code C.int
	if err := binary.Read(buf, binary.LittleEndian, &code); err == nil {
		return int(code)
	}
	return -42
}

func resultValueToZenoh(cbytes [8]byte) *Zenoh {
	buf := bytes.NewBuffer(cbytes[:])
	var ptr uint64
	if err := binary.Read(buf, binary.LittleEndian, &ptr); err == nil {
		uptr := uintptr(ptr)
		return (*Zenoh)(unsafe.Pointer(uptr))
	}
	return nil
}

// resultValueToPublisher gets the Publisher (z_pub_t) from a z_pub_p_result_t.value (union type)
func resultValueToPublisher(cbytes [8]byte) *Publisher {
	buf := bytes.NewBuffer(cbytes[:])
	var ptr uint64
	if err := binary.Read(buf, binary.LittleEndian, &ptr); err == nil {
		uptr := uintptr(ptr)
		return (*Publisher)(unsafe.Pointer(uptr))
	}
	return nil
}

// resultValueToSubscriber gets the Subscriber (z_sub_t) from a z_sub_p_result_t.value (union type)
func resultValueToSubscriber(cbytes [8]byte) *C.z_sub_t {
	buf := bytes.NewBuffer(cbytes[:])
	var ptr uint64
	if err := binary.Read(buf, binary.LittleEndian, &ptr); err == nil {
		uptr := uintptr(ptr)
		return (*C.z_sub_t)(unsafe.Pointer(uptr))
	}
	return nil
}

// resultValueToStorage gets the Storage (z_sto_t) from a z_sto_p_result_t.value (union type)
func resultValueToStorage(cbytes [8]byte) *C.z_sto_t {
	buf := bytes.NewBuffer(cbytes[:])
	var ptr uint64
	if err := binary.Read(buf, binary.LittleEndian, &ptr); err == nil {
		uptr := uintptr(ptr)
		return (*C.z_sto_t)(unsafe.Pointer(uptr))
	}
	return nil
}

// resultValueToEval gets the Eval (z_eva_t) from a z_eval_p_result_t.value (union type)
func resultValueToEval(cbytes [8]byte) *C.z_eva_t {
	buf := bytes.NewBuffer(cbytes[:])
	var ptr uint64
	if err := binary.Read(buf, binary.LittleEndian, &ptr); err == nil {
		uptr := uintptr(ptr)
		return (*C.z_eva_t)(unsafe.Pointer(uptr))
	}
	return nil
}

// resIDToRName gets the rname (string) from a z_res_id_t (union type)
func resIDToRName(cbytes [8]byte) string {
	buf := bytes.NewBuffer(cbytes[:])
	var ptr uint64
	if err := binary.Read(buf, binary.LittleEndian, &ptr); err == nil {
		uptr := uintptr(ptr)
		return C.GoString((*C.char)(unsafe.Pointer(uptr)))
	}
	panic("resIDToRName: failed to read 64bits pointer from z_res_id_t union (represented as a [8]byte)")

}
