package main

import (
	"fmt"
	"os"
	"time"

	"github.com/atolab/zenoh-go"
)

func main() {
	locator := "tcp/127.0.0.1:7447"
	if len(os.Args) > 1 {
		locator = os.Args[1]
	}

	uri := "/demo/example/zenoh-go-stream"
	if len(os.Args) > 2 {
		uri = os.Args[2]
	}

	value := "Stream from Go!"
	if len(os.Args) > 3 {
		value = os.Args[3]
	}

	fmt.Println("Connecting to " + locator + "...")
	z, err := zenoh.ZOpen(locator, nil)
	if err != nil {
		panic(err.Error())
	}

	fmt.Println("Declaring Publisher on '" + uri + "'...")
	pub, err := z.DeclarePublisher(uri)
	if err != nil {
		panic(err.Error())
	}

	for idx := 0; idx < 100; idx++ {
		time.Sleep(1 * time.Second)
		s := fmt.Sprintf("[%4d] %s", idx, value)
		fmt.Printf("Streaming Data ('%s': '%s')...\n", uri, s)
		pub.StreamData([]byte(s))
	}

	z.UndeclarePublisher(pub)
	z.Close()
}
