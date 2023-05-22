/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

// TODO remove after 1.48 release

package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
)

const TerraformStateDataKey = "node-tf-state.json"
const TerraformStateNamespace = "d8-system"
const OpenstackV2ResourceType = "openstack_blockstorage_volume_v2"
const OpenstackV3ResourceType = "openstack_blockstorage_volume_v3"

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 10},
}, dependency.WithExternalDependencies(openstackTerraformStateMigration))

func openstackTerraformStateMigration(input *go_hook.HookInput, dc dependency.Container) error {
	kubeCl, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("could not initialize Kubernetes client: %v", err)
	}

	terraformStateLabels := map[string]string{
		"node.deckhouse.io/terraform-state": "",
	}

	terraformStateSecrets, err := kubeCl.CoreV1().
		Secrets(TerraformStateNamespace).
		List(context.TODO(), metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(metav1.SetAsLabelSelector(terraformStateLabels))})
	if err != nil {
		return fmt.Errorf("failed to get Terraform state secrets in namespace %s with labels %s. The migration process has been aborted", TerraformStateNamespace, terraformStateLabels)
	}

	if terraformStateSecrets.Items == nil {
		input.LogEntry.Infof("Terraform state not found. Migration is not needed.")
		return nil
	}

	for _, secret := range terraformStateSecrets.Items {
		input.LogEntry.Infof("Proceeding with Secret/%s/%s", TerraformStateNamespace, secret.ObjectMeta.Name)
		backupSecretName := secret.ObjectMeta.Name + "-backup"

		secretBackupExists, err := isSecretBackupExists(backupSecretName, TerraformStateNamespace, kubeCl, input)
		if secretBackupExists && err == nil {
			input.LogEntry.Infof("Secret backup with name %s/%s already exists! Migration is not needed for Secret/%s/%s", TerraformStateNamespace, backupSecretName, TerraformStateNamespace, secret.ObjectMeta.Name)
			continue
		}
		terraformStateRaw, ok := secret.Data[TerraformStateDataKey]
		if !ok {
			return fmt.Errorf("key %s not found in Secret/%s/%s. ", TerraformStateDataKey, TerraformStateNamespace, secret.ObjectMeta.Name)
		}

		if !gjson.ValidBytes(terraformStateRaw) {
			return fmt.Errorf("invalid JSON: %s", terraformStateRaw)
		}

		terraformState := gjson.ParseBytes(terraformStateRaw)
		openstackV2Resources := terraformState.Get("resources.#(type==\"openstack_blockstorage_volume_v2\")#")

		if len(openstackV2Resources.Array()) == 0 {
			input.LogEntry.Infof("No old resources found. Migration is not needed for Secret/%s/%s.", TerraformStateNamespace, secret.ObjectMeta.Name)
			continue
		}

		if !BackupSecret(backupSecretName, secret, TerraformStateNamespace, kubeCl, input) {
			return fmt.Errorf("can't create backup for Secret/%s/%s. The migration process has been aborted", TerraformStateNamespace, secret.ObjectMeta.Name)
		}

		terraformResources := terraformState.Get("resources")
		for i, terraformResource := range terraformResources.Array() {
			if terraformResource.Get("type").String() == OpenstackV2ResourceType {
				input.LogEntry.Infof("Found resourceType = %s with name %s. Index = %d. Modifying resource.", terraformResource.Get("type").String(), terraformResource.Get("name").String(), i)
				terraformStateRaw, err = sjson.SetBytes(terraformStateRaw, fmt.Sprintf("resources.%d.instances.0.attributes.multiattach", i), nil)
				if err != nil {
					return err
				}
				terraformStateRaw, err = sjson.SetBytes(terraformStateRaw, fmt.Sprintf("resources.%d.instances.0.attributes.enable_online_resize", i), true)
				if err != nil {
					return err
				}
				terraformStateRaw, err = sjson.SetBytes(terraformStateRaw, fmt.Sprintf("resources.%d.type", i), OpenstackV3ResourceType)
				if err != nil {
					return err
				}
			}
			for j, terraformDependency := range terraformResource.Get("instances.0.dependencies").Array() {
				if strings.Contains(terraformDependency.String(), "openstack_blockstorage_volume_v2") {
					input.LogEntry.Infof("Found dependency %s with old resource in resource = %s and type = %s. Modifying dependency.", terraformDependency.String(), terraformResource.Get("name").String(), terraformResource.Get("type").String())
					terraformStateRaw, err = sjson.SetBytes(terraformStateRaw, fmt.Sprintf("resources.%d.instances.0.dependencies.%d", i, j), strings.ReplaceAll(terraformDependency.String(), OpenstackV2ResourceType, OpenstackV3ResourceType))
					if err != nil {
						return err
					}
				}
			}
		}

		var newTerraformState bytes.Buffer
		err = json.Indent(&newTerraformState, terraformStateRaw, "", "  ")
		if err != nil {
			return err
		}

		secret.Data[TerraformStateDataKey] = newTerraformState.Bytes()
		_, err = kubeCl.CoreV1().
			Secrets(TerraformStateNamespace).
			Update(context.TODO(), &secret, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		input.LogEntry.Infof("Migration process completed for Secret/%s/%s.", TerraformStateNamespace, secret.ObjectMeta.Name)
	}

	input.LogEntry.Infof("Terraform migrate hook has been finished.")
	return nil
}

