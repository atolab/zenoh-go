package zenoh

/*
#include <zenoh.h>
#include <stdio.h>
#include <zenoh/codec.h>

void subscriber_callback_cgo(const z_resource_id_t *rid, const unsigned char *data, size_t length, const z_data_info_t *info, void *arg) {
	void callSubscriberCallback(const z_resource_id_t*, const unsigned char*, size_t, const z_data_info_t*, void*);
	callSubscriberCallback(rid, data, length, info, arg);
}

void storage_subscriber_callback_cgo(const z_resource_id_t *rid, const unsigned char *data, size_t length, const z_data_info_t *info, void *arg) {
	void callStorageSubscriberCallback(const z_resource_id_t*, const unsigned char*, size_t, const z_data_info_t*, void*);
	callStorageSubscriberCallback(rid, data, length, info, arg);
}

void storage_query_handler_cgo(const char *rname, const char *predicate, replies_sender_t send_replies, void *query_handle, void *arg) {
	void callStorageQueryHandler(const char*, const char*, replies_sender_t, void*, void*);
	callStorageQueryHandler(rname, predicate, send_replies, query_handle, arg);
}

void reply_callback_cgo(const z_reply_value_t *reply, void *arg) {
	void callReplyCallback(const z_reply_value_t*, void*);
	callReplyCallback(reply, arg);
}

*/
import "C"
