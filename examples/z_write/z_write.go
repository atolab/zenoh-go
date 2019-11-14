package main

import (
	"fmt"
	"os"

	"github.com/atolab/zenoh-go"
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

	fmt.Println("Openning session...")
	z, err := zenoh.ZOpen(locator, nil)
	if err != nil {
		panic(err.Error())
	}

	fmt.Printf("Writing Data ('%s': '%s')...\n", uri, value)
	z.WriteData(uri, []byte(value))

	z.Close()
}
