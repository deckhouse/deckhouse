package main

import (
	"time"

	"github.com/romana/rlog"
)

var (
	// новый id образа с тем же именем
	// (смена самого имени образа будет обрабатываться самим Deployment'ом автоматом)
	ImageUpdated     chan string
	AntiopaImageId   string
	AntiopaImageName string
	PodName          string
)

// InitRegistryManager получает имя образа по имени пода и запрашивает id этого образа.
func InitRegistryManager() {
	ImageUpdated = make(chan string)
	AntiopaImageName = KubeGetPodImageName(Hostname)
	AntiopaImageId, _ = DockerRegistryGetImageId(AntiopaImageName)
}

// RunRegistryManager каждые 10 секунд проверяет
// не изменился ли id образа.
func RunRegistryManager() {
	rlog.Debug("Run registry manager")

	ticker := time.NewTicker(time.Duration(60) * time.Second)

	for {
		select {
		case <-ticker.C:
			rlog.Debugf("Checking registry for updates")
			imageID, err := DockerRegistryGetImageId(AntiopaImageName)
			if err != nil {
				rlog.Errorf("REGISTRY Cannot check image id: %v", err)
			} else {
				if imageID != AntiopaImageId {
					AntiopaImageId = imageID
					ImageUpdated <- imageID
				}
			}
		}
	}
}
