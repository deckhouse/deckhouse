package main

import (
	"context"
	"fmt"

	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createSyncConfigMap() error {
	kclient, err := k8s.NewClient()
	if err != nil {
		return fmt.Errorf("new k8s client: %w", err)
	}

	const (
		ns   = "d8-system"
		name = "docs-sync"
	)

	cm := &core.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	_, err = kclient.CoreV1().ConfigMaps(ns).Create(context.Background(), cm, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}

	return nil
}
