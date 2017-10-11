package main

import (
	"github.com/romana/rlog"
)

var (
	// новый id образа с тем же именем
	// (смена самого имени образа будет обрабатываться самим Deployment'ом автоматом)
	ImageUpdated chan string
)

func InitRegistryManager() {
	ImageUpdated = make(chan string)

	// запросить у куба и положить в локальную переменную (чтобы больше не лазить лишний раз в куб)
	// текущий id образа
}

func RunRegistryManager() {
	ticker := time.NewTicker(time.Duration(10) * time.Second)

	for {
		select {
		case <-ticker.C:
			rlog.Debugf("Checking registry for updates")
		}
	}
}
