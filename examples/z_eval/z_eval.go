package main

import (
	"fmt"
	"os"
	"time"

	"github.com/atolab/zenoh-go"
)

var uri string

func queryHandler(rname string, predicate string, repliesSender *zenoh.RepliesSender) {
	fmt.Println("Handling Query: " + rname)

	replies := make([]zenoh.Resource, 1, 1)
	replies[0].RName = uri
	replies[0].Data = []byte("\x0bEVAL_RES_GO")
	replies[0].Encoding = 0
	replies[0].Kind = 0

	repliesSender.SendReplies(replies)
}

func main() {

	locator := "tcp/127.0.0.1:7447"
	if len(os.Args) > 1 {
		locator = os.Args[1]
	}

	uri = "/demo/eval"
	if len(os.Args) > 2 {
		uri = os.Args[2]
	}

	fmt.Println("Connecting to " + locator + "...")
	z, err := zenoh.ZOpen("tcp/127.0.0.1:7447")
	if err != nil {
		panic(err.Error())
	}

	fmt.Println("Declaring Eval: " + uri)
	_, err = z.DeclareEval(uri, queryHandler)
	if err != nil {
		panic(err.Error())
	}

	for {
		time.Sleep(1 * time.Second)
	}

}
