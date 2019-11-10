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

// Forward declarations of callbacks (see c_callbacks.go)
extern void subscriber_handle_data_cgo(const z_resource_id_t *rid, const unsigned char *data, size_t length, const z_data_info_t *info, void *arg);
extern void storage_handle_data_cgo(const z_resource_id_t *rid, const unsigned char *data, size_t length, const z_data_info_t *info, void *arg);
extern void storage_handle_query_cgo(const char *rname, const char *predicate, z_replies_sender_t send_replies, void *query_handle, void *arg);
extern void eval_handle_query_cgo(const char *rname, const char *predicate, z_replies_sender_t send_replies, void *query_handle, void *arg);
extern void handle_reply_cgo(const z_reply_value_t *reply, void *arg);

// Indirection since Go cannot call a C function pointer (z_replies_sender_t)
inline void call_replies_sender(z_replies_sender_t send_replies, void *query_handle, z_array_p_resource_t *replies) {
	send_replies(query_handle, *replies);
}

*/
import "C"
import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"sync"
	"time"
	"unsafe"

	log "github.com/sirupsen/logrus"
)

const Z_INFO_PID_KEY = C.Z_INFO_PID_KEY
const Z_INFO_PEER_KEY = C.Z_INFO_PEER_KEY
const Z_INFO_PEER_PID_KEY = C.Z_INFO_PEER_PID_KEY

const Z_USER_KEY = C.Z_USER_KEY
const Z_PASSWD_KEY = C.Z_PASSWD_KEY

// ZError reports an error that occured in the zenoh-c library
type ZError struct {
	msg  string
	code int
}

func (e *ZError) Error() string {
	return e.msg + " (error code:" + strconv.Itoa(e.code) + ")"
}

var logger = log.WithFields(log.Fields{" pkg": "zenoh"})

// Open a zenoh session with the infrastructure component (zenoh router, zenoh broker, ...) reachable at location 'locator'.
// 'locator' is a string representation of a network endpoint. A typical locator looks like this : "tcp/127.0.0.1:7447".
// 'properties' is a map of properties that will be used to establish and configure the zenoh session.
// 'properties' will typically contain the username and password informations needed to establish the zenoh session with a secured infrastructure.
// It can be set to "nil".
// Return a handle to the zenoh session.
func ZOpen(locator *string, properties map[int][]byte) (*Zenoh, error) {
	logger.WithField("locator", locator).Debug("ZOpen")

	pvec := ((C.z_vec_t)(C.z_vec_make(C.uint(len(properties)))))
	for k, v := range properties {
		value := C.z_array_uint8_t{length: C.uint(len(v)), elem: (*C.uchar)(unsafe.Pointer(&v[0]))}
		prop := ((*C.z_property_t)(C.z_property_make(C.ulong(k), value)))
		C.z_vec_append(&pvec, unsafe.Pointer(prop))
	}

	var l *C.char
	if locator != nil {
		l := C.CString(*locator)
		defer C.free(unsafe.Pointer(l))
	}

	result := C.z_open(l, nil, &pvec)
	if result.tag == C.Z_ERROR_TAG {
		return nil, &ZError{"z_open failed", resultValueToErrorCode(result.value)}
	}
	z := resultValueToZenoh(result.value)

	logger.WithField("locator", locator).Debug("Run z_recv_loop")
	go C.z_recv_loop(z)

	return z, nil
}

// Close the zenoh session 'z'.
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

// Return a map of properties containing various informations about the
// established zenoh session 'z'.
func (z *Zenoh) Info() map[int][]byte {
	info := map[int][]byte{}
	cprops := C.z_info(z)
	propslength := int(C.z_vec_length(&cprops))
	for i := 0; i < propslength; i++ {
		cvalue := ((*C.z_property_t)(C.z_vec_get(&cprops, C.uint(i)))).value
		info[i] = C.GoBytes(unsafe.Pointer(cvalue.elem), C.int(cvalue.length))
	}
	return info
}

type subscriberHandlersRegistry struct {
	mu       *sync.Mutex
	index    int
	dHandler map[int]DataHandler
}

var subReg = subscriberHandlersRegistry{new(sync.Mutex), 0, make(map[int]DataHandler)}

