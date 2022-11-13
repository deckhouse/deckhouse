package kinds

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	"github.com/flant/constraint_exporter/pkg/gatekeeper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

const (
	checksumAnnotation = "security.deckhouse.io/constraints-checksum"
)

type KindTracker struct {
	client      *kubernetes.Clientset
	cmNamespace string
	cmName      string

	latestChecksum string
}

func NewKindTracker(client *kubernetes.Clientset, cmNS, cmName string) *KindTracker {
	return &KindTracker{
		client:      client,
		cmNamespace: cmNS,
		cmName:      cmName,
	}
}

func deduplicateKinds(constraints []gatekeeper.Constraint) (map[string]gatekeeper.MatchKind /*kinds checksum*/, string) {
	if len(constraints) == 0 {
		return nil, ""
	}

	// deduplicate
	m := make(map[string]gatekeeper.MatchKind, 0)
	hasher := sha256.New()

	for _, con := range constraints {
		for _, k := range con.Spec.Match.Kinds {
			sort.Strings(k.APIGroups)
			sort.Strings(k.Kinds)
			key := fmt.Sprintf("%s:%s", strings.Join(k.APIGroups, ","), strings.Join(k.Kinds, ","))

			if _, ok := m[key]; !ok {
				m[key] = k
				hasher.Write([]byte(key))
			}
		}
	}

	return m, fmt.Sprintf("%x", hasher.Sum(nil))
}

func (kt *KindTracker) UpdateKinds(constraints []gatekeeper.Constraint) {
	if len(constraints) == 0 {
		return
	}

	deduplicated, checksum := deduplicateKinds(constraints)

	if len(deduplicated) == 0 {
		return
	}

	if checksum == kt.latestChecksum {
		return
	}

	klog.Info("Checksum is not equal. Updating")

	cm, err := kt.client.CoreV1().ConfigMaps(kt.cmNamespace).Get(context.TODO(), kt.cmName, v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			err = kt.createCM()
			if err != nil {
				klog.Errorf("Create kinds cm failed: %s", err)
				return
			}
		} else {
			klog.Errorf("Get kinds cm failed: %s", err)
			return
		}
	}

	kinds := make([]gatekeeper.MatchKind, 0, len(deduplicated))
	for _, k := range deduplicated {
		kinds = append(kinds, k)
	}

	data, _ := yaml.Marshal(kinds)

	if len(cm.Annotations) == 0 {
		cm.Annotations = make(map[string]string, 0)
	}

	cm.Annotations[checksumAnnotation] = checksum
	cm.Data = map[string]string{"validate-kinds.yaml": string(data)}

	_, err = kt.client.CoreV1().ConfigMaps(kt.cmNamespace).Update(context.TODO(), cm, v1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Update kinds failed: %s", err)
		return
	}
	kt.latestChecksum = checksum
}

func (kt *KindTracker) FindInitialChecksum() error {
	cm, err := kt.client.CoreV1().ConfigMaps(kt.cmNamespace).Get(context.TODO(), kt.cmName, v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			klog.Info("Kinds configmap not found. Creating.")
			err = kt.createCM()
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	v, ok := cm.Annotations[checksumAnnotation]
	if !ok {
		return nil
	}

	kt.latestChecksum = v
	return nil
}

func (kt *KindTracker) createCM() error {
	cm := &corev1.ConfigMap{
		TypeMeta: v1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      kt.cmName,
			Namespace: kt.cmNamespace,
			Labels: map[string]string{
				"owner": "constraint-exporter",
			},
			Annotations: nil,
		},
		Data: nil,
	}

	_, err := kt.client.CoreV1().ConfigMaps(kt.cmNamespace).Create(context.TODO(), cm, v1.CreateOptions{})

	return err
}
