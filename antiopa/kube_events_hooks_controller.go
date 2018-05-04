package main

import (
	"fmt"

	"github.com/deckhouse/deckhouse/antiopa/kube_events_manager"
	"github.com/deckhouse/deckhouse/antiopa/module_manager"
	"github.com/deckhouse/deckhouse/antiopa/task"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type KubeEventHook struct {
	HookName string

	InformerType kube_events_manager.InformerType
	Kind         string
	Namespace    string
	Selector     *metav1.LabelSelector
	JqFilter     string

	Config module_manager.KubeEventsOnAction
}

func MakeKubeEventHookDescriptors(hook module_manager.Hook) []*KubeEventHook {
	if hook.KubeEvents == nil {
		return nil
	}

	res := make([]*KubeEventHook, 0)

	for _, data := range []struct {
		InformerType kube_events_manager.InformerType
		Configs      []*module_manager.KubeEventsOnAction
	}{
		{kube_events_manager.OnAdd, hook.KubeEvents.OnAdd},
		{kube_events_manager.OnUpdate, hook.KubeEvents.OnUpdate},
		{kube_events_manager.OnDelete, hook.KubeEvents.OnDelete},
	} {
		for _, config := range data.Configs {
			if config.NamespaceSelector == nil || config.NamespaceSelector.Any {
				res = append(res, &KubeEventHook{
					HookName:     hook.Name,
					InformerType: data.InformerType,
					Kind:         config.Kind,
					Namespace:    "",
					Selector:     config.Selector,
					JqFilter:     config.JqFilter,
				})
			} else {
				for _, namespace := range config.NamespaceSelector.MatchNames {
					res = append(res, &KubeEventHook{
						HookName:     hook.Name,
						InformerType: data.InformerType,
						Kind:         config.Kind,
						Namespace:    namespace,
						Selector:     config.Selector,
						JqFilter:     config.JqFilter,
					})
				}
			}

		}
	}

	return res
}

type KubeEventsHooksController struct {
	GlobalHooks map[string]*KubeEventHook
	ModuleHooks map[string]*KubeEventHook
}

func NewKubeEventsHooksController() *KubeEventsHooksController {
	obj := &KubeEventsHooksController{}
	obj.GlobalHooks = make(map[string]*KubeEventHook)
	obj.ModuleHooks = make(map[string]*KubeEventHook)
	return obj
}

func (obj *KubeEventsHooksController) EnableGlobalHooks(moduleManager module_manager.ModuleManager, eventsManager kube_events_manager.KubeEventsManager) error {
	globalHooks := moduleManager.GetGlobalHooksInOrder(module_manager.KubeEvents)

	for _, globalHookName := range globalHooks {
		globalHook, _ := ModuleManager.GetGlobalHook(globalHookName)

		for _, desc := range MakeKubeEventHookDescriptors(*globalHook.Hook) {
			configId, err := eventsManager.Run(desc.InformerType, desc.Kind, desc.Namespace, desc.Selector, desc.JqFilter)
			if err != nil {
				return err
			}
			obj.GlobalHooks[configId] = desc
		}
	}

	return nil
}

func (obj *KubeEventsHooksController) DisableAllHooks(eventsManager kube_events_manager.KubeEventsManager) error {
	var err error

	for configId := range obj.GlobalHooks {
		err = eventsManager.Stop(configId)
		if err != nil {
			return err
		}
	}

	for configId := range obj.ModuleHooks {
		err = eventsManager.Stop(configId)
		if err != nil {
			return err
		}
	}

	return nil
}

func (obj *KubeEventsHooksController) EnableModuleHooks(moduleName string, moduleManager module_manager.ModuleManager, eventsManager kube_events_manager.KubeEventsManager) error {
	//moduleHooks, _ := ModuleManager.GetModuleHooksInOrder(moduleName, module_manager.KubeEvents)
	//
	//for _, moduleHookName := range moduleHooks {
	//	moduleHook, _ := ModuleManager.GetModuleHook(moduleHookName)
	//
	//	for _, kubeEventsConfig := range moduleHook.KubeEvents {
	//		configId, err := eventsManager.Run(kubeEventsConfig)
	//		if err != nil {
	//			return err
	//		}
	//
	//		if obj.ModuleHooksByConfigId[configId] == nil {
	//			obj.ModuleHooksByConfigId[configId] = make([]string, 0)
	//		}
	//		obj.ModuleHooksByConfigId[configId] = append(obj.ModuleHooksByConfigId[configId], moduleHookName)
	//	}
	//}

	return nil
}

func (obj *KubeEventsHooksController) DisableModuleHooks(moduleName string, moduleManager module_manager.ModuleManager, eventsManager kube_events_manager.KubeEventsManager) error {
	//disabledModuleHooks, err := moduleManager.GetModuleHooksInOrder(moduleName, module_manager.KubeEvents)
	//if err != nil {
	//	return err
	//}
	//
	//for configId, moduleHookName := range obj.ModuleHooks {
	//	for _, disabledModuleHookName := range disabledModuleHooks {
	//		if moduleHookName == disabledModuleHookName {
	//			err := eventsManager.Stop(configId)
	//			if err != nil {
	//				return err
	//			}
	//
	//			delete(obj.ModuleHooks, configId)
	//
	//			break
	//		}
	//	}
	//}

	return nil
}

func (obj *KubeEventsHooksController) HandleEvent(configId string) (*struct{ Tasks []task.Task }, error) {
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
