package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/atolab/zenoh-go"
)

var uri string

func queryHandler(rname string, predicate string, repliesSender *zenoh.RepliesSender) {
	fmt.Printf(">> [Query handler] Handling '%s?%s'\n", rname, predicate)

	replies := make([]zenoh.Resource, 1, 1)
	replies[0].RName = uri
	replies[0].Data = []byte("Eval from Go!")
	replies[0].Encoding = 0
	replies[0].Kind = 0

	repliesSender.SendReplies(replies)
}

func main() {

	locator := "tcp/127.0.0.1:7447"
	if len(os.Args) > 1 {
		locator = os.Args[1]
	}

	uri = "/demo/example/zenoh-go-eval"
	if len(os.Args) > 2 {
		uri = os.Args[2]
	}

	fmt.Println("Connecting to " + locator + "...")
	z, err := zenoh.ZOpen("tcp/127.0.0.1:7447")
	if err != nil {
		panic(err.Error())
	}

	fmt.Println("Declaring Eval on '" + uri + "'...")
	e, err := z.DeclareEval(uri, queryHandler)
	if err != nil {
		panic(err.Error())
	}

	reader := bufio.NewReader(os.Stdin)
	var c rune
	for c != 'q' {
		c, _, _ = reader.ReadRune()
	}

	z.UndeclareEval(e)
	z.Close()
}