//export callSubscriberDataHandler
func callSubscriberDataHandler(rid *C.z_resource_id_t, data unsafe.Pointer, length C.size_t, info *C.z_data_info_t, arg unsafe.Pointer) {
	var rname string
	if rid.kind == C.Z_STR_RES_ID {
		rname = resIDToRName(rid.id)
	} else {
		fmt.Printf("INTERNAL ERROR: DataHandler received a non-string z_resource_id_t with kind=%d\n", rid.kind)
		return
	}

	dataSlice := C.GoBytes(data, C.int(length))

	// Note: 'arg' parameter is used to store the index of handler in subReg.dHandler. Don't use it as a real C memory address !!
	index := uintptr(arg)
	goHandler := subReg.dHandler[int(index)]
	goHandler(rname, dataSlice, info)
}

// Declare a subscribtion for all published data matching the provided resource selezctor 'resource'.
// 'resource' is the resource selection to subscribe to.
// 'mode' is the subscription mode.
// 'dataHandler' is the callback function that will be called each time a data matching the subscribed 'resource' selectoin is received.
// Return a zenoh subscriber.
func (z *Zenoh) DeclareSubscriber(resource string, mode SubMode, dataHandler DataHandler) (*Subscriber, error) {
	logger.WithField("resource", resource).Debug("DeclareSubscriber")

	r := C.CString(resource)
	defer C.free(unsafe.Pointer(r))

	subReg.mu.Lock()
	defer subReg.mu.Unlock()
	subReg.index++
	for subReg.dHandler[subReg.index] != nil {
		subReg.index++
	}
	subReg.dHandler[subReg.index] = dataHandler

	// Note: 'arg' parameter is used to store the index of handler in subReg.dHandler. Don't use it as a real C memory address !!
	result := C.z_declare_subscriber(z, r, &mode,
		(C.z_data_handler_t)(unsafe.Pointer(C.subscriber_handle_data_cgo)),
		unsafe.Pointer(uintptr(subReg.index)))
	if result.tag == C.Z_ERROR_TAG {
		delete(subReg.dHandler, subReg.index)
		return nil, &ZError{"z_declare_subscriber for " + resource + " failed", resultValueToErrorCode(result.value)}
	}

	sub := new(Subscriber)
	sub.zsub = resultValueToSubscriber(result.value)
	sub.regIndex = subReg.index

	return sub, nil
}

// Declare a publication for resource selection 'resource'.
// 'resource' is the resource selection to publish.
// Return a zenoh publisher.
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

type storageHandlersRegistry struct {
	mu       *sync.Mutex
	index    int
	dHandler map[int]DataHandler
	qHandler map[int]QueryHandler
}

var stoHdlReg = storageHandlersRegistry{new(sync.Mutex), 0, make(map[int]DataHandler), make(map[int]QueryHandler)}

//export callStorageDataHandler
func callStorageDataHandler(rid *C.z_resource_id_t, data unsafe.Pointer, length C.size_t, info *C.z_data_info_t, arg unsafe.Pointer) {
	var rname string
	if rid.kind == C.Z_STR_RES_ID {
		rname = resIDToRName(rid.id)
	} else {
		fmt.Printf("INTERNAL ERROR: DataHandler received a non-string z_resource_id_t with kind=%d\n", rid.kind)
		return
	}

	dataSlice := C.GoBytes(data, C.int(length))

	// Note: 'arg' parameter is used to store the index of handler in stoHdlReg.subCb. Don't use it as a real C memory address !!
	index := uintptr(arg)
	goHandler := stoHdlReg.dHandler[int(index)]
	goHandler(rname, dataSlice, info)
}

//export callStorageQueryHandler
func callStorageQueryHandler(rname *C.char, predicate *C.char, sendReplies unsafe.Pointer, queryHandle unsafe.Pointer, arg unsafe.Pointer) {
	goRname := C.GoString(rname)
	goPredicate := C.GoString(predicate)
	goRepliesSender := new(RepliesSender)
	goRepliesSender.sendRepliesFunc = C.z_replies_sender_t(sendReplies)
	goRepliesSender.queryHandle = queryHandle

	// Note: 'arg' parameter is used to store the index of handler in stoHdlReg.qHandler. Don't use it as a real C memory address !!
	index := uintptr(arg)
	goHandler := stoHdlReg.qHandler[int(index)]
	goHandler(goRname, goPredicate, goRepliesSender)
}

