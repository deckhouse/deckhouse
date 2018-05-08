package kube_events_manager

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/romana/rlog"
	"gopkg.in/satori/go.uuid.v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
	Run(informerType InformerType, kind, namespace string, labelSelector *metav1.LabelSelector, jqFilter string) (string, error)
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

func (em *MainKubeEventsManager) Run(informerType InformerType, kind, namespace string, labelSelector *metav1.LabelSelector, jqFilter string) (string, error) {
	kubeEventsInformer, err := em.addKubeEventsInformer(kind, namespace, labelSelector, jqFilter, func(kubeEventsInformer *KubeEventsInformer) cache.ResourceEventHandlerFuncs {
		return cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				checksum, err := resourceMd5(obj, jqFilter)
				if err != nil {
					rlog.Error("Kube events manager: %s", err)
				} else {
					err = kubeEventsInformer.HandleKubeEvent(obj, checksum, informerType == OnAdd)
					if err != nil {
						rlog.Error("Kube events manager: %s", err)
					}
				}
			},
			UpdateFunc: func(_ interface{}, obj interface{}) {
				checksum, err := resourceMd5(obj, jqFilter)
				if err != nil {
					rlog.Error("Kube events manager: %s", err)
				} else {
					err := kubeEventsInformer.HandleKubeEvent(obj, checksum, informerType == OnUpdate)
					if err != nil {
						rlog.Error("Kube events manager: %s", err)
					}
				}
			},
			DeleteFunc: func(obj interface{}) {
				err := kubeEventsInformer.HandleKubeEvent(obj, "", informerType == OnDelete)
				if err != nil {
					rlog.Error("Kube events manager: %s", err)
				}
			},
		}
	})

	if err != nil {
		return "", err
	}

	go kubeEventsInformer.Run()

	return kubeEventsInformer.ConfigId, nil
}

func (em *MainKubeEventsManager) addKubeEventsInformer(kind, namespace string, labelSelector *metav1.LabelSelector, jqFilter string, resourceEventHandlerFuncs func(kubeEventsInformer *KubeEventsInformer) cache.ResourceEventHandlerFuncs) (*KubeEventsInformer, error) {
	kubeEventsInformer := NewKubeEventsInformer()

	listOptions := metav1.ListOptions{}
	if labelSelector != nil {
		listOptions.LabelSelector = labelSelector.String()
	}

	var runtimeObj runtime.Object
	switch kind {
	case "configmaps":
		runtimeObj = &v1.ConfigMap{}

		configMapList, err := kube.KubernetesClient.CoreV1().ConfigMaps(namespace).List(listOptions)
		if err != nil {
			return nil, fmt.Errorf("failed list resources: %s", err)
		}

		for _, resource := range configMapList.Items {
			resourceId := generateChecksumId(resource.Name, resource.Namespace)
			if checksum, err := resourceMd5(resource, jqFilter); err != nil {
				return nil, fmt.Errorf("failed resource md5: %s", err)
			} else {
				kubeEventsInformer.Checksum[resourceId] = checksum
			}
		}
	case "pods":
		runtimeObj = &v1.Pod{}

		podList, err := kube.KubernetesClient.CoreV1().Pods(namespace).List(listOptions)
		if err != nil {
			return nil, fmt.Errorf("failed list resources: %s", err)
		}

		for _, resource := range podList.Items {
			resourceId := generateChecksumId(resource.Name, resource.Namespace)
			if checksum, err := resourceMd5(resource, jqFilter); err != nil {
				return nil, fmt.Errorf("failed resource md5: %s", err)
			} else {
				kubeEventsInformer.Checksum[resourceId] = checksum
			}
		}
	case "endpoints":
		runtimeObj = &v1.Endpoints{}

		endpointList, err := kube.KubernetesClient.CoreV1().Endpoints(namespace).List(listOptions)
		if err != nil {
			return nil, fmt.Errorf("failed list resources: %s", err)
		}

		for _, resource := range endpointList.Items {
			resourceId := generateChecksumId(resource.Name, resource.Namespace)
			if checksum, err := resourceMd5(resource, jqFilter); err != nil {
				return nil, fmt.Errorf("failed resource md5: %s", err)
			} else {
				kubeEventsInformer.Checksum[resourceId] = checksum
			}
		}
	case "services":
		runtimeObj = &v1.Service{}

		serviceList, err := kube.KubernetesClient.CoreV1().Services(namespace).List(listOptions)
		if err != nil {
			return nil, fmt.Errorf("failed list resources: %s", err)
		}

		for _, resource := range serviceList.Items {
			resourceId := generateChecksumId(resource.Name, resource.Namespace)
			if checksum, err := resourceMd5(resource, jqFilter); err != nil {
				return nil, fmt.Errorf("failed resource md5: %s", err)
			} else {
				kubeEventsInformer.Checksum[resourceId] = checksum
			}
		}
	case "serviceaccounts":
		runtimeObj = &v1.ServiceAccount{}

		serviceAccountList, err := kube.KubernetesClient.CoreV1().ServiceAccounts(namespace).List(listOptions)
		if err != nil {
			return nil, fmt.Errorf("failed list resources: %s", err)
		}

		for _, resource := range serviceAccountList.Items {
			resourceId := generateChecksumId(resource.Name, resource.Namespace)
			if checksum, err := resourceMd5(resource, jqFilter); err != nil {
				return nil, fmt.Errorf("failed resource md5: %s", err)
			} else {
				kubeEventsInformer.Checksum[resourceId] = checksum
			}
		}
	case "replicationcontrollers":
		runtimeObj = &v1.ReplicationController{}

		replicationControllerList, err := kube.KubernetesClient.CoreV1().ReplicationControllers(namespace).List(listOptions)
		if err != nil {
			return nil, fmt.Errorf("failed list resources: %s", err)
		}

		for _, resource := range replicationControllerList.Items {
			resourceId := generateChecksumId(resource.Name, resource.Namespace)
			if checksum, err := resourceMd5(resource, jqFilter); err != nil {
				return nil, fmt.Errorf("failed resource md5: %s", err)
			} else {
				kubeEventsInformer.Checksum[resourceId] = checksum
			}
		}
	default:
		return nil, fmt.Errorf("kind '%s' isn't supported", kind)
	}

	optionsModifier := func(options *metav1.ListOptions) {
		if labelSelector != nil {
			options.LabelSelector = labelSelector.String()
		}
	}

	restKubeClient := kube.KubernetesClient.CoreV1().RESTClient()
	lw := cache.NewFilteredListWatchFromClient(restKubeClient, kind, namespace, optionsModifier)

	kubeEventsInformer.SharedInformer = cache.NewSharedInformer(lw, runtimeObj, time.Duration(15)*time.Second)
	kubeEventsInformer.SharedInformer.AddEventHandler(resourceEventHandlerFuncs(kubeEventsInformer))
	kubeEventsInformer.ConfigId = uuid.NewV4().String()

	em.KubeEventsInformersByConfigId[kubeEventsInformer.ConfigId] = kubeEventsInformer

	return kubeEventsInformer, nil
}

