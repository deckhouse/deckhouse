package main

import (
	"github.com/romana/rlog"
)

var (
	KubeNodesChanged chan bool
)

/*
 * Изменение состава nodes (add/delete)
 * Изменение манифеста известной нам node
 */

func InitKubeNodeManager() {
	rlog.Debug("Init kube node manager")
}

func RunKubeNodeManager() {
	rlog.Debug("Run kube node manager")
}
