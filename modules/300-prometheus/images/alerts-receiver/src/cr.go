/*
Copyright 2023 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"

	"github.com/prometheus/common/model"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	clusterAlertFiring       = "firing"
	clusterAlertFiringStaled = "firing (stale)"
)

type ClusterAlert struct {
	v1.TypeMeta   `json:",inline"`
	v1.ObjectMeta `json:"metadata,omitempty"`

	Alert  ClusterAlertSpec   `json:"alert,omitempty"`
	Status ClusterAlertStatus `json:"status,omitempty"`
}

type ClusterAlertStatus struct {
	AlertStatus    string `json:"alertStatus,omitempty"`
	StartsAt       string `json:"startsAt,omitempty"`
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
}

type ClusterAlertSpec struct {
	Name          string         `json:"name"`
	SeverityLevel string         `json:"severityLevel,omitempty"`
	Summary       string         `json:"summary,omitempty"`
	Description   string         `json:"description,omitempty"`
	Annotations   model.LabelSet `json:"annotations,omitempty"`
	Labels        model.LabelSet `json:"labels"`
}

func listCRs() (map[string]struct{}, error) {
	log.Info("list CRs")
	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	crList, err := config.k8sClient.Resource(GVR).List(ctx, v1.ListOptions{
		LabelSelector:        "app=" + appName + ",heritage=deckhouse",
		ResourceVersionMatch: v1.ResourceVersionMatchNotOlderThan,
		ResourceVersion:      "0",
	})
	cancel()
	if err != nil {
		return nil, err
	}
	res := make(map[string]struct{}, len(crList.Items))
	for _, item := range crList.Items {
		res[item.GetName()] = struct{}{}
	}
	log.Infof("found %d CRs in cluster", len(crList.Items))
	return res, nil
}

// Remove CR from cluster
func removeCR(fingerprint string) error {
	log.Infof("remove CR with name %s from cluster", fingerprint)
	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	err := config.k8sClient.Resource(GVR).Delete(ctx, fingerprint, v1.DeleteOptions{})
	cancel()
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}
