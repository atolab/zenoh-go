package zenoh

/*
#cgo CFLAGS: -DZENOH_MACOS
#cgo LDFLAGS: -lzenohc

#define ZENOH_MACOS 1

#include <stdlib.h>
#include <zenoh.h>
#include <zenoh/recv_loop.h>

// Forward declarations of callbacks (see callbacks.go)
extern void subscriber_callback_cgo(const z_resource_id_t *rid, const unsigned char *data, size_t length, const z_data_info_t *info, void *arg);

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
)

// ZError reports an error that occured in the zenoh-c library
type ZError struct {
	msg  string
	code int
}

func (e *ZError) Error() string {
	return e.msg + " (error code:" + strconv.Itoa(e.code) + ")"
}

// ZOpen opens a connection to a Zenoh broker or peer.
func ZOpen(locator string) (*Zenoh, error) {
	l := C.CString(locator)
	defer C.free(unsafe.Pointer(l))

	fmt.Println("Call z_open on" + locator)
	result := C.z_open(l, nil, nil)

	if result.tag == C.Z_ERROR_TAG {
		return nil, &ZError{"z_open on " + locator + " failed", resultValueToErrorCode(result.value)}
	}

	z := resultValueToZenoh(result.value)
	errcode := C.z_start_recv_loop(z)
	if errcode != 0 {
		return nil, &ZError{"z_start_recv_loop failed", int(errcode)}
	}

	return z, nil
}

// ZOpenWUP opens a connection to a Zenoh broker or peer, using a username and a password.
func ZOpenWUP(locator string, uname string, passwd string) (*Zenoh, error) {
	l := C.CString(locator)
	defer C.free(unsafe.Pointer(l))
	u := C.CString(uname)
	defer C.free(unsafe.Pointer(u))
	p := C.CString(passwd)
	defer C.free(unsafe.Pointer(p))

	fmt.Println("Call z_open_wup on" + locator)
	result := C.z_open_wup(l, u, p)

	if result.tag == C.Z_ERROR_TAG {
		return nil, &ZError{"z_open on " + locator + " failed", resultValueToErrorCode(result.value)}
	}

	return resultValueToZenoh(result.value), nil
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
	r := C.CString(resource)
	defer C.free(unsafe.Pointer(r))

	result := C.z_declare_publisher(z, r)
	if result.tag == C.Z_ERROR_TAG {
		return nil, &ZError{"z_declare_publisher for " + resource + " failed", resultValueToErrorCode(result.value)}
	}

	return resultValueToPublisher(result.value), nil
}

//
// z_declare_storage
//

// StreamCompactData writes a payload for the resource with which the Publisher is declared
func (p *Publisher) StreamCompactData(payload []byte) error {
	result := C.z_stream_compact_data(p, (*C.uchar)(unsafe.Pointer(&payload[0])), C.ulong(len(payload)))
	if result != 0 {
		return &ZError{"z_stream_compact_data of " + strconv.Itoa(len(payload)) + " bytes buffer failed", int(result)}
	}
	return nil
}

// StreamData writes a payload for the resource with which the Publisher is declared
func (p *Publisher) StreamData(payload []byte) error {
	result := C.z_stream_data(p, (*C.uchar)(unsafe.Pointer(&payload[0])), C.ulong(len(payload)))
	if result != 0 {
		return &ZError{"z_stream_data of " + strconv.Itoa(len(payload)) + " bytes buffer failed", int(result)}
	}
	return nil
}

// WriteData writes a payload for a resource in Zenoh
func (z *Zenoh) WriteData(resource string, payload []byte) error {
	r := C.CString(resource)
	defer C.free(unsafe.Pointer(r))

	result := C.z_write_data(z, r, (*C.uchar)(unsafe.Pointer(&payload[0])), C.ulong(len(payload)))
	if result != 0 {
		return &ZError{"z_write_data of " + strconv.Itoa(len(payload)) + " bytes buffer on " + resource + "failed", int(result)}
	}
	return nil
}

// StreamDataWO writes a payload for the resource with which the Publisher is declared
func (p *Publisher) StreamDataWO(payload []byte, encoding int, kind int) error {
	result := C.z_stream_data_wo(p, (*C.uchar)(unsafe.Pointer(&payload[0])), C.ulong(len(payload)), C.uchar(encoding), C.uchar(kind))
	if result != 0 {
		return &ZError{"z_stream_data_wo of " + strconv.Itoa(len(payload)) + " bytes buffer failed", int(result)}
	}
	return nil
}

// WriteDataWO writes a payload for a resource in Zenoh
func (z *Zenoh) WriteDataWO(resource string, payload []byte, encoding int, kind int) error {
	r := C.CString(resource)
	defer C.free(unsafe.Pointer(r))

	result := C.z_write_data_wo(z, r, (*C.uchar)(unsafe.Pointer(&payload[0])), C.ulong(len(payload)), C.uchar(encoding), C.uchar(kind))
	if result != 0 {
		return &ZError{"z_write_data_wo of " + strconv.Itoa(len(payload)) + " bytes buffer on " + resource + "failed", int(result)}
	}
	return nil
}

//
// z_query
//

// UndeclareSubscriber a Subscriber
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

// UndeclarePublisher a Publisher
func (z *Zenoh) UndeclarePublisher(p *Publisher) error {
	result := C.z_undeclare_publisher(p)
	if result != 0 {
		return &ZError{"z_undeclare_publisher failed", int(result)}
	}
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

// SubscriberCallback the callback to be implemented for the reception of subscribed resources
type SubscriberCallback func(rid string, data []byte, info *DataInfo)

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
