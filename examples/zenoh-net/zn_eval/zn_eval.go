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
	"bufio"
	"fmt"
	"os"

	znet "github.com/atolab/zenoh-go/net"
)

var uri string

func queryHandler(rname string, predicate string, repliesSender *znet.RepliesSender) {
	fmt.Printf(">> [Query handler] Handling '%s?%s'\n", rname, predicate)

	replies := make([]znet.Resource, 1, 1)
	replies[0].RName = uri
	replies[0].Data = []byte("Eval from Go!")
	replies[0].Encoding = 0
	replies[0].Kind = 0

	repliesSender.SendReplies(replies)
}

func main() {
	uri = "/demo/example/zenoh-go-eval"
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

	fmt.Println("Declaring Eval on '" + uri + "'...")
	e, err := s.DeclareEval(uri, queryHandler)
	if err != nil {
		panic(err.Error())
	}
	defer s.UndeclareEval(e)

	reader := bufio.NewReader(os.Stdin)
	var c rune
	for c != 'q' {
		c, _, _ = reader.ReadRune()
	}

}
