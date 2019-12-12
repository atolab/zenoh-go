package main

import (
	"bufio"
	"fmt"
	"os"

	znet "github.com/atolab/zenoh-go/net"
)

var stored map[string][]byte

func listener(rname string, data []byte, info *znet.DataInfo) {
	str := string(data)
	fmt.Printf(">> [Storage listener] Received ('%20s' : '%s')\n", rname, str)
	stored[rname] = data
}

func queryHandler(rname string, predicate string, repliesSender *znet.RepliesSender) {
	fmt.Printf(">> [Query handler   ] Handling '%s?%s'\n", rname, predicate)
	replies := make([]znet.Resource, 0, len(stored))
	for k, v := range stored {
		if znet.RNameIntersect(rname, k) {
			var res znet.Resource
			res.RName = k
			res.Data = v
			res.Encoding = 0
			res.Kind = 0
			replies = append(replies, res)
		}
	}

	repliesSender.SendReplies(replies)
}

func main() {
	stored = make(map[string][]byte)

	uri := "/demo/example/**"
	if len(os.Args) > 1 {
		uri = os.Args[1]
	}

	var locator *string
	if len(os.Args) > 2 {
		locator = &os.Args[2]
	}

	fmt.Println("Opening session...")
	s, err := znet.Open(locator, nil)
	if err != nil {
		panic(err.Error())
	}
	defer s.Close()

	fmt.Println("Declaring Storage on '" + uri + "'...")
	sto, err := s.DeclareStorage(uri, listener, queryHandler)
	if err != nil {
		panic(err.Error())
	}
	defer s.UndeclareStorage(sto)

	reader := bufio.NewReader(os.Stdin)
	var c rune
	for c != 'q' {
		c, _, _ = reader.ReadRune()
	}
}
