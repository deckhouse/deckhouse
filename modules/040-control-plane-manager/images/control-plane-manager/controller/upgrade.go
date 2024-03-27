package main

import (
	"context"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"golang.org/x/mod/semver"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const clusterAdminsGroupAndClusterRoleBinding = "kubeadm:cluster-admins"

func upgradeToK8s129() error {
	config, _ := rest.InClusterConfig()
	kubeClient, _ := kubernetes.NewForConfig(config)

	_, err := kubeClient.RbacV1().ClusterRoleBindings().Get(context.TODO(), clusterAdminsGroupAndClusterRoleBinding, metav1.GetOptions{})

	if err != nil && strings.Contains(err.Error(), "not found") {
		log.Print("Create ClusterRoleBinding \"kubeadm:cluster-admins\"")
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
			return fmt.Errorf("Create ClusterRoleBinding \"kubeadm:cluster-admins\" error: %v", err)
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
