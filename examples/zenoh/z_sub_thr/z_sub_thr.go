package main

import (
	"fmt"
	"os"
	"time"

	"github.com/atolab/zenoh-go"
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

func listener(changes []zenoh.Change) {
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

	s, err := zenoh.NewSelector("/test/thr")
	if err != nil {
		panic(err.Error())
	}

	fmt.Println("Login to Zenoh...")
	y, err := zenoh.Login(locator, nil)
	if err != nil {
		panic(err.Error())
	}

	fmt.Println("Use Workspace on '/'")
	root, _ := zenoh.NewPath("/")
	w := y.Workspace(root)

	fmt.Println("Subscribe on " + s.ToString())
	subid, err := w.Subscribe(s, listener)
	if err != nil {
		panic(err.Error())
	}

	time.Sleep(60 * time.Second)

	w.Unsubscribe(subid)

	err = y.Logout()
	if err != nil {
		panic(err.Error())
	}

}
