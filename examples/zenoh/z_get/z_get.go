package main

import (
	"fmt"
	"os"

	"github.com/atolab/zenoh-go"
)

func main() {
	selector := "/demo/example/**"
	if len(os.Args) > 1 {
		selector = os.Args[1]
	}

	var locator *string
	if len(os.Args) > 2 {
		locator = &os.Args[2]
	}

	s, err := zenoh.NewSelector(selector)
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

	fmt.Println("Get from " + s.ToString())
	for _, pv := range w.Get(s) {
		fmt.Println("  " + pv.Path().ToString() + " : " + pv.Value().ToString())
	}

	err = y.Logout()
	if err != nil {
		panic(err.Error())
	}

}
