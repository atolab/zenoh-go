package main

import (
	"fmt"
	"os"
	"time"

	znet "github.com/atolab/zenoh-go/net"
)

func main() {
	uri := "/demo/example/zenoh-go-stream"
	if len(os.Args) > 1 {
		uri = os.Args[1]
	}

	value := "Stream from Go!"
	if len(os.Args) > 2 {
		value = os.Args[2]
	}

	var locator *string
	if len(os.Args) > 3 {
		locator = &os.Args[3]
	}

	fmt.Println("Opening session...")
	s, err := znet.ZOpen(locator, nil)
	if err != nil {
		panic(err.Error())
	}
	defer s.Close()

	fmt.Println("Declaring Publisher on '" + uri + "'...")
	pub, err := s.DeclarePublisher(uri)
	if err != nil {
		panic(err.Error())
	}
	defer s.UndeclarePublisher(pub)

	for idx := 0; idx < 100; idx++ {
		time.Sleep(1 * time.Second)
		str := fmt.Sprintf("[%4d] %s", idx, value)
		fmt.Printf("Streaming Data ('%s': '%s')...\n", uri, str)
		pub.StreamData([]byte(str))
	}
}