func resourceMd5(obj interface{}, jqFilter string) (string, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}

	var res string
	if jqFilter != "" {
		stdout, stderr, err := execJq(jqFilter, data)
		if err != nil {
			return "", fmt.Errorf("failed exec jq: \nerr: '%s'\nstderr: '%s'", err, stderr)
		}

		res = stdout
	} else {
		res = string(data)
	}

	h := md5.New()
	io.WriteString(h, res)

	return string(h.Sum(nil)), nil
}

func (em *MainKubeEventsManager) Stop(configId string) error {
	kubeEventsInformer, ok := em.KubeEventsInformersByConfigId[configId]
	if ok {
		kubeEventsInformer.Stop()
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
	kubeEventsInformer := &KubeEventsInformer{}
	kubeEventsInformer.Checksum = make(map[string]string)
	kubeEventsInformer.SharedInformerStop = make(chan struct{}, 1)
	return kubeEventsInformer
}

func (ei *KubeEventsInformer) HandleKubeEvent(obj interface{}, newChecksum string, sendSignal bool) error {
	objectId, err := runtimeResourceId(obj.(runtime.Object))
	if err != nil {
		return fmt.Errorf("failed to get object id: %s", err)
	}

	if ei.Checksum[objectId] != newChecksum {
		ei.Checksum[objectId] = newChecksum

		if sendSignal {
			KubeEventCh <- ei.ConfigId
		}
	}

	return nil
}

func runtimeResourceId(obj interface{}) (string, error) {
	runtimeObj := obj.(runtime.Object)
	accessor := meta.NewAccessor()

	name, err := accessor.Name(runtimeObj)
	if err != nil {
		return "", err
	}

	namespace, err := accessor.Namespace(runtimeObj)
	if err != nil {
		return "", err
	}

	return generateChecksumId(name, namespace), nil
}

func generateChecksumId(name, namespace string) string {
	return fmt.Sprintf("%s-%s", name, namespace)
}

func (ei *KubeEventsInformer) Run() {
	ei.SharedInformer.Run(ei.SharedInformerStop)
}

func (ei *KubeEventsInformer) Stop() {
	ei.SharedInformerStop <- struct{}{}
}

func execJq(jqFilter string, jsonData []byte) (stdout string, stderr string, err error) {
	cmd := exec.Command("/usr/bin/jq", jqFilter)

	var stdinBuf bytes.Buffer
	_, err = stdinBuf.WriteString(string(jsonData))
	if err != nil {
		panic(err)
	}
	cmd.Stdin = &stdinBuf
	var stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	err = cmd.Run()
	stdout = strings.TrimSpace(stdoutBuf.String())
	stderr = strings.TrimSpace(stderrBuf.String())

	return
}
