package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/atolab/zenoh-go"
)

func listener(rid string, data []byte, info *zenoh.DataInfo) {
	str := string(data)
	fmt.Printf(">> [Subscription listener] Received ('%s': '%s')\n", rid, str)
}

func main() {
	locator := "tcp/127.0.0.1:7447"
	if len(os.Args) > 1 {
		locator = os.Args[1]
	}

	uri := "/demo/example/**"
	if len(os.Args) > 2 {
		uri = os.Args[2]
	}

	fmt.Println("Connecting to " + locator + "...")
	z, err := zenoh.ZOpenWUP(locator, "user", "password")
	if err != nil {
		panic(err.Error())
	}

	fmt.Println("Declaring Subscriber on '" + uri + "'...")
	sub, err := z.DeclareSubscriber(uri, zenoh.NewSubMode(zenoh.ZPullMode), listener)
	if err != nil {
		panic(err.Error())
	}

	reader := bufio.NewReader(os.Stdin)
	var c rune
	for c != 'q' {
		c, _, _ = reader.ReadRune()
		sub.Pull()
	}

	z.UndeclareSubscriber(sub)
	z.Close()
}
