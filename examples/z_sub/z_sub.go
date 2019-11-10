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
	var locator *string
	if len(os.Args) > 1 {
		locator = &os.Args[1]
	}

	uri := "/demo/example/**"
	if len(os.Args) > 2 {
		uri = os.Args[2]
	}

	fmt.Println("Openning session...")
	z, err := zenoh.ZOpen(locator, nil)
	if err != nil {
		panic(err.Error())
	}

	fmt.Println("Declaring Subscriber on '" + uri + "'...")
	sub, err := z.DeclareSubscriber(uri, zenoh.NewSubMode(zenoh.ZPushMode), listener)
	if err != nil {
		panic(err.Error())
	}

	reader := bufio.NewReader(os.Stdin)
	var c rune
	for c != 'q' {
		c, _, _ = reader.ReadRune()
	}

	z.UndeclareSubscriber(sub)
	z.Close()
}
