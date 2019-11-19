package main

import (
	"encoding/hex"
	"fmt"
	"os"

	"github.com/atolab/zenoh-go"
)

func main() {
	var locator *string
	if len(os.Args) > 1 {
		locator = &os.Args[1]
	}

	fmt.Println("Openning session...")
	properties := map[int][]byte{
		zenoh.ZUserKey:   []byte("user"),
		zenoh.ZPasswdKey: []byte("password")}
	z, err := zenoh.ZOpen(locator, properties)
	if err != nil {
		panic(err.Error())
	}

	info := z.Info()
	fmt.Println("LOCATOR :  " + string(info[zenoh.ZInfoPeerKey]))
	fmt.Println("PID :      " + hex.EncodeToString(info[zenoh.ZInfoPidKey]))
	fmt.Println("PEER PID : " + hex.EncodeToString(info[zenoh.ZInfoPeerPidKey]))

	z.Close()
}
