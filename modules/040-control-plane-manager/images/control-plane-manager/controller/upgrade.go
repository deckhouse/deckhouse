/*
Copyright 2024 Flant JSC

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
	"fmt"
	log "github.com/sirupsen/logrus"
	"golang.org/x/mod/semver"
	rbac "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const clusterAdminsGroupAndClusterRoleBinding = "kubeadm:cluster-admins"

func upgradeToK8s129() error {
	fmt.Println("I'm fine")
	config, _ := rest.InClusterConfig()
	kubeClient, _ := kubernetes.NewForConfig(config)

	_, err := kubeClient.RbacV1().ClusterRoleBindings().Get(context.TODO(), clusterAdminsGroupAndClusterRoleBinding, metav1.GetOptions{})

	if apierrors.IsNotFound(err) {
		log.Print("create ClusterRoleBinding \"kubeadm:cluster-admins\"")
		clusterRoleBinding := &rbac.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterAdminsGroupAndClusterRoleBinding,
			},
			RoleRef: rbac.RoleRef{
				APIGroup: rbac.GroupName,
				Kind:     "ClusterRole",
				Name:     "cluster-admin",
			},
			Subjects: []rbac.Subject{
				{
					Kind: rbac.GroupKind,
					Name: clusterAdminsGroupAndClusterRoleBinding,
				},
			},
		}

		_, err = kubeClient.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterRoleBinding, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("create ClusterRoleBinding \"kubeadm:cluster-admins\" error: %v", err)
		}
	}

	return nil
}

func upgrade() error {
	if semver.Compare("v1.29.0", fmt.Sprintf("v%s", config.KubernetesVersion)) == 0 {
		return upgradeToK8s129()
	}

	return nil
}
