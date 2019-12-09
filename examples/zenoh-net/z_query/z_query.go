package main

import (
	"fmt"
	"os"
	"time"

	znet "github.com/atolab/zenoh-go/net"
)

func replyHandler(reply *znet.ReplyValue) {
	switch reply.Kind() {
	case znet.ZStorageData, znet.ZEvalData:
		str := string(reply.Data())
		switch reply.Kind() {
		case znet.ZStorageData:
			fmt.Printf(">> [Reply handler] received -Storage Data- ('%s': '%s')\n", reply.RName(), str)
		case znet.ZEvalData:
			fmt.Printf(">> [Reply handler] received -Eval Data-    ('%s': '%s')\n", reply.RName(), str)
		}

	case znet.ZStorageFinal:
		fmt.Println(">> [Reply handler] received -Storage Final-")

	case znet.ZEvalFinal:
		fmt.Println(">> [Reply handler] received -Eval Final-")

	case znet.ZReplyFinal:
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

	fmt.Println("Opening session...")
	s, err := znet.ZOpen(locator, nil)
	if err != nil {
		panic(err.Error())
	}
	defer s.Close()

	fmt.Println("Sending Query '" + uri + "'...")
	err = s.QueryWO(uri, "", replyHandler, znet.NewQueryDest(znet.ZAll), znet.NewQueryDest(znet.ZAll))
	if err != nil {
		panic(err.Error())
	}

	time.Sleep(1 * time.Second)
}
