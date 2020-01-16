/*
 * Copyright (c) 2014, 2020 Contributors to the Eclipse Foundation
 *
 * See the NOTICE file(s) distributed with this work for additional
 * information regarding copyright ownership.
 *
 * This program and the accompanying materials are made available under the
 * terms of the Eclipse Public License 2.0 which is available at
 * http://www.eclipse.org/legal/epl-2.0, or the Apache License, Version 2.0
 * which is available at https://www.apache.org/licenses/LICENSE-2.0.
 *
 * SPDX-License-Identifier: EPL-2.0 OR Apache-2.0
 *
 * Contributors: Julien Enoch, ADLINK Technology Inc.
 * Initial implementation of Eclipse Zenoh.
 */

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

	fmt.Println("Subscribe on " + selector)
	subid, err := w.Subscribe(s,
		func(changes []zenoh.Change) {
			for _, c := range changes {
				switch c.Kind() {
				case zenoh.PUT:
					fmt.Printf(">> [Subscription listener] Received PUT on '%s': '%s')\n", c.Path().ToString(), c.Value().ToString())
				case zenoh.UPDATE:
					fmt.Printf(">> [Subscription listener] Received UPDATE on '%s': '%s')\n", c.Path().ToString(), c.Value().ToString())
				case zenoh.REMOVE:
					fmt.Printf(">> [Subscription listener] Received REMOVE on '%s')\n", c.Path().ToString())
				default:
					fmt.Printf(">> [Subscription listener] Received unknown operation with kind '%d' on '%s')\n", c.Kind(), c.Path().ToString())
				}
			}
		})
	if err != nil {
		panic(err.Error())
	}

	fmt.Println("Enter 'q' to quit...")
	fmt.Println()
	var b = make([]byte, 1)
	for b[0] != 'q' {
		os.Stdin.Read(b)
	}

	err = w.Unsubscribe(subid)
	if err != nil {
		panic(err.Error())
	}

	err = y.Logout()
	if err != nil {
		panic(err.Error())
	}

}