// Declare a storage for all data matching the provided resource selector 'resource'.
// 'resource' is the resource selection to store.
// 'dataHandler' is the callback function that will be called each time a data matching the stored 'resource' selection is received.
// 'queryHandler' is the callback function that will be called each time a query for data matching the stored 'resource' selection is received.
// The 'queryHandler' function MUST call the provided 'sendReplies' function with the resulting data. 'sendReplies' can be called with an empty array.
// Return a zenoh storage.
func (z *Zenoh) DeclareStorage(resource string, dataHandler DataHandler, queryHandler QueryHandler) (*Storage, error) {
	logger.WithField("resource", resource).Debug("DeclareStorage")

	r := C.CString(resource)
	defer C.free(unsafe.Pointer(r))

	stoHdlReg.mu.Lock()
	defer stoHdlReg.mu.Unlock()

	stoHdlReg.index++
	for stoHdlReg.dHandler[stoHdlReg.index] != nil {
		stoHdlReg.index++
	}
	stoHdlReg.dHandler[stoHdlReg.index] = dataHandler
	stoHdlReg.qHandler[stoHdlReg.index] = queryHandler

	// Note: 'arg' parameter is used to store the index of handler in stoHdlReg. Don't use it as a real C memory address !!
	result := C.z_declare_storage(z, r,
		(C.z_data_handler_t)(unsafe.Pointer(C.storage_handle_data_cgo)),
		(C.z_query_handler_t)(unsafe.Pointer(C.storage_handle_query_cgo)),
		unsafe.Pointer(uintptr(stoHdlReg.index)))
	if result.tag == C.Z_ERROR_TAG {
		delete(stoHdlReg.dHandler, stoHdlReg.index)
		delete(stoHdlReg.qHandler, stoHdlReg.index)
		return nil, &ZError{"z_declare_storage for " + resource + " failed", resultValueToErrorCode(result.value)}
	}

	storage := new(Storage)
	storage.zsto = resultValueToStorage(result.value)
	storage.regIndex = subReg.index

	return storage, nil
}

type evalHandlersRegistry struct {
	mu       *sync.Mutex
	index    int
	qHandler map[int]QueryHandler
}

var evalHdlReg = evalHandlersRegistry{new(sync.Mutex), 0, make(map[int]QueryHandler)}

//export callEvalQueryHandler
func callEvalQueryHandler(rname *C.char, predicate *C.char, sendReplies unsafe.Pointer, queryHandle unsafe.Pointer, arg unsafe.Pointer) {
	goRname := C.GoString(rname)
	goPredicate := C.GoString(predicate)
	goRepliesSender := new(RepliesSender)
	goRepliesSender.sendRepliesFunc = C.z_replies_sender_t(sendReplies)
	goRepliesSender.queryHandle = queryHandle

	// Note: 'arg' parameter is used to store the index of handler in evalHdlReg.qHandler. Don't use it as a real C memory address !!
	index := uintptr(arg)
	goHandler := evalHdlReg.qHandler[int(index)]
	goHandler(goRname, goPredicate, goRepliesSender)
}

// Declare an eval able to provide data matching the provided resource selector 'resource'.
// 'resource' is the resource selection to evaluate.
// 'handler' is the callback function that will be called each time a query for data matching the evaluated 'resource' selection is received.
// The 'handler' function MUST call the provided 'sendReplies' function with the resulting data. 'sendReplies'can be called with an empty array.
// Return a zenoh eval.
func (z *Zenoh) DeclareEval(resource string, handler QueryHandler) (*Eval, error) {
	logger.WithField("resource", resource).Debug("DeclareEval")

	r := C.CString(resource)
	defer C.free(unsafe.Pointer(r))

	evalHdlReg.mu.Lock()
	defer evalHdlReg.mu.Unlock()

	evalHdlReg.index++
	for evalHdlReg.qHandler[evalHdlReg.index] != nil {
		evalHdlReg.index++
	}
	evalHdlReg.qHandler[evalHdlReg.index] = handler

	// Note: 'arg' parameter is used to store the index of handler in evalHdlReg. Don't use it as a real C memory address !!
	result := C.z_declare_eval(z, r,
		(C.z_query_handler_t)(unsafe.Pointer(C.eval_handle_query_cgo)),
		unsafe.Pointer(uintptr(evalHdlReg.index)))
	if result.tag == C.Z_ERROR_TAG {
		delete(evalHdlReg.qHandler, evalHdlReg.index)
		return nil, &ZError{"z_declare_eval for " + resource + " failed", resultValueToErrorCode(result.value)}
	}

	eval := new(Eval)
	eval.zeval = resultValueToEval(result.value)
	eval.regIndex = evalHdlReg.index

	return eval, nil
}

