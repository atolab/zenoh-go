package main

import (
	"encoding/hex"
	"fmt"
	"os"

	znet "github.com/atolab/zenoh-go/net"
)

func main() {
	var locator *string
	if len(os.Args) > 1 {
		locator = &os.Args[1]
	}

	fmt.Println("Opening session...")
	properties := map[int][]byte{
		znet.UserKey:   []byte("user"),
		znet.PasswdKey: []byte("password")}
	s, err := znet.Open(locator, properties)
	if err != nil {
		panic(err.Error())
	}
	defer s.Close()

	info := s.Info()
	fmt.Println("LOCATOR :  " + string(info[znet.InfoPeerKey]))
	fmt.Println("PID :      " + hex.EncodeToString(info[znet.InfoPidKey]))
	fmt.Println("PEER PID : " + hex.EncodeToString(info[znet.InfoPeerPidKey]))
}
