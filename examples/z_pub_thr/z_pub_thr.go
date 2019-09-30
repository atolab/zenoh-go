package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/atolab/zenoh-go"
)

func main() {
	locator := "tcp/127.0.0.1:7447"
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
		locator = os.Args[2]
	}

	data := make([]byte, length)
	for i := 0; i < length; i++ {
		data[i] = byte(i % 10)
	}

	z, err := zenoh.ZOpen(locator)
	if err != nil {
		panic(err.Error())
	}

	pub, err := z.DeclarePublisher("/test/thr")
	if err != nil {
		panic(err.Error())
	}

	for true {
		pub.StreamData(data)
	}

}
