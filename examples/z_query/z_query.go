package main

import (
	"fmt"
	"os"
	"time"

	"github.com/atolab/zenoh-go"
)

func replyHandler(reply *zenoh.ReplyValue) {
	switch reply.Kind() {
	case zenoh.ZStorageData:
	case zenoh.ZEvalData:
		data := reply.Data()
		_, data = zenoh.VleDecode(data)
		switch reply.Kind() {
		case zenoh.ZStorageData:
			fmt.Printf("Received Storage Data. (%s, %s)\n", reply.RName(), string(data))
		case zenoh.ZEvalData:
			fmt.Printf("Received Eval   Data. (%s, %s)\n", reply.RName(), string(data))
		}

	case zenoh.ZStorageFinal:
		fmt.Println("Received Storage Final.")

	case zenoh.ZEvalFinal:
		fmt.Println("Received Eval Final.")

	case zenoh.ZReplyFinal:
		fmt.Println("Received Reply Final.")
	}

}

func main() {
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

	fmt.Println("Send Query...")
	err = z.Query(uri, "", replyHandler)
	if err != nil {
		panic(err.Error())
	}

	time.Sleep(100 * time.Millisecond)

}