func isSecretBackupExists(backupSecretName string, namespace string, kubeCl k8s.Client, input *go_hook.HookInput) (bool, error) {
	input.LogEntry.Debugf("Function isSecretBackupExists: Starting function with parameters: backupSecretName=%s; namespace=%s", backupSecretName, namespace)
	backupSecret, err := kubeCl.CoreV1().
		Secrets(namespace).
		Get(context.TODO(), backupSecretName, metav1.GetOptions{})
	input.LogEntry.Debugf("Function isSecretBackupExists: Get secret. Secret=%s; err=%s", backupSecret, err)

	if errors.IsNotFound(err) {
		input.LogEntry.Debugf("Function isSecretBackupExists: errors.IsNotFound(err) = %t. secret \"%s\" not found'. Return false and nil", errors.IsNotFound(err), backupSecretName)
		return false, nil
	}

	if err == nil {
		input.LogEntry.Debugf("Function isSecretBackupExists: err == nil. Return true and nil")
		return true, nil
	}

	input.LogEntry.Debugf("Function isSecretBackupExists: err !=nil and errors.IsNotFound(err) no true. Return false and err")
	return false, err
}

func BackupSecret(backupSecretName string, secret v1.Secret, namespace string, kubeCl k8s.Client, input *go_hook.HookInput) bool {
	nodeName := secret.ObjectMeta.Labels["node.deckhouse.io/node-name"]
	nodeGroup := secret.ObjectMeta.Labels["node.deckhouse.io/node-group"]
	// nodeName := strings.TrimPrefix(secret.ObjectMeta.Name, "d8-node-terraform-state-")

	input.LogEntry.Infof("Starting backup for Secret/%s/%s", namespace, secret.ObjectMeta.Name)
	secretBackup := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupSecretName,
			Namespace: namespace,
			Labels: map[string]string{
				"heritage":                     "deckhouse",
				"name":                         backupSecretName,
				"node.deckhouse.io/node-group": nodeGroup,
				"node.deckhouse.io/node-name":  nodeName,
				"node.deckhouse.io/terraform-state-backup": "",
			},
		},
		Data: secret.Data,
	}

	_, err := kubeCl.CoreV1().
		Secrets(namespace).
		Create(context.TODO(), secretBackup, metav1.CreateOptions{})

	if err != nil {
		input.LogEntry.Warnf("An error occurred when creating secret backup. Backup aborted. Error: %s.", err)
		return false
	}
	input.LogEntry.Infof("Secret backup completed successfully.")
	return true
}
