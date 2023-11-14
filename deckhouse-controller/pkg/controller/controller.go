package controller

import (
	"context"
	"fmt"

	"k8s.io/client-go/util/retry"

	log "github.com/sirupsen/logrus"

	"github.com/flant/addon-operator/pkg/module_manager/models/modules/events"
	"github.com/flant/addon-operator/pkg/utils"
	"github.com/flant/addon-operator/pkg/values/validation"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/client/clientset/versioned"
)

type DeckhouseController struct {
	ctx context.Context

	dirs            []string
	valuesValidator *validation.ValuesValidator
	kubeClient      *versioned.Clientset

	deckhouseModules map[string]*DeckhouseModule
	// <module-name>: <module-source>
	sourceModule map[string]string
}

func NewDeckhouseController(ctx context.Context, config *rest.Config, moduleDirs string, vv *validation.ValuesValidator) (*DeckhouseController, error) {
	mcClient, err := versioned.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &DeckhouseController{
		ctx:             ctx,
		kubeClient:      mcClient,
		dirs:            utils.SplitToPaths(moduleDirs),
		valuesValidator: vv,

		deckhouseModules: make(map[string]*DeckhouseModule),
	}, nil
}

func (dml *DeckhouseController) Start(ec chan events.ModuleEvent) error {
	_ = dml.kubeClient.DeckhouseV1alpha1().Modules().Delete(dml.ctx, "common", v1.DeleteOptions{})
	_ = dml.kubeClient.DeckhouseV1alpha1().Modules().Delete(dml.ctx, "registrypackages", v1.DeleteOptions{})

	err := dml.RestoreAbsentSourceModules()
	if err != nil {
		return err
	}

	err = dml.searchAndLoadDeckhouseModules()
	if err != nil {
		return err
	}

	go dml.runEventLoop(ec)

	return nil
}

func (dml *DeckhouseController) runEventLoop(ec chan events.ModuleEvent) {
	for event := range ec {
		fmt.Println("GET EVENT", event)
		mod, ok := dml.deckhouseModules[event.ModuleName]
		if !ok {
			log.Errorf("Module %q registered but not found in Deckhouse. Possible bug?", mod.basic.GetName())
			continue
		}
		switch event.EventType {
		case events.ModuleRegistered:
			err := dml.handleModuleRegistration(mod)
			if err != nil {
				log.Errorf("Error occurred during the module %q registration: %s", mod.basic.GetName(), err)
				continue
			}

		case events.ModulePurged:
			err := dml.handleModulePurge(mod)
			if err != nil {
				log.Errorf("Error occurred during the module %q purge: %s", mod.basic.GetName(), err)
				continue
			}

		case events.ModuleEnabled:
			err := dml.handleEnabledModule(mod, true)
			if err != nil {
				log.Errorf("Error occurred during the module %q turning on: %s", mod.basic.GetName(), err)
				continue
			}

		case events.ModuleDisabled:
			err := dml.handleEnabledModule(mod, false)
			if err != nil {
				log.Errorf("Error occurred during the module %q turning off: %s", mod.basic.GetName(), err)
				continue
			}
		}
	}
}

func (dml *DeckhouseController) handleModulePurge(m *DeckhouseModule) error {
	return retry.OnError(retry.DefaultRetry, errors.IsServiceUnavailable, func() error {
		return dml.kubeClient.DeckhouseV1alpha1().Modules().Delete(dml.ctx, m.basic.GetName(), v1.DeleteOptions{})
	})
}

func (dml *DeckhouseController) handleModuleRegistration(m *DeckhouseModule) error {
	return retry.OnError(retry.DefaultRetry, errors.IsServiceUnavailable, func() error {
		source := dml.sourceModule[m.basic.GetName()]
		newModule := m.AsKubeObject(source)

		existModule, err := dml.kubeClient.DeckhouseV1alpha1().Modules().Get(dml.ctx, m.basic.GetName(), v1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				_, err = dml.kubeClient.DeckhouseV1alpha1().Modules().Create(dml.ctx, newModule, v1.CreateOptions{})
				return err
			}

			return err
		}

		existModule.Properties = newModule.Properties

		_, err = dml.kubeClient.DeckhouseV1alpha1().Modules().Update(dml.ctx, existModule, v1.UpdateOptions{})

		return err
	})
}

func (dml *DeckhouseController) handleEnabledModule(m *DeckhouseModule, enable bool) error {
	return retry.OnError(retry.DefaultRetry, errors.IsServiceUnavailable, func() error {
		obj, err := dml.kubeClient.DeckhouseV1alpha1().Modules().Get(dml.ctx, m.basic.GetName(), v1.GetOptions{})
		if err != nil {
			return err
		}

		obj.Properties.State = "Disabled"
		if enable {
			obj.Properties.State = "Enabled"
		}

		_, err = dml.kubeClient.DeckhouseV1alpha1().Modules().Update(dml.ctx, obj, v1.UpdateOptions{})
		if err != nil {
			return err
		}

		// Update ModuleConfig if exists
		mc, err := dml.kubeClient.DeckhouseV1alpha1().ModuleConfigs().Get(dml.ctx, m.basic.GetName(), v1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return nil
			}

			return err
		}

		mc.Status.Status = "Disabled"
		if enable {
			mc.Status.Status = "Enabled"
		}

		_, err = dml.kubeClient.DeckhouseV1alpha1().ModuleConfigs().Update(dml.ctx, mc, v1.UpdateOptions{})

		return err
	})
}
