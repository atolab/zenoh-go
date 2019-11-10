package main

import (
	"fmt"
	"os"

	"github.com/atolab/zenoh-go"
)

func main() {
	var locator *string
	if len(os.Args) > 1 {
		locator = &os.Args[1]
	}

	uri := "/demo/example/zenoh-go-write"
	if len(os.Args) > 2 {
		uri = os.Args[2]
	}

	value := "Write from Go!"
	if len(os.Args) > 3 {
		value = os.Args[3]
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
