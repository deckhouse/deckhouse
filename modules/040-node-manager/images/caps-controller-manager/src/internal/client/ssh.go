package client

import (
	"caps-controller-manager/internal/scope"
	"caps-controller-manager/internal/ssh"
	"caps-controller-manager/internal/ssh/clissh"
	"caps-controller-manager/internal/ssh/gossh"
)

func CreateSSHClient(instanceScope *scope.InstanceScope) (ssh.SSH, error) {
	name := instanceScope.InstanceName()
	addr := instanceScope.InstanceAddress()

	if instanceScope.SSHLegacyMode {
		instanceScope.Logger.Info("using clissh", "instance", name, "addr", addr)
		return clissh.CreateSSHClient(instanceScope)
	}

	instanceScope.Logger.Info("using gossh", "instance", name, "addr", addr)
	return gossh.CreateSSHClient(instanceScope)
}
