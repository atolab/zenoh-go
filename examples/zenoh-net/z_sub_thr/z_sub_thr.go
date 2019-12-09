package main

import (
	"fmt"
	"os"
	"time"

	znet "github.com/atolab/zenoh-go/net"
)

const n = 100000

var count uint64
var start, stop time.Time

func printStats(start time.Time, stop time.Time) {
	t0 := float64(start.UnixNano()) / 1000000000.0
	t1 := float64(stop.UnixNano()) / 1000000000
	thpt := float64(n) / (t1 - t0)
	fmt.Printf("%f msgs/sec\n", thpt)
}

func listener(rid string, data []byte, info *znet.DataInfo) {
	if count == 0 {
		start = time.Now()
		count++
	} else if count < n {
		count++
	} else {
		stop = time.Now()
		printStats(start, stop)
		count = 0
	}
}

func main() {
	var locator *string
	if len(os.Args) > 1 {
		locator = &os.Args[1]
	}

	s, err := znet.ZOpen(locator, nil)
	if err != nil {
		panic(err.Error())
	}
	defer s.Close()

	sub, err := s.DeclareSubscriber("/test/thr", znet.NewSubMode(znet.ZPushMode), listener)
	if err != nil {
		panic(err.Error())
	}
	defer s.UndeclareSubscriber(sub)

	time.Sleep(60 * time.Second)

}
