package zenoh

/*

#include <zenoh.h>
#include <stdio.h>
#include <zenoh/codec.h>


void subscriber_callback_cgo(const z_resource_id_t *rid, const unsigned char *data, size_t length, const z_data_info_t *info, void *arg) {
	void callSubscriberCallback(const z_resource_id_t*, const unsigned char*, size_t, const z_data_info_t*, void*);
	callSubscriberCallback(rid, data, length, info, arg);
}

*/
import "C"
