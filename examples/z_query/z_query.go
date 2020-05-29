package main

import (
	"fmt"
	"os"
	"time"

	"github.com/atolab/zenoh-go"
)

func replyHandler(reply *zenoh.ReplyValue) {
	switch reply.Kind() {
	case zenoh.ZStorageData, zenoh.ZEvalData:
		str := string(reply.Data())
		switch reply.Kind() {
		case zenoh.ZStorageData:
			fmt.Printf(">> [Reply handler] received -Storage Data- ('%s': '%s')\n", reply.RName(), str)
		case zenoh.ZEvalData:
			fmt.Printf(">> [Reply handler] received -Eval Data-    ('%s': '%s')\n", reply.RName(), str)
		}

	case zenoh.ZStorageFinal:
		fmt.Println(">> [Reply handler] received -Storage Final-")

	case zenoh.ZEvalFinal:
		fmt.Println(">> [Reply handler] received -Eval Final-")

	case zenoh.ZReplyFinal:
		fmt.Println(">> [Reply handler] received -Reply Final-")
	}
}

func main() {
	uri := "/demo/example/**"
	if len(os.Args) > 1 {
		uri = os.Args[1]
	}

	var locator *string
	if len(os.Args) > 2 {
		locator = &os.Args[2]
	}

	fmt.Println("Openning session...")
	z, err := zenoh.ZOpen(locator, nil)
	if err != nil {
		panic(err.Error())
	}

	fmt.Println("Sending Query '" + uri + "'...")
	err = z.QueryWO(uri, "", replyHandler, zenoh.NewQueryDest(zenoh.ZAll), zenoh.NewQueryDest(zenoh.ZAll))
	if err != nil {
		panic(err.Error())
	}

	time.Sleep(1 * time.Second)

	z.Close()
}