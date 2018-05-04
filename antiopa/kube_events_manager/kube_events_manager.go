package kube_events_manager

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/romana/rlog"
	"github.com/satori/go.uuid"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/deckhouse/deckhouse/antiopa/kube"
)

var (
	KubeEventCh chan string
)

type InformerType int

const (
	OnAdd InformerType = iota
	OnUpdate
	OnDelete
)

type KubeEventsManager interface {
	Run(informerType InformerType, kind, namespace string, labelSelector *metav1.LabelSelector) (string, error)
	Stop(configId string) error
}

type MainKubeEventsManager struct {
	KubeEventsInformersByConfigId map[string]*KubeEventsInformer
}

func NewMainKubeEventsManager() *MainKubeEventsManager {
	em := &MainKubeEventsManager{}
	em.KubeEventsInformersByConfigId = make(map[string]*KubeEventsInformer)
	return em
}

func Init() (KubeEventsManager, error) {
	em := NewMainKubeEventsManager()
	KubeEventCh = make(chan string, 1)
	return em, nil
}

func (em *MainKubeEventsManager) Run(informerType InformerType, kind, namespace string, labelSelector *metav1.LabelSelector) (string, error) {
	eventInformer, err := em.addInformer(kind, namespace, labelSelector, func(ei *KubeEventsInformer) cache.ResourceEventHandlerFuncs {
		resourceEventHandlerFuncs := cache.ResourceEventHandlerFuncs{}

		switch informerType {
		case OnAdd:
			resourceEventHandlerFuncs.AddFunc = func(obj interface{}) {
				configMap := obj.(*v1.ConfigMap)

				configMapId := fmt.Sprintf("%s-%s", configMap.Name, configMap.Namespace)
				configMapChecksum := md5OfMap(configMap.Data)
				if ei.Checksum[configMapId] != configMapChecksum {
					ei.Checksum[configMapId] = configMapChecksum
					KubeEventCh <- ei.ConfigId
				}
			}
		case OnUpdate:
			resourceEventHandlerFuncs.UpdateFunc = func(_ interface{}, newObj interface{}) {
				configMap := newObj.(*v1.ConfigMap)

				configMapId := fmt.Sprintf("%s-%s", configMap.Name, configMap.Namespace)
				configMapChecksum := md5OfMap(configMap.Data)
				if ei.Checksum[configMapId] != configMapChecksum {
					ei.Checksum[configMapId] = configMapChecksum
					KubeEventCh <- ei.ConfigId
				}
			}
		case OnDelete:
			resourceEventHandlerFuncs.DeleteFunc = func(obj interface{}) {
				KubeEventCh <- ei.ConfigId
			}
		}

		return resourceEventHandlerFuncs
	})

	if err != nil {
		return "", err
	}

	go eventInformer.Run()

	return eventInformer.ConfigId, nil
}

func (em *MainKubeEventsManager) addInformer(kind, namespace string, labelSelector *metav1.LabelSelector, resourceEventHandlerFuncs func(ei *KubeEventsInformer) cache.ResourceEventHandlerFuncs) (*KubeEventsInformer, error) {
	kubeEventsInformer := NewKubeEventsInformer()

	uid, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}
	configId := uid.String()

	kubeEventsInformer.ConfigId = configId
	kubeEventsInformer.SharedInformer.AddEventHandler(resourceEventHandlerFuncs(kubeEventsInformer))

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

	em.KubeEventsInformersByConfigId[configId] = kubeEventsInformer

	return kubeEventsInformer, nil
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
	ei, ok := em.KubeEventsInformersByConfigId[configId]
	if ok {
		ei.Stop()
	} else {
		rlog.Errorf("Kube events informer '%s' not found!", configId)
	}
	return nil
}

type KubeEventsInformer struct {
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

func (ei *KubeEventsInformer) Run() {
	ei.SharedInformer.Run(ei.SharedInformerStop)
}

func (ei *KubeEventsInformer) Stop() {
	ei.SharedInformerStop <- struct{}{}
}
