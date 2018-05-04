package kube_events_manager

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/satori/go.uuid"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/deckhouse/deckhouse/antiopa/kube"
	"github.com/deckhouse/deckhouse/antiopa/module_manager"
)

var (
	KubeEventCh chan string
)

type KubeEventsManager interface {
	Run(config *module_manager.KubeEventsConfig) (string, error)
	Stop(configId string) error
}

type MainKubeEventsManager struct {
	KubeEventsInformersByConfigId map[string][]*KubeEventsInformer
}

func NewMainKubeEventsManager() *MainKubeEventsManager {
	em := &MainKubeEventsManager{}
	em.KubeEventsInformersByConfigId = make(map[string][]*KubeEventsInformer)
	return em
}

func Init() (KubeEventsManager, error) {
	em := NewMainKubeEventsManager()
	KubeEventCh = make(chan string, 1)
	return em, nil
}

func (em *MainKubeEventsManager) Run(config *module_manager.KubeEventsConfig) (string, error) {
	uid, err := uuid.NewV4()
	if err != nil {
		return "", err
	}
	configId := uid.String()

	if config.OnAdd != nil {
		if err := em.addInformersOnAdd(configId, config.OnAdd); err != nil {
			return "", err
		}
	}

	if config.OnUpdate != nil {
		if err := em.addInformersOnUpdate(configId, config.OnAdd); err != nil {
			return "", err
		}
	}

	if config.OnDelete != nil {
		if err := em.addInformersOnDelete(configId, config.OnDelete); err != nil {
			return "", err
		}
	}

	return configId, nil
}

func (em *MainKubeEventsManager) addInformersOnAdd(configId string, config *module_manager.KubeEventsOnAction) error {
	return em.addInformers(configId, config, func(ei *KubeEventsInformer) cache.ResourceEventHandlerFuncs {
		return cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				configMap := obj.(*v1.ConfigMap)

				configMapId := fmt.Sprintf("%s-%s", configMap.Name, configMap.Namespace)
				configMapChecksum := md5OfMap(configMap.Data)
				if ei.Checksum[configMapId] != configMapChecksum {
					ei.Checksum[configMapId] = configMapChecksum
					KubeEventCh <- ei.ConfigId
				}
			},
		}
	})
}

func (em *MainKubeEventsManager) addInformersOnUpdate(configId string, config *module_manager.KubeEventsOnAction) error {
	return em.addInformers(configId, config, func(ei *KubeEventsInformer) cache.ResourceEventHandlerFuncs {
		return cache.ResourceEventHandlerFuncs{
			UpdateFunc: func(_ interface{}, newObj interface{}) {
				configMap := newObj.(*v1.ConfigMap)

				configMapId := fmt.Sprintf("%s-%s", configMap.Name, configMap.Namespace)
				configMapChecksum := md5OfMap(configMap.Data)
				if ei.Checksum[configMapId] != configMapChecksum {
					ei.Checksum[configMapId] = configMapChecksum
					KubeEventCh <- ei.ConfigId
				}
			},
		}
	})
}

func (em *MainKubeEventsManager) addInformersOnDelete(configId string, config *module_manager.KubeEventsOnAction) error {
	return em.addInformers(configId, config, func(ei *KubeEventsInformer) cache.ResourceEventHandlerFuncs {
		return cache.ResourceEventHandlerFuncs{
			DeleteFunc: func(obj interface{}) {
				KubeEventCh <- ei.ConfigId
			},
		}
	})
}

func (em *MainKubeEventsManager) addInformers(configId string, config *module_manager.KubeEventsOnAction, resourceEventHandlerFuncs func(ei *KubeEventsInformer) cache.ResourceEventHandlerFuncs) error {
	var kubeEventsInformers []*KubeEventsInformer
	if config.NamespaceSelector.Any {
		kubeEventsInformers = append(kubeEventsInformers, em.newInformer(config.Kind, "", config.Selector))
	} else {
		for _, namespace := range config.NamespaceSelector.MatchNames {
			kubeEventsInformers = append(kubeEventsInformers, em.newInformer(config.Kind, namespace, config.Selector))
		}
	}

	for _, kubeEventsInformer := range kubeEventsInformers {
		kubeEventsInformer.Config = config
		kubeEventsInformer.ConfigId = configId
		kubeEventsInformer.SharedInformer.AddEventHandler(resourceEventHandlerFuncs(kubeEventsInformer))
		kubeEventsInformer.SharedInformer.Run(kubeEventsInformer.SharedInformerStop)
	}

	em.KubeEventsInformersByConfigId[configId] = kubeEventsInformers

	return nil
}

func (em *MainKubeEventsManager) newInformer(kind, namespace string, labelSelector *metav1.LabelSelector) *KubeEventsInformer {
	kubeEventsInformer := NewKubeEventsInformer()

	listOptions := &metav1.ListOptions{}

	if labelSelector != nil {
		listOptions.LabelSelector = labelSelector.String()
	}

	configMaps, _ := kube.KubernetesClient.CoreV1().ConfigMaps(namespace).List(*listOptions)
	for _, configMap := range configMaps.Items {
		configMapId := fmt.Sprintf("%s-%s", configMap.Name, configMap.Namespace)
		kubeEventsInformer.Checksum[configMapId] = md5OfMap(configMap.Data)
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

func md5OfMap(obj map[string]string) string {
	data, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}

	h := md5.New()
	io.WriteString(h, string(data))

	return string(h.Sum(nil))
}

func (em *MainKubeEventsManager) Stop(configId string) error {
	for _, ei := range em.KubeEventsInformersByConfigId[configId] {
		go ei.Stop()
	}
	return nil
}

type KubeEventsInformer struct {
	Config             *module_manager.KubeEventsOnAction
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
