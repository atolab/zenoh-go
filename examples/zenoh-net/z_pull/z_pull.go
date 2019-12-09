package main

import (
	"bufio"
	"fmt"
	"os"

	znet "github.com/atolab/zenoh-go/net"
)

func listener(rid string, data []byte, info *znet.DataInfo) {
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

	fmt.Println("Opening session...")
	s, err := znet.ZOpen(locator, nil)
	if err != nil {
		panic(err.Error())
	}
	defer s.Close()

	fmt.Println("Declaring Subscriber on '" + uri + "'...")
	sub, err := s.DeclareSubscriber(uri, znet.NewSubMode(znet.ZPullMode), listener)
	if err != nil {
		panic(err.Error())
	}
	defer s.UndeclareSubscriber(sub)

	fmt.Println("Press <enter> to pull data...")
	reader := bufio.NewReader(os.Stdin)
	var c rune
	for c != 'q' {
		c, _, _ = reader.ReadRune()
		sub.Pull()
	}
}
