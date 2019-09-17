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

	uri := "/demo/example/zenoh-go-write"
	if len(os.Args) > 2 {
		uri = os.Args[2]
	}

	value := "Write from Go!"
	if len(os.Args) > 3 {
		value = os.Args[3]
	}

	fmt.Println("Connecting to " + locator + "...")
	z, err := zenoh.ZOpenWUP(locator, "user", "password")
	if err != nil {
		panic(err.Error())
	}

	fmt.Printf("Writing Data ('%s': '%s')...\n", uri, value)
	z.WriteData(uri, []byte(value))

	z.Close()
}
