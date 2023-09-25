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
