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
	rlog.Info("Init config manager")
}

func RunKubeNodeManager() {
	rlog.Info("Run config manager")
}
