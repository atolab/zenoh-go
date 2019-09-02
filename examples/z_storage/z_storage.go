package main

import (
	"fmt"
	"os"
	"time"

	"github.com/atolab/zenoh-go"
)

var stored map[string][]byte

func listener(rname string, data []byte, info *zenoh.DataInfo) {
	fmt.Printf("Received data: %s : [% x]\n", rname, data)
	stored[rname] = data
}

func queryHandler(rname string, predicate string, repliesSender *zenoh.RepliesSender) {
	fmt.Println("Handling Query: " + rname)
	replies := make([]zenoh.Resource, 0, len(stored))
	for k, v := range stored {
		var res zenoh.Resource
		res.RName = k
		res.Data = v
		res.Encoding = 0
		res.Kind = 0
		replies = append(replies, res)
	}

	repliesSender.SendReplies(replies)
}

func main() {
	stored = make(map[string][]byte)

	locator := "tcp/127.0.0.1:7447"
	if len(os.Args) > 1 {
		locator = os.Args[1]
	}

	uri := "/demo/**"
	if len(os.Args) > 2 {
		uri = os.Args[2]
	}

	fmt.Println("Connecting to " + locator + "...")
	z, err := zenoh.ZOpen("tcp/127.0.0.1:7447")
	if err != nil {
		panic(err.Error())
	}

	fmt.Println("Declaring Storage: " + uri)
	_, err = z.DeclareStorage(uri, listener, queryHandler)
	if err != nil {
		panic(err.Error())
	}

	for {
		time.Sleep(1 * time.Second)
	}

}
