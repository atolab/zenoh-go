package main

import (
	"fmt"
	"os"

	znet "github.com/atolab/zenoh-go/net"
)

func main() {
	uri := "/demo/example/zenoh-go-write"
	if len(os.Args) > 1 {
		uri = os.Args[1]
	}

	value := "Write from Go!"
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

	fmt.Printf("Writing Data ('%s': '%s')...\n", uri, value)
	s.WriteData(uri, []byte(value))
}
