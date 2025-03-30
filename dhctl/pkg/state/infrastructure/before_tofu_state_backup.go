// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package infrastructure

import (
	"context"
	"fmt"
	"strings"
	"time"

	apiv1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

const (
	tofuBackupPrefix          = "tf-bkp"
	tofuBackupNodeStatePrefix = tofuBackupPrefix + "-node"
	baseInfraBackupSecretName = tofuBackupPrefix + "-cluster-state"
)

func addTofuBackupLabelAndAnnotation(secret *apiv1.Secret) {
	if len(secret.Labels) == 0 {
		secret.Labels = make(map[string]string, 1)
	}

	secret.Labels["dhctl.deckhouse.io/before-tofu-state-backup"] = "true"

	if len(secret.Annotations) == 0 {
		secret.Annotations = make(map[string]string, 1)
	}

	secret.Annotations["dhctl.deckhouse.io/before-tofu-state-backup-time"] = time.Now().Format(time.RFC3339)
}

func nodeStateSecretNameToBackupName(secret *apiv1.Secret) string {
	suffix := strings.TrimPrefix(secret.Name, "d8-node-terraform-")
	return tofuBackupNodeStatePrefix + "-" + suffix
}

type TofuMigrationStateBackuper struct {
	kubeProvider kubernetes.KubeClientProvider
	logger       log.Logger
}

func NewTofuMigrationStateBackuper(kubeCl kubernetes.KubeClientProvider, logger log.Logger) *TofuMigrationStateBackuper {
	return &TofuMigrationStateBackuper{
		kubeProvider: kubeCl,
		logger:       logger,
	}
}

func (t *TofuMigrationStateBackuper) doBackupStates(ctx context.Context) error {
	exists, err := t.isBackupSecretExist(ctx, baseInfraBackupSecretName)
	if err != nil {
		return err
	}

	if !exists {
		baseSecret, err := t.getBaseInfraSecret(ctx)
		if err != nil {
			return err
		}

		err = t.saveBackupSecret(ctx, "base", baseSecret, baseInfraBackupSecretName)
		if err != nil {
			return err
		}
	} else {
		t.logger.LogInfoF("Backup secret %s for base infrastructure state exists. Skip backup.", baseInfraBackupSecretName)
	}

	secrets, err := GetNodesStateSecretsFromCluster(ctx, t.kubeProvider.KubeClient())
	if err != nil {
		return err
	}

	for _, secret := range secrets {
		nodeBackupSecretName := nodeStateSecretNameToBackupName(secret)
		exists, err := t.isBackupSecretExist(ctx, nodeBackupSecretName)
		if err != nil {
			return err
		}

		if exists {
			t.logger.LogInfoF("Backup secret %s for base infrastructure state exists. Skip backup.", nodeBackupSecretName)
			continue
		}

		err = t.saveBackupSecret(ctx, nodeBackupSecretName, secret, nodeBackupSecretName)
		if err != nil {
			return err
		}

	}

	return nil
}

func (t *TofuMigrationStateBackuper) BackupStates(ctx context.Context) error {
	return t.logger.LogProcess("default", "Backup infrastructure states before migrate to opentofu", func() error {
		return t.doBackupStates(ctx)
	})
}

func (t *TofuMigrationStateBackuper) getBaseInfraSecret(ctx context.Context) (*apiv1.Secret, error) {
	var secret *apiv1.Secret
	err := retry.NewLoop("Get base infrastructure backup state exists", 15, 5*time.Second).
		WithLogger(t.logger).
		BreakIf(k8serrors.IsNotFound).
		RunContext(ctx, func() error {
			var err error
			secret, err = t.kubeProvider.KubeClient().CoreV1().Secrets(global.D8SystemNamespace).Get(ctx, manifests.InfrastructureClusterStateName, metav1.GetOptions{})
			if err != nil {
				return err
			}

			return nil
		})

	if err != nil {
		return nil, err
	}

	return secret, nil
}

func (t *TofuMigrationStateBackuper) isBackupSecretExist(ctx context.Context, name string) (bool, error) {
	var backupSecretExists bool
	err := retry.NewLoop("Check base infrastructure backup state exists", 15, 5*time.Second).WithLogger(t.logger).
		RunContext(ctx, func() error {
			_, err := t.kubeProvider.KubeClient().CoreV1().Secrets(global.D8SystemNamespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				if k8serrors.IsNotFound(err) {
					backupSecretExists = false
					return nil
				}
			}

			backupSecretExists = true
			return nil
		})

	if err != nil {
		return false, err
	}

	return backupSecretExists, nil
}

func (t *TofuMigrationStateBackuper) saveBackupSecret(ctx context.Context, processPrefix string, secret *apiv1.Secret, newName string) error {
	bkpSecret := secret.DeepCopy()
	bkpSecret.Name = newName
	addTofuBackupLabelAndAnnotation(secret)

	return retry.NewLoop(fmt.Sprintf("Save %s infrastructure backup state", processPrefix), 15, 5*time.Second).
		WithLogger(t.logger).
		RunContext(ctx, func() error {
			var err error
			_, err = t.kubeProvider.KubeClient().CoreV1().Secrets(global.D8SystemNamespace).Create(ctx, bkpSecret, metav1.CreateOptions{})
			if err != nil {
				return err
			}

			return nil
		})
}
