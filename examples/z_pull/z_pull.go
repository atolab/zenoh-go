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

	fmt.Println("Declaring Subscriber on '" + uri + "'...")
	sub, err := z.DeclareSubscriber(uri, zenoh.NewSubMode(zenoh.ZPullMode), listener)
	if err != nil {
		panic(err.Error())
	}

	fmt.Println("Press <enter> to pull data...")
	reader := bufio.NewReader(os.Stdin)
	var c rune
	for c != 'q' {
		c, _, _ = reader.ReadRune()
		sub.Pull()
	}

	z.UndeclareSubscriber(sub)
	z.Close()
}
