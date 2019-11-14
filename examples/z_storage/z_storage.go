package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/atolab/zenoh-go"
)

var stored map[string][]byte

func listener(rname string, data []byte, info *zenoh.DataInfo) {
	str := string(data)
	fmt.Printf(">> [Storage listener] Received ('%20s' : '%s')\n", rname, str)
	stored[rname] = data
}

func queryHandler(rname string, predicate string, repliesSender *zenoh.RepliesSender) {
	fmt.Printf(">> [Query handler   ] Handling '%s?%s'\n", rname, predicate)
	replies := make([]zenoh.Resource, 0, len(stored))
	for k, v := range stored {
		if zenoh.RNameIntersect(rname, k) {
			var res zenoh.Resource
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

	fmt.Println("Openning session...")
	z, err := zenoh.ZOpen(locator, nil)
	if err != nil {
		panic(err.Error())
	}

	fmt.Println("Declaring Storage on '" + uri + "'...")
	s, err := z.DeclareStorage(uri, listener, queryHandler)
	if err != nil {
		panic(err.Error())
	}

	reader := bufio.NewReader(os.Stdin)
	var c rune
	for c != 'q' {
		c, _, _ = reader.ReadRune()
	}

	z.UndeclareStorage(s)
	z.Close()
}
