package client

import (
	"fmt"

	"caps-controller-manager/internal/scope"

	"github.com/go-logr/logr"
)

func getLogger(i *scope.InstanceScope, operation string) logr.Logger {
	return i.Logger.WithName(fmt.Sprintf("%s %s %s", operation, i.InstanceName(), i.InstanceAddress()))
}
