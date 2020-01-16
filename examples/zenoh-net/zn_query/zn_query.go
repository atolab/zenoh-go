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
	"time"

	znet "github.com/atolab/zenoh-go/net"
)

func replyHandler(reply *znet.ReplyValue) {
	switch reply.Kind() {
	case znet.ZNStorageData, znet.ZNEvalData:
		str := string(reply.Data())
		switch reply.Kind() {
		case znet.ZNStorageData:
			fmt.Printf(">> [Reply handler] received -Storage Data- ('%s': '%s')\n", reply.RName(), str)
		case znet.ZNEvalData:
			fmt.Printf(">> [Reply handler] received -Eval Data-    ('%s': '%s')\n", reply.RName(), str)
		}

	case znet.ZNStorageFinal:
		fmt.Println(">> [Reply handler] received -Storage Final-")

	case znet.ZNEvalFinal:
		fmt.Println(">> [Reply handler] received -Eval Final-")

	case znet.ZNReplyFinal:
		fmt.Println(">> [Reply handler] received -Reply Final-")
	}
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
	s, err := znet.Open(locator, nil)
	if err != nil {
		panic(err.Error())
	}
	defer s.Close()

	fmt.Println("Sending Query '" + uri + "'...")
	err = s.QueryWO(uri, "", replyHandler, znet.NewQueryDest(znet.ZNAll), znet.NewQueryDest(znet.ZNAll))
	if err != nil {
		panic(err.Error())
	}

	time.Sleep(1 * time.Second)
}
