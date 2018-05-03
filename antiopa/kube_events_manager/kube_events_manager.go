package kube_events_manager

import (
	"fmt"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/deckhouse/deckhouse/antiopa/kube"
	"github.com/deckhouse/deckhouse/antiopa/module_manager"
)

type KubeEventsManager interface {
	Run(config module_manager.KubeEventsConfig) (string, error)
	Stop(configId string) error
}

type MainKubeEventsManager struct {
	KubeEventCh                   chan string
	KubeEventsInformersByConfigId map[string][]*KubeEventsInformer
}

func NewMainKubeEventsManager() *MainKubeEventsManager {
	em := &MainKubeEventsManager{}
	em.KubeEventsInformersByConfigId = make(map[string][]*KubeEventsInformer)
	em.KubeEventCh = make(chan string, 1)
	return em
}

func Init() (KubeEventsManager, error) {
	em := NewMainKubeEventsManager()
	return em, nil
}

func (em *MainKubeEventsManager) Run(config module_manager.KubeEventsConfig) (string, error) {
	return "", nil
}

func (em *MainKubeEventsManager) AddOnAdd(config module_manager.KubeEventsConfig) (string, error) {
	configId, err := em.AddConfig(config, func(ei *KubeEventsInformer) cache.ResourceEventHandlerFuncs {
		return cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				configMap := obj.(*v1.ConfigMap)

				configMapId := fmt.Sprintf("%s-%s", configMap.Name, configMap.Namespace)
				if configMap.ResourceVersion != ei.Checksum[configMapId] {
					ei.Checksum[configMapId] = configMap.ResourceVersion
					em.KubeEventCh <- ei.ConfigId
				}
			},
		}
	})
	return configId, err
}

func (em *MainKubeEventsManager) AddOnUpdate(config module_manager.KubeEventsConfig) (string, error) {
	configId, err := em.AddConfig(config, func(ei *KubeEventsInformer) cache.ResourceEventHandlerFuncs {
		return cache.ResourceEventHandlerFuncs{
			UpdateFunc: func(_ interface{}, newObj interface{}) {
				configMap := newObj.(*v1.ConfigMap)

				configMapId := fmt.Sprintf("%s-%s", configMap.Name, configMap.Namespace)
				if configMap.ResourceVersion != ei.Checksum[configMapId] {
					ei.Checksum[configMapId] = configMap.ResourceVersion
					em.KubeEventCh <- ei.ConfigId
				}
			},
		}
	})
	return configId, err
}

func (em *MainKubeEventsManager) AddOnDelete(config module_manager.KubeEventsConfig) (string, error) {
	configId, err := em.AddConfig(config, func(ei *KubeEventsInformer) cache.ResourceEventHandlerFuncs {
		return cache.ResourceEventHandlerFuncs{
			DeleteFunc: func(obj interface{}) {
				em.KubeEventCh <- ei.ConfigId
			},
		}
	})
	return configId, err
}

func (em *MainKubeEventsManager) AddConfig(config module_manager.KubeEventsConfig, resourceEventHandlerFuncs func(ei *KubeEventsInformer) cache.ResourceEventHandlerFuncs) (string, error) {
	//var kubeEventsInformers []*KubeEventsInformer
	//if config.NamespaceSelector.Any {
	//	kubeEventsInformers = append(kubeEventsInformers, em.NewKubeEventsInformer(config.Kind, "", config.Selector))
	//} else {
	//	for _, namespace := range config.NamespaceSelector.MatchNames {
	//		kubeEventsInformers = append(kubeEventsInformers, em.NewKubeEventsInformer(config.Kind, namespace, config.Selector))
	//	}
	//}
	//
	//configId := uuid.NewV4().String()
	//for _, kubeEventsInformer := range kubeEventsInformers {
	//	kubeEventsInformer.Config = config
	//	kubeEventsInformer.ConfigId = configId
	//	kubeEventsInformer.SharedInformer.AddEventHandler(resourceEventHandlerFuncs(kubeEventsInformer))
	//	kubeEventsInformer.SharedInformer.Run(kubeEventsInformer.SharedInformerStop)
	//}
	//
	//em.KubeEventsInformersByConfigId[configId] = kubeEventsInformers
	//
	//return configId, nil

	return "", nil
}

func (em *MainKubeEventsManager) NewKubeEventsInformer(kind, namespace string, labelSelector *metav1.LabelSelector) *KubeEventsInformer {
	kubeEventsInformer := NewKubeEventsInformer()

	listOptions := &metav1.ListOptions{}

	if labelSelector != nil {
		listOptions.LabelSelector = labelSelector.String()
	}

	configMaps, _ := kube.KubernetesClient.CoreV1().ConfigMaps(namespace).List(*listOptions)
	for _, configMap := range configMaps.Items {
		configMapId := fmt.Sprintf("%s-%s", configMap.Name, configMap.Namespace)
		kubeEventsInformer.Checksum[configMapId] = configMap.ResourceVersion
	}

	optionsModifier := func(options *metav1.ListOptions) {
		if labelSelector != nil {
			*options = *listOptions
		}
	}

	restKubeClient := kube.KubernetesClient.CoreV1().RESTClient()
	lw := cache.NewFilteredListWatchFromClient(restKubeClient, kind, namespace, optionsModifier)

	kubeEventsInformer.SharedInformer = cache.NewSharedInformer(lw, &v1.ConfigMap{}, time.Duration(15)*time.Second)

	return kubeEventsInformer
}

func (em *MainKubeEventsManager) Stop(configId string) error {
	for _, ei := range em.KubeEventsInformersByConfigId[configId] {
		go ei.Stop()
	}
	return nil
}

type KubeEventsInformer struct {
	Config             module_manager.KubeEventsConfig
	ConfigId           string
	Checksum           map[string]string
	SharedInformer     cache.SharedInformer
	SharedInformerStop chan struct{}
}

func NewKubeEventsInformer() *KubeEventsInformer {
	ei := &KubeEventsInformer{}
	ei.Checksum = make(map[string]string)
	ei.SharedInformerStop = make(chan struct{}, 1)
	return ei
}

func (ei *KubeEventsInformer) Stop() {
	ei.SharedInformerStop <- struct{}{}
}
