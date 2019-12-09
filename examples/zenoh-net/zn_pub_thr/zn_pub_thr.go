package main

import (
	"fmt"
	"os"
	"strconv"

	znet "github.com/atolab/zenoh-go/net"
)

func main() {
	var locator *string
	if len(os.Args) < 2 {
		fmt.Printf("USAGE:\n\tz_pub_thr <payload-size> [<zenoh-locator>]\n\n")
		os.Exit(-1)
	}

	length, err := strconv.Atoi(os.Args[1])
	if err != nil {
		panic(err.Error())
	}
	fmt.Printf("Running throughput test for payload of %d bytes\n", length)
	if len(os.Args) > 2 {
		locator = &os.Args[2]
	}

	data := make([]byte, length)
	for i := 0; i < length; i++ {
		data[i] = byte(i % 10)
	}

	s, err := znet.ZOpen(locator, nil)
	if err != nil {
		panic(err.Error())
	}
	defer s.Close()

	pub, err := s.DeclarePublisher("/test/thr")
	if err != nil {
		panic(err.Error())
	}
	defer s.UndeclarePublisher(pub)

	for true {
		pub.StreamData(data)
	}

}