// Send data in a 'compact_data' message for the resource selector published by publisher 'p'.
// 'payload' is the data to be sent.
func (p *Publisher) StreamCompactData(payload []byte) error {
	b, l := bufferToC(payload)
	result := C.z_stream_compact_data(p, b, l)
	if result != 0 {
		return &ZError{"z_stream_compact_data of " + strconv.Itoa(len(payload)) + " bytes buffer failed", int(result)}
	}
	return nil
}

// Send data in a 'stream_data' message for the resource selector published by publisher 'p'.
// 'payload' is the data to be sent.
func (p *Publisher) StreamData(payload []byte) error {
	b, l := bufferToC(payload)
	result := C.z_stream_data(p, b, l)
	if result != 0 {
		return &ZError{"z_stream_data of " + strconv.Itoa(len(payload)) + " bytes buffer failed", int(result)}
	}
	return nil
}

// Send data in a 'write_data' message for the resource selector 'resource'.
// 'resource' is the resource name selector of the data to be sent.
// 'payload' is the data to be sent.
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

// Send data in a 'stream_data' message for the resource selector published by publisher 'p'.
// 'payload' is the data to be sent.
// 'encoding' is a metadata information associated with the published data that represents the encoding of the published data.
// 'kind' is a metadata information associated with the published data that represents the kind of publication.
func (p *Publisher) StreamDataWO(payload []byte, encoding uint8, kind uint8) error {
	b, l := bufferToC(payload)
	result := C.z_stream_data_wo(p, b, l, C.uchar(encoding), C.uchar(kind))
	if result != 0 {
		return &ZError{"z_stream_data_wo of " + strconv.Itoa(len(payload)) + " bytes buffer failed", int(result)}
	}
	return nil
}

// Send data in a 'write_data' message for the resource selector 'resource'.
// 'resource' is the resource name selector of the data to be sent.
// 'payload' is the data to be sent.
// 'encoding' is a metadata information associated with the published data that represents the encoding of the published data.
// 'kind' is a metadata information associated with the published data that represents the kind of publication.
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

// Pull data for the `ZPullMode` or `ZPeriodicPullMode` subscribtion 's'. The pulled data will be provided
// by calling the 'dataHandler' function provided to the `DeclareSubscriber` function.
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

// Return true if the resource name selector 'rname1' intersects with the resrouce name selector 'rname2'.
func RNameIntersect(rname1 string, rname2 string) bool {
	r1 := C.CString(rname1)
	defer C.free(unsafe.Pointer(r1))
	r2 := C.CString(rname2)
	defer C.free(unsafe.Pointer(r2))

	return C.intersect(r1, r2) != 0
}

type replyHandlersRegistry struct {
	mu       *sync.Mutex
	index    int
	rHandler map[int]ReplyHandler
}

var replyReg = replyHandlersRegistry{new(sync.Mutex), 0, make(map[int]ReplyHandler)}

//export callReplyHandler
func callReplyHandler(reply *C.z_reply_value_t, arg unsafe.Pointer) {
	index := uintptr(arg)
	goHandler := replyReg.rHandler[int(index)]
	goHandler(reply)
}

