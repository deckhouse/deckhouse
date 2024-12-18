/*
Copyright 2021 Flant JSC

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

package scheduler

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
)

const (
	Namespace = "d8-upmeter"
)

type Deleter interface {
	Delete(string)
}

func newPersistentVolumeClaimDeleter(patcher go_hook.PatchCollector, logger go_hook.Logger) Deleter {
	deleter := &objDeleter{
		patcher:    patcher,
		apiVersion: "v1",
		kind:       "PersistentVolumeClaim",
		namespace:  Namespace,
	}
	message := func(pvcName string) string {
		return fmt.Sprintf("PVC %q marked for deletion", pvcName)
	}

	return newLoggingDeleter(deleter, logger, message)
}

func newStatefulSetDeleter(patcher go_hook.PatchCollector, logger go_hook.Logger) Deleter {
	deleter := &objDeleter{
		patcher:    patcher,
		apiVersion: "apps/v1",
		kind:       "StatefulSet",
		namespace:  Namespace,
	}
	message := func(stsName string) string {
		return fmt.Sprintf("StatefulSet %q marked for deletion", stsName)
	}

	return newLoggingDeleter(deleter, logger, message)
}

func newLoggingDeleter(delegate Deleter, logger go_hook.Logger, message func(string) string) Deleter {
	return &loggingDeleter{
		delegate: delegate,
		logger:   logger,
		message:  message,
	}
}

// loggingDeleter wraps a Deleter and logs about the deletion
type loggingDeleter struct {
	delegate Deleter
	logger   go_hook.Logger
	message  func(string) string
}

func (d *loggingDeleter) Delete(name string) {
	d.delegate.Delete(name)
	d.logger.Warn(d.message(name))
}

// objDeleter is the generic implementation of a Deleter interface
type objDeleter struct {
	patcher    go_hook.PatchCollector
	apiVersion string
	kind       string
	namespace  string
}

func (d *objDeleter) Delete(name string) {
	d.patcher.Delete(d.apiVersion, d.kind, d.namespace, name)
}
