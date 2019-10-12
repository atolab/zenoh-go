package main

import (
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
	properties := map[int]string{
		zenoh.Z_USER_KEY:   "user",
		zenoh.Z_PASSWD_KEY: "password"}
	z, err := zenoh.ZOpen(locator, properties)
	if err != nil {
		panic(err.Error())
	}

	info := z.Info()
	fmt.Println("LOCATOR :  " + info[zenoh.Z_INFO_PEER_KEY])
	fmt.Println("PID :      " + info[zenoh.Z_INFO_PID_KEY])
	fmt.Println("PEER PID : " + info[zenoh.Z_INFO_PEER_PID_KEY])

	z.Close()
}