// Query data matching resource selector 'resource'.
// 'resource' is the resource selection to query.
// 'predicate' is a string that will be  propagated to the storages and evals that should provide the queried data.
// It may allow them to filter, transform and/or compute the queried data.
// 'replyHandler' is the callback function that will be called on reception of the replies of the query.
func (z *Zenoh) Query(resource string, predicate string, replyHandler ReplyHandler) error {
	r := C.CString(resource)
	defer C.free(unsafe.Pointer(r))
	p := C.CString(predicate)
	defer C.free(unsafe.Pointer(p))

	replyReg.mu.Lock()
	defer replyReg.mu.Unlock()
	replyReg.index++
	for replyReg.rHandler[replyReg.index] != nil {
		replyReg.index++
	}
	replyReg.rHandler[replyReg.index] = replyHandler

	result := C.z_query(z, r, p,
		(C.z_reply_handler_t)(unsafe.Pointer(C.handle_reply_cgo)),
		unsafe.Pointer(uintptr(replyReg.index)))
	if result != 0 {
		return &ZError{"z_query on " + resource + "failed", int(result)}
	}
	return nil
}

// Query data matching resource selector 'resource'.
// 'resource' is the resource selection to query.
// 'predicate' is a string that will be  propagated to the storages and evals that should provide the queried data.
// It may allow them to filter, transform and/or compute the queried data.
// 'replyHandler' is the callback function that will be called on reception of the replies of the query.
// 'dest_storages' indicates which matching storages should be destination of the query.
// 'dest_evals' indicates which matching evals should be destination of the query.
func (z *Zenoh) QueryWO(resource string, predicate string, replyHandler ReplyHandler, dest_storages QueryDest, dest_evals QueryDest) error {
	r := C.CString(resource)
	defer C.free(unsafe.Pointer(r))
	p := C.CString(predicate)
	defer C.free(unsafe.Pointer(p))

	replyReg.mu.Lock()
	defer replyReg.mu.Unlock()
	replyReg.index++
	for replyReg.rHandler[replyReg.index] != nil {
		replyReg.index++
	}
	replyReg.rHandler[replyReg.index] = replyHandler

	result := C.z_query_wo(z, r, p,
		(C.z_reply_handler_t)(unsafe.Pointer(C.handle_reply_cgo)),
		unsafe.Pointer(uintptr(replyReg.index)),
		dest_storages, dest_evals)
	if result != 0 {
		return &ZError{"z_query on " + resource + "failed", int(result)}
	}
	return nil
}

// Undeclare the subscribtion 's'.
func (z *Zenoh) UndeclareSubscriber(s *Subscriber) error {
	result := C.z_undeclare_subscriber(s.zsub)
	if result != 0 {
		return &ZError{"z_undeclare_subscriber failed", int(result)}
	}
	subReg.mu.Lock()
	delete(subReg.dHandler, s.regIndex)
	subReg.mu.Unlock()

	return nil
}

// Undeclare the publication 'p'.
func (z *Zenoh) UndeclarePublisher(p *Publisher) error {
	result := C.z_undeclare_publisher(p)
	if result != 0 {
		return &ZError{"z_undeclare_publisher failed", int(result)}
	}
	return nil
}

// Undeclare the storage 's'.
func (z *Zenoh) UndeclareStorage(s *Storage) error {
	result := C.z_undeclare_storage(s.zsto)
	if result != 0 {
		return &ZError{"z_undeclare_storage failed", int(result)}
	}
	stoHdlReg.mu.Lock()
	delete(stoHdlReg.dHandler, s.regIndex)
	delete(stoHdlReg.qHandler, s.regIndex)
	stoHdlReg.mu.Unlock()

	return nil
}

// Undeclare the eval 'e'.
func (z *Zenoh) UndeclareEval(e *Eval) error {
	result := C.z_undeclare_eval(e.zeval)
	if result != 0 {
		return &ZError{"z_undeclare_eval failed", int(result)}
	}
	evalHdlReg.mu.Lock()
	delete(evalHdlReg.qHandler, e.regIndex)
	evalHdlReg.mu.Unlock()

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

// Resource is a Zenoh resource with a name and a value (data).
type Resource struct {
	RName    string
	Data     []byte
	Encoding uint8
	Kind     uint8
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

// SrcId returns the unique id of the storage or eval that sent this reply
func (r *ReplyValue) SrcId() []byte {
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
