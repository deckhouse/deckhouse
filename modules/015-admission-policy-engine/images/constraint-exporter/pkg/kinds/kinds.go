package kinds

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	"github.com/flant/constraint_exporter/pkg/gatekeeper"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func deduplicateKinds(constraints []gatekeeper.Constraint) map[string]gatekeeper.MatchKind {
	// deduplicate
	m := make(map[string]gatekeeper.MatchKind)

	for _, con := range constraints {
		for _, k := range con.Spec.Match.Kinds {
			sort.Strings(k.APIGroups)
			sort.Strings(k.Kinds)
			key := fmt.Sprintf("%s:%s", strings.Join(k.APIGroups, ","), strings.Join(k.Kinds, ","))

			m[key] = k
		}
	}

	return m
}

func UpdateCM(client *kubernetes.Clientset, constraints []gatekeeper.Constraint, ns, cmName string) {
	if len(constraints) == 0 {
		return
	}

	deduplicated := deduplicateKinds(constraints)

	if len(deduplicated) == 0 {
		return
	}

	hasher := sha256.New()

	for key := range deduplicated {
		hasher.Write([]byte(key))
	}

	hashsum := fmt.Sprintf("%x", hasher.Sum(nil))

	cm, err := client.CoreV1().ConfigMaps(ns).Get(context.TODO(), cmName, v1.GetOptions{})
	if err != nil {

	}
}
