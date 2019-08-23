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

	uri := "/demo/hello/alpha"
	if len(os.Args) > 2 {
		uri = os.Args[2]
	}

	value := "Hello World!"
	if len(os.Args) > 3 {
		value = os.Args[3]
	}

	fmt.Println("Connecting to " + locator + "...")
	z, err := zenoh.ZOpen("tcp/127.0.0.1:7447")
	if err != nil {
		panic(err.Error())
	}

	fmt.Println("Declaring Publisher...")
	pub, err := z.DeclarePublisher(uri)
	if err != nil {
		panic(err.Error())
	}

	data := []byte(value)
	vle := zenoh.VleEncode(len(data))
	payload := append(vle, data...)

	fmt.Println("Streaming Data...")
	for true {
		pub.StreamData(payload)
		z.WriteData("/demo/hello/beta", payload)
		z.WriteData("/demo/hello/gamma", payload)
		z.WriteData("/demo/hello/eta", payload)
		time.Sleep(1 * time.Second)
	}

}
