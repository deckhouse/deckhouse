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
	Name     string

	EventTypes   []module_manager.OnKubernetesEventType
	Kind         string
	Namespace    string
	Selector     *metav1.LabelSelector
	JqFilter     string
	AllowFailure bool
	Debug        bool

	Config module_manager.OnKubernetesEventConfig
}

func MakeKubeEventHookDescriptors(hook *module_manager.Hook, hookConfig *module_manager.HookConfig) []*KubeEventHook {
	res := make([]*KubeEventHook, 0)

	for _, config := range hookConfig.OnKubernetesEvent {
		if config.NamespaceSelector.Any {
			res = append(res, ConvertOnKubernetesEventToKubeEventHook(hook, config, ""))
		} else {
			for _, namespace := range config.NamespaceSelector.MatchNames {
				res = append(res, ConvertOnKubernetesEventToKubeEventHook(hook, config, namespace))
			}
		}
	}

	return res
}

func ConvertOnKubernetesEventToKubeEventHook(hook *module_manager.Hook, config module_manager.OnKubernetesEventConfig, namespace string) *KubeEventHook {
	return &KubeEventHook{
		HookName:     hook.Name,
		Name:         config.Name,
		EventTypes:   config.EventTypes,
		Kind:         config.Kind,
		Namespace:    namespace,
		Selector:     config.Selector,
		JqFilter:     config.JqFilter,
		AllowFailure: config.AllowFailure,
		Debug:        !config.DisableDebug,
	}
}

type KubeEventsHooksController interface {
	EnableGlobalHooks(moduleManager module_manager.ModuleManager, eventsManager kube_events_manager.KubeEventsManager) error
	EnableModuleHooks(moduleName string, moduleManager module_manager.ModuleManager, eventsManager kube_events_manager.KubeEventsManager) error
	DisableModuleHooks(moduleName string, moduleManager module_manager.ModuleManager, eventsManager kube_events_manager.KubeEventsManager) error
	HandleEvent(kubeEvent kube_events_manager.KubeEvent) (*struct{ Tasks []task.Task }, error)
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
			configId, err := eventsManager.Run(desc.EventTypes, desc.Kind, desc.Namespace, desc.Selector, desc.JqFilter, desc.Debug)
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
			configId, err := eventsManager.Run(desc.EventTypes, desc.Kind, desc.Namespace, desc.Selector, desc.JqFilter, desc.Debug)
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

func (obj *MainKubeEventsHooksController) HandleEvent(kubeEvent kube_events_manager.KubeEvent) (*struct{ Tasks []task.Task }, error) {
	res := &struct{ Tasks []task.Task }{Tasks: make([]task.Task, 0)}
	var desc *KubeEventHook
	var taskType task.TaskType

	if moduleDesc, hasKey := obj.ModuleHooks[kubeEvent.ConfigId]; hasKey {
		desc = moduleDesc
		taskType = task.ModuleHookRun
	} else if globalDesc, hasKey := obj.GlobalHooks[kubeEvent.ConfigId]; hasKey {
		desc = globalDesc
		taskType = task.GlobalHookRun
	}

	if desc != nil && taskType != "" {
		bindingName := desc.Name
		if desc.Name == "" {
			bindingName = module_manager.ContextBindingType[module_manager.KubeEvents]
		}
		newTask := task.NewTask(taskType, desc.HookName).
			WithBinding(module_manager.KubeEvents).
			WithBindingContext(module_manager.BindingContext{
				Binding:           bindingName,
				ResourceEvent:     kubeEvent.Event,
				ResourceNamespace: kubeEvent.Namespace,
				ResourceKind:      kubeEvent.Kind,
				ResourceName:      kubeEvent.Name,
			}).
			WithAllowFailure(desc.Config.AllowFailure)

		res.Tasks = append(res.Tasks, newTask)
	} else {
		return nil, fmt.Errorf("unknown kube event: no such config id '%s' registered", kubeEvent.ConfigId)
	}

	return res, nil
}
