/*
Copyright 2023 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"
)

func main() {
	var server string
	flag.StringVar(&server, "server", "127.0.0.1:10254", "server address:port to check open tcp port")
	flag.Parse()

	for {
		_, err := net.DialTimeout("tcp", server, time.Second*1)
		if err != nil {
			fmt.Println(err)
			os.Exit(0)
		}
		time.Sleep(time.Second * 1)
	}
}
