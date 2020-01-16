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
	"strconv"

	znet "github.com/atolab/zenoh-go/net"
)

func main() {
	var locator *string
	if len(os.Args) < 2 {
		fmt.Printf("USAGE:\n\tzn_pub_thr <payload-size> [<zenoh-locator>]\n\n")
		os.Exit(-1)
	}

	length, err := strconv.Atoi(os.Args[1])
	if err != nil {
		panic(err.Error())
	}
	fmt.Printf("Running throughput test for payload of %d bytes\n", length)
	if len(os.Args) > 2 {
		locator = &os.Args[2]
	}

	data := make([]byte, length)
	for i := 0; i < length; i++ {
		data[i] = byte(i % 10)
	}

	s, err := znet.Open(locator, nil)
	if err != nil {
		panic(err.Error())
	}
	defer s.Close()

	pub, err := s.DeclarePublisher("/test/thr")
	if err != nil {
		panic(err.Error())
	}
	defer s.UndeclarePublisher(pub)

	for true {
		pub.StreamData(data)
	}

}
