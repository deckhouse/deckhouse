package main

import (
	"fmt"

	"github.com/deckhouse/deckhouse/antiopa/kube_events_manager"
	"github.com/deckhouse/deckhouse/antiopa/module_manager"
	"github.com/deckhouse/deckhouse/antiopa/task"

	"github.com/romana/rlog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type KubeEventHook struct {
	HookName string

	EventTypes []module_manager.OnKubernetesEventType
	Kind       string
	Namespace  string
	Selector   *metav1.LabelSelector
	JqFilter   string

	Config module_manager.OnKubernetesEventConfig
}

func MakeKubeEventHookDescriptors(hook *module_manager.Hook, hookConfig *module_manager.HookConfig) []*KubeEventHook {
	res := make([]*KubeEventHook, 0)

	for _, config := range hookConfig.OnKubernetesEvent {
		if config.NamespaceSelector.Any {
			res = append(res, &KubeEventHook{
				HookName:   hook.Name,
				EventTypes: config.EventTypes,
				Kind:       config.Kind,
				Namespace:  "",
				Selector:   config.Selector,
				JqFilter:   config.JqFilter,
			})
		} else {
			for _, namespace := range config.NamespaceSelector.MatchNames {
				res = append(res, &KubeEventHook{
					HookName:   hook.Name,
					EventTypes: config.EventTypes,
					Kind:       config.Kind,
					Namespace:  namespace,
					Selector:   config.Selector,
					JqFilter:   config.JqFilter,
				})
			}
		}
	}

	return res
}

type KubeEventsHooksController interface {
	EnableGlobalHooks(moduleManager module_manager.ModuleManager, eventsManager kube_events_manager.KubeEventsManager) error
	EnableModuleHooks(moduleName string, moduleManager module_manager.ModuleManager, eventsManager kube_events_manager.KubeEventsManager) error
	DisableModuleHooks(moduleName string, moduleManager module_manager.ModuleManager, eventsManager kube_events_manager.KubeEventsManager) error
	HandleEvent(configId string) (*struct{ Tasks []task.Task }, error)
}

type MainKubeEventsHooksController struct {
	GlobalHooks    map[string]*KubeEventHook
	ModuleHooks    map[string]*KubeEventHook
	EnabledModules []string
}

func NewMainKubeEventsHooksController() *MainKubeEventsHooksController {
	obj := &MainKubeEventsHooksController{}
	obj.GlobalHooks = make(map[string]*KubeEventHook)
	obj.ModuleHooks = make(map[string]*KubeEventHook)
	obj.EnabledModules = make([]string, 0)
	return obj
}

func (obj *MainKubeEventsHooksController) EnableGlobalHooks(moduleManager module_manager.ModuleManager, eventsManager kube_events_manager.KubeEventsManager) error {
	globalHooks := moduleManager.GetGlobalHooksInOrder(module_manager.KubeEvents)

	for _, globalHookName := range globalHooks {
		globalHook, _ := ModuleManager.GetGlobalHook(globalHookName)

		for _, desc := range MakeKubeEventHookDescriptors(globalHook.Hook, &globalHook.Config.HookConfig) {
			configId, err := eventsManager.Run(desc.EventTypes, desc.Kind, desc.Namespace, desc.Selector, desc.JqFilter)
			if err != nil {
				return err
			}
			obj.GlobalHooks[configId] = desc

			rlog.Debugf("main: run informer %s for global hook %s", configId, globalHook.Name)
		}
	}

	return nil
}

func (obj *MainKubeEventsHooksController) EnableModuleHooks(moduleName string, moduleManager module_manager.ModuleManager, eventsManager kube_events_manager.KubeEventsManager) error {
	for _, enabledModuleName := range obj.EnabledModules {
		if enabledModuleName == moduleName {
			// already enabled
			return nil
		}
	}

	moduleHooks, err := ModuleManager.GetModuleHooksInOrder(moduleName, module_manager.KubeEvents)
	if err != nil {
		return err
	}

	for _, moduleHookName := range moduleHooks {
		moduleHook, _ := ModuleManager.GetModuleHook(moduleHookName)

		for _, desc := range MakeKubeEventHookDescriptors(moduleHook.Hook, &moduleHook.Config.HookConfig) {
			configId, err := eventsManager.Run(desc.EventTypes, desc.Kind, desc.Namespace, desc.Selector, desc.JqFilter)
			if err != nil {
				return err
			}
			obj.ModuleHooks[configId] = desc

			rlog.Debugf("main: run informer %s for module hook %s", configId, moduleHook.Name)
		}
	}

	obj.EnabledModules = append(obj.EnabledModules, moduleName)

	return nil
}

func (obj *MainKubeEventsHooksController) DisableModuleHooks(moduleName string, moduleManager module_manager.ModuleManager, eventsManager kube_events_manager.KubeEventsManager) error {
	moduleEnabledInd := -1
	for i, enabledModuleName := range obj.EnabledModules {
		if enabledModuleName == moduleName {
			moduleEnabledInd = i
			break
		}
	}
	if moduleEnabledInd < 0 {
		return nil
	}
	obj.EnabledModules = append(obj.EnabledModules[:moduleEnabledInd], obj.EnabledModules[moduleEnabledInd+1:]...)

	disabledModuleHooks, err := moduleManager.GetModuleHooksInOrder(moduleName, module_manager.KubeEvents)
	if err != nil {
		return err
	}

	for configId, desc := range obj.ModuleHooks {
		for _, disabledModuleHookName := range disabledModuleHooks {
			if desc.HookName == disabledModuleHookName {
				err := eventsManager.Stop(configId)
				if err != nil {
					return err
				}

				delete(obj.ModuleHooks, configId)

				break
			}
		}
	}

	return nil
}

func (obj *MainKubeEventsHooksController) HandleEvent(configId string) (*struct{ Tasks []task.Task }, error) {
	res := &struct{ Tasks []task.Task }{Tasks: make([]task.Task, 0)}

	if desc, hasKey := obj.ModuleHooks[configId]; hasKey {
		newTask := task.NewTask(task.ModuleHookRun, desc.HookName).
			WithBinding(module_manager.KubeEvents).
			WithAllowFailure(desc.Config.AllowFailure)

		res.Tasks = append(res.Tasks, newTask)
	} else if desc, hasKey := obj.GlobalHooks[configId]; hasKey {
		newTask := task.NewTask(task.GlobalHookRun, desc.HookName).
			WithBinding(module_manager.KubeEvents).
			WithAllowFailure(desc.Config.AllowFailure)

		res.Tasks = append(res.Tasks, newTask)
	} else {
		return nil, fmt.Errorf("unknown kube event: no such config id '%s' registered", configId)
	}

	return res, nil
}
