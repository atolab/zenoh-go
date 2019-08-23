package main

import (
	"fmt"
	"os"
	"time"

	"github.com/atolab/zenoh-go"
)

func listener(rid string, data []byte, info *zenoh.DataInfo) {
	_, data = zenoh.VleDecode(data)
	fmt.Printf(">>: (%s, %s)\n", rid, string(data))
}

func main() {
	locator := "tcp/127.0.0.1:7447"
	if len(os.Args) > 1 {
		locator = os.Args[1]
	}

	uri := "/demo/hello/*"
	if len(os.Args) > 2 {
		uri = os.Args[2]
	}

	fmt.Println("Connecting to " + locator + "...")
	z, err := zenoh.ZOpen("tcp/127.0.0.1:7447")
	if err != nil {
		panic(err.Error())
	}

	fmt.Println("Declaring Subscriber...")
	_, err = z.DeclareSubscriber(uri, zenoh.NewSubMode(zenoh.ZPushMode), listener)
	if err != nil {
		panic(err.Error())
	}

	time.Sleep(3 * time.Second)

}
