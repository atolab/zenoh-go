package main

import (
	"encoding/hex"
	"fmt"
	"os"

	"github.com/atolab/zenoh-go"
)

func main() {
	locator := "tcp/127.0.0.1:7447"
	if len(os.Args) > 1 {
		locator = os.Args[1]
	}

	fmt.Println("Connecting to " + locator + "...")
	properties := map[int][]byte{
		zenoh.Z_USER_KEY:   []byte("user"),
		zenoh.Z_PASSWD_KEY: []byte("password")}
	z, err := zenoh.ZOpen(locator, properties)
	if err != nil {
		panic(err.Error())
	}

	info := z.Info()
	fmt.Println("LOCATOR :  " + string(info[zenoh.Z_INFO_PEER_KEY]))
	fmt.Println("PID :      " + hex.EncodeToString(info[zenoh.Z_INFO_PID_KEY]))
	fmt.Println("PEER PID : " + hex.EncodeToString(info[zenoh.Z_INFO_PEER_PID_KEY]))

	z.Close()
}
