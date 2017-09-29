package main

import (
	"time"

	"github.com/romana/rlog"
)

var ()

func Init() {
	rlog.Info("Init")

	InitConfigManager()
}

func Run() {
	rlog.Info("Run")

	go RunConfigManager()

	for {
		time.Sleep(time.Duration(1) * time.Second)
	}
}

func main() {
	Init()
	Run()
}
