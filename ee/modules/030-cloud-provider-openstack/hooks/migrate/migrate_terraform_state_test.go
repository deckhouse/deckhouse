/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

// TODO remove after 1.48 release

package hooks

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func createTerraformStateSecret(secretName string, terraformStateDataKey string, terraformState string, nodeGroup string, secretType string) error {
	var secretTemplate = `
---
apiVersion: v1
data:
  %s: %s
  %s: %s
kind: Secret
metadata:
  labels:
    heritage: deckhouse
    name: %s
    node.deckhouse.io/node-group: %s
    %s: ""
  name: %s
type: Opaque
`
	// terraformState := fmt.Sprintf(terraformState, nodeName)

	secretYaml := fmt.Sprintf(secretTemplate,
		terraformStateDataKey, base64.StdEncoding.EncodeToString([]byte(terraformState)),
		TestKey, base64.StdEncoding.EncodeToString([]byte(TestData)),
		secretName, nodeGroup, secretType, secretName)

	var secret v1.Secret
	err := yaml.Unmarshal([]byte(secretYaml), &secret)
	if err != nil {
		return err
	}
	_, err = dependency.TestDC.MustGetK8sClient().
		CoreV1().
		Secrets(TerraformStateNamespace).
		Create(context.TODO(), &secret, metav1.CreateOptions{})
	return err
}

var _ = Describe("Global :: migrate_terraform_state ::", func() {
	const (
		initValuesString       = `{}`
		initConfigValuesString = `{}`
	)

	Context("Secrets with terraform state does not exist in "+TerraformStateNamespace, func() {
		f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook should not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook should generate proper log messages", func() {
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Terraform state not found. Migration is not needed."))
		})

	})

	Context("Single master with root size: Secret with terraform state exists in "+TerraformStateNamespace+" namespace, field "+TerraformNodeStateDataKey+" contains old data", func() {
		f := HookExecutionConfigInit(initValuesString, initConfigValuesString)
		nodeName := "master-0"

		BeforeEach(func() {
			f.KubeStateSet("")
			err := createTerraformStateSecret("d8-node-terraform-state-"+nodeName, TerraformNodeStateDataKey, oldTerraformStateWithRootDiskSize, "master", "node.deckhouse.io/terraform-state")
			Expect(err).To(BeNil())
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook should not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook should make backup", func() {
			secret, err := dependency.TestDC.K8sClient.CoreV1().
				Secrets(TerraformStateNamespace).
				Get(context.TODO(), "d8-node-terraform-state-"+nodeName+"-backup", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(secret.Data).NotTo(BeNil())
			Expect(secret.Data[TerraformNodeStateDataKey]).To(MatchJSON(oldTerraformStateWithRootDiskSize))
			Expect(secret.Data[TestKey]).To(BeEquivalentTo(TestData))
		})

		It("Hook should migrate Terraform state", func() {
			secret, err := dependency.TestDC.K8sClient.CoreV1().
				Secrets(TerraformStateNamespace).
				Get(context.TODO(), "d8-node-terraform-state-"+nodeName, metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(secret.Data).NotTo(BeNil())
			Expect(secret.Data[TerraformNodeStateDataKey]).To(MatchJSON(newTerraformStateWithRootDiskSize))
			Expect(secret.Data[TestKey]).To(BeEquivalentTo(TestData))

			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Found resourceType = %s with name kubernetes_data.", OpenstackV2ResourceType))
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Found resourceType = %s with name master.", OpenstackV2ResourceType))
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Found dependency"))
		})
	})

	Context("Single master with root size: Migration has been done already", func() {
		f := HookExecutionConfigInit(initValuesString, initConfigValuesString)
		nodeName := "master-0"

		BeforeEach(func() {
			f.KubeStateSet("")
			err := createTerraformStateSecret("d8-node-terraform-state-"+nodeName, TerraformNodeStateDataKey, newTerraformStateWithRootDiskSize, "master", "node.deckhouse.io/terraform-state")
			Expect(err).To(BeNil())
			err = createTerraformStateSecret("d8-node-terraform-state-"+nodeName+"-backup", TerraformNodeStateDataKey, oldTerraformStateWithRootDiskSize, "master", "node.deckhouse.io/terraform-state-backup")
			Expect(err).To(BeNil())
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook should not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook should not change existing state", func() {
			secret, err := dependency.TestDC.K8sClient.CoreV1().
				Secrets(TerraformStateNamespace).
				Get(context.TODO(), "d8-node-terraform-state-"+nodeName, metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(secret.Data).NotTo(BeNil())
			Expect(secret.Data[TerraformNodeStateDataKey]).To(MatchJSON(newTerraformStateWithRootDiskSize))
			Expect(secret.Data[TestKey]).To(BeEquivalentTo(TestData))
		})

		It("Hook should generate proper log messages", func() {
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Secret backup with name %s/%s already exists! Migration is not needed for Secret/%s/%s", TerraformStateNamespace, "d8-node-terraform-state-"+nodeName+"-backup", TerraformStateNamespace, "d8-node-terraform-state-"+nodeName))
		})
	})

	Context("Single master with root size: Secret with terraform state exists in "+TerraformStateNamespace+" namespace, field "+TerraformNodeStateDataKey+" contains new data, no migration was done", func() {
		f := HookExecutionConfigInit(initValuesString, initConfigValuesString)
		nodeName := "master-0"

		BeforeEach(func() {
			f.KubeStateSet("")
			err := createTerraformStateSecret("d8-node-terraform-state-"+nodeName, TerraformNodeStateDataKey, newTerraformStateWithRootDiskSize, "master", "node.deckhouse.io/terraform-state")
			Expect(err).To(BeNil())
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook should not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook should not change existing state", func() {
			secret, err := dependency.TestDC.K8sClient.CoreV1().
				Secrets(TerraformStateNamespace).
				Get(context.TODO(), "d8-node-terraform-state-"+nodeName, metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(secret.Data).NotTo(BeNil())
			Expect(secret.Data[TerraformNodeStateDataKey]).To(MatchJSON(newTerraformStateWithRootDiskSize))
			Expect(secret.Data[TestKey]).To(BeEquivalentTo(TestData))
		})

		It("Hook should not create backup", func() {
			_, err := dependency.TestDC.K8sClient.CoreV1().
				Secrets(TerraformStateNamespace).
				Get(context.TODO(), "d8-node-terraform-state-"+nodeName+"-backup", metav1.GetOptions{})
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		It("Hook should generate proper log messages", func() {
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("No old resources found. Migration is not needed for Secret/%s/%s", TerraformStateNamespace, "d8-node-terraform-state-"+nodeName))
		})
	})

	Context("Single master with long node name", func() {
		nodeName := "s-aa-bbbbbbbbbbb-ddddd-ee-master-0"

		Context("Single master with root size: Secret with terraform state exists in "+TerraformStateNamespace+" namespace, field "+TerraformNodeStateDataKey+" contains old data", func() {

			f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

			BeforeEach(func() {
				f.KubeStateSet("")
				err := createTerraformStateSecret("d8-node-terraform-state-"+nodeName, TerraformNodeStateDataKey, oldTerraformStateWithRootDiskSize, "master", "node.deckhouse.io/terraform-state")
				Expect(err).To(BeNil())
				f.BindingContexts.Set(f.GenerateOnStartupContext())
				f.RunHook()
			})

			It("Hook should not fail", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("Hook should make backup with short name", func() {
				secret, err := dependency.TestDC.K8sClient.CoreV1().
					Secrets(TerraformStateNamespace).
					Get(context.TODO(), "d8-node-state-"+nodeName+"-backup", metav1.GetOptions{})
				Expect(err).To(BeNil())
				Expect(secret.Data).NotTo(BeNil())
				Expect(secret.Data[TerraformNodeStateDataKey]).To(MatchJSON(oldTerraformStateWithRootDiskSize))
				Expect(secret.Data[TestKey]).To(BeEquivalentTo(TestData))
			})

			It("Hook should migrate Terraform state", func() {
				secret, err := dependency.TestDC.K8sClient.CoreV1().
					Secrets(TerraformStateNamespace).
					Get(context.TODO(), "d8-node-terraform-state-"+nodeName, metav1.GetOptions{})
				Expect(err).To(BeNil())
				Expect(secret.Data).NotTo(BeNil())
				Expect(secret.Data[TerraformNodeStateDataKey]).To(MatchJSON(newTerraformStateWithRootDiskSize))
				Expect(secret.Data[TestKey]).To(BeEquivalentTo(TestData))

				Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Found resourceType = %s with name kubernetes_data.", OpenstackV2ResourceType))
				Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Found resourceType = %s with name master.", OpenstackV2ResourceType))
				Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Found dependency"))
			})
		})

		Context("Single master with root size: Migration has been done already", func() {
			f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

			BeforeEach(func() {
				f.KubeStateSet("")
				err := createTerraformStateSecret("d8-node-terraform-state-"+nodeName, TerraformNodeStateDataKey, newTerraformStateWithRootDiskSize, "master", "node.deckhouse.io/terraform-state")
				Expect(err).To(BeNil())
				err = createTerraformStateSecret("d8-node-state-"+nodeName+"-backup", TerraformNodeStateDataKey, oldTerraformStateWithRootDiskSize, "master", "node.deckhouse.io/terraform-state-backup")
				Expect(err).To(BeNil())
				f.BindingContexts.Set(f.GenerateOnStartupContext())
				f.RunHook()
			})

			It("Hook should not fail", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("Hook should not change existing state", func() {
				secret, err := dependency.TestDC.K8sClient.CoreV1().
					Secrets(TerraformStateNamespace).
					Get(context.TODO(), "d8-node-terraform-state-"+nodeName, metav1.GetOptions{})
				Expect(err).To(BeNil())
				Expect(secret.Data).NotTo(BeNil())
				Expect(secret.Data[TerraformNodeStateDataKey]).To(MatchJSON(newTerraformStateWithRootDiskSize))
				Expect(secret.Data[TestKey]).To(BeEquivalentTo(TestData))
			})

			It("Hook should generate proper log messages", func() {
				Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Secret backup with name %s/%s already exists! Migration is not needed for Secret/%s/%s", TerraformStateNamespace, "d8-node-state-"+nodeName+"-backup", TerraformStateNamespace, "d8-node-terraform-state-"+nodeName))
			})
		})
	})

	Context("Multi master and other CloudPermanent nodes with root size: Secret with terraform state exists in "+TerraformStateNamespace+" namespace, field "+TerraformNodeStateDataKey+" contains old data", func() {
		f := HookExecutionConfigInit(initValuesString, initConfigValuesString)
		nodeNames := []string{"master-0", "master-1", "master-2", "test-0", "front-0", "front-1"}
		terraformStateBackupLabels := map[string]string{
			"node.deckhouse.io/terraform-state-backup": "",
		}

		BeforeEach(func() {
			f.KubeStateSet("")
			for _, nodeName := range nodeNames {
				err := createTerraformStateSecret("d8-node-terraform-state-"+nodeName, TerraformNodeStateDataKey, oldTerraformStateWithRootDiskSize, strings.Split(nodeName, "-")[0], "node.deckhouse.io/terraform-state")
				Expect(err).To(BeNil())
			}

			err := createTerraformStateSecret("d8-cluster-terraform-state", TerraformClusterStateDataKey, oldTerraformStateWithRootDiskSize, "d8-cluster-terraform-state", "test")
			Expect(err).To(BeNil())

			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook should not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook should make backup", func() {
			for _, nodeName := range nodeNames {
				secret, err := dependency.TestDC.K8sClient.CoreV1().
					Secrets(TerraformStateNamespace).
					Get(context.TODO(), "d8-node-terraform-state-"+nodeName+"-backup", metav1.GetOptions{})
				Expect(err).To(BeNil())
				Expect(secret.Data).NotTo(BeNil())
				Expect(secret.Data[TerraformNodeStateDataKey]).To(MatchJSON(oldTerraformStateWithRootDiskSize))
				Expect(secret.Data[TestKey]).To(BeEquivalentTo(TestData))
				Expect(secret.ObjectMeta.Labels["node.deckhouse.io/node-group"]).To(BeEquivalentTo(strings.Split(nodeName, "-")[0]))
			}

			secret, err := dependency.TestDC.K8sClient.CoreV1().
				Secrets(TerraformStateNamespace).
				Get(context.TODO(), "d8-cluster-terraform-state-backup", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(secret.Data).NotTo(BeNil())
			Expect(secret.Data[TerraformClusterStateDataKey]).To(MatchJSON(oldTerraformStateWithRootDiskSize))
			Expect(secret.Data[TestKey]).To(BeEquivalentTo(TestData))

			terraformStateBackupSecrets, err := dependency.TestDC.K8sClient.CoreV1().
				Secrets(TerraformStateNamespace).
				List(context.TODO(), metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(metav1.SetAsLabelSelector(terraformStateBackupLabels))})
			Expect(err).To(BeNil())
			Expect(len(terraformStateBackupSecrets.Items)).To(BeEquivalentTo(7))
		})

		It("Hook should migrate Terraform state", func() {
			for _, nodeName := range nodeNames {
				secret, err := dependency.TestDC.K8sClient.CoreV1().
					Secrets(TerraformStateNamespace).
					Get(context.TODO(), "d8-node-terraform-state-"+nodeName, metav1.GetOptions{})
				Expect(err).To(BeNil())
				Expect(secret.Data).NotTo(BeNil())
				Expect(secret.Data[TerraformNodeStateDataKey]).To(MatchJSON(newTerraformStateWithRootDiskSize))
				Expect(secret.Data[TestKey]).To(BeEquivalentTo(TestData))
				Expect(secret.ObjectMeta.Labels["node.deckhouse.io/node-group"]).To(BeEquivalentTo(strings.Split(nodeName, "-")[0]))
			}

			secret, err := dependency.TestDC.K8sClient.CoreV1().
				Secrets(TerraformStateNamespace).
				Get(context.TODO(), "d8-cluster-terraform-state", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(secret.Data).NotTo(BeNil())
			Expect(secret.Data[TerraformClusterStateDataKey]).To(MatchJSON(newTerraformStateWithRootDiskSize))
			Expect(secret.Data[TestKey]).To(BeEquivalentTo(TestData))

			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Found resourceType = %s with name kubernetes_data.", OpenstackV2ResourceType))
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Found resourceType = %s with name master.", OpenstackV2ResourceType))
		})
	})

	Context("Multi-master master with long node name", func() {
		Context("Multi master and other CloudPermanent nodes with root size with long size: Secret with terraform state exists in "+TerraformStateNamespace+" namespace, field "+TerraformNodeStateDataKey+" contains old data", func() {
			f := HookExecutionConfigInit(initValuesString, initConfigValuesString)
			nodeNames := []string{"s-aa-bbbbbbbbbbb-ddddd-ee-master-0", "s-aa-bbbbbbbbbbb-ddddd-ee-master-1", "s-aa-bbbbbbbbbbb-ddddd-ee-master-2", "s-aa-bbbbbbbbbbb-ddddd-ee-testes-0"}
			terraformStateBackupLabels := map[string]string{
				"node.deckhouse.io/terraform-state-backup": "",
			}

			BeforeEach(func() {
				f.KubeStateSet("")
				for _, nodeName := range nodeNames {
					err := createTerraformStateSecret("d8-node-terraform-state-"+nodeName, TerraformNodeStateDataKey, oldTerraformStateWithRootDiskSize, strings.Split(nodeName, "-")[0], "node.deckhouse.io/terraform-state")
					Expect(err).To(BeNil())
				}

				err := createTerraformStateSecret("d8-cluster-terraform-state", TerraformClusterStateDataKey, oldTerraformStateWithRootDiskSize, "d8-cluster-terraform-state", "test")
				Expect(err).To(BeNil())

				f.BindingContexts.Set(f.GenerateOnStartupContext())
				f.RunHook()
			})

			It("Hook should not fail", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("Hook should make backup", func() {
				for _, nodeName := range nodeNames {
					secret, err := dependency.TestDC.K8sClient.CoreV1().
						Secrets(TerraformStateNamespace).
						Get(context.TODO(), "d8-node-state-"+nodeName+"-backup", metav1.GetOptions{})
					Expect(err).To(BeNil())
					Expect(secret.Data).NotTo(BeNil())
					Expect(secret.Data[TerraformNodeStateDataKey]).To(MatchJSON(oldTerraformStateWithRootDiskSize))
					Expect(secret.Data[TestKey]).To(BeEquivalentTo(TestData))
					Expect(secret.ObjectMeta.Labels["node.deckhouse.io/node-group"]).To(BeEquivalentTo(strings.Split(nodeName, "-")[0]))
				}

				secret, err := dependency.TestDC.K8sClient.CoreV1().
					Secrets(TerraformStateNamespace).
					Get(context.TODO(), "d8-cluster-terraform-state-backup", metav1.GetOptions{})
				Expect(err).To(BeNil())
				Expect(secret.Data).NotTo(BeNil())
				Expect(secret.Data[TerraformClusterStateDataKey]).To(MatchJSON(oldTerraformStateWithRootDiskSize))
				Expect(secret.Data[TestKey]).To(BeEquivalentTo(TestData))

				terraformStateBackupSecrets, err := dependency.TestDC.K8sClient.CoreV1().
					Secrets(TerraformStateNamespace).
					List(context.TODO(), metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(metav1.SetAsLabelSelector(terraformStateBackupLabels))})
				Expect(err).To(BeNil())
				Expect(len(terraformStateBackupSecrets.Items)).To(BeEquivalentTo(5))
			})

			It("Hook should not change existing state", func() {
				for _, nodeName := range nodeNames {
					secret, err := dependency.TestDC.K8sClient.CoreV1().
						Secrets(TerraformStateNamespace).
						Get(context.TODO(), "d8-node-terraform-state-"+nodeName, metav1.GetOptions{})
					Expect(err).To(BeNil())
					Expect(secret.Data).NotTo(BeNil())
					Expect(secret.Data[TerraformNodeStateDataKey]).To(MatchJSON(newTerraformStateWithRootDiskSize))
					Expect(secret.Data[TestKey]).To(BeEquivalentTo(TestData))
					Expect(secret.ObjectMeta.Labels["node.deckhouse.io/node-group"]).To(BeEquivalentTo(strings.Split(nodeName, "-")[0]))
				}

				secret, err := dependency.TestDC.K8sClient.CoreV1().
					Secrets(TerraformStateNamespace).
					Get(context.TODO(), "d8-cluster-terraform-state", metav1.GetOptions{})
				Expect(err).To(BeNil())
				Expect(secret.Data).NotTo(BeNil())
				Expect(secret.Data[TerraformClusterStateDataKey]).To(MatchJSON(newTerraformStateWithRootDiskSize))
				Expect(secret.Data[TestKey]).To(BeEquivalentTo(TestData))

				Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Found resourceType = %s with name kubernetes_data.", OpenstackV2ResourceType))
				Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Found resourceType = %s with name master.", OpenstackV2ResourceType))
			})
		})

		Context("Multi master and other CloudPermanent nodes with root size with long size: Secret with terraform state exists in "+TerraformStateNamespace+" namespace, field "+TerraformNodeStateDataKey+" contains old data and backups is exists", func() {
			f := HookExecutionConfigInit(initValuesString, initConfigValuesString)
			nodeNames := []string{"s-aa-bbbbbbbbbbb-ddddd-ee-master-0", "s-aa-bbbbbbbbbbb-ddddd-ee-master-1", "s-aa-bbbbbbbbbbb-ddddd-ee-master-2", "s-aa-bbbbbbbbbbb-ddddd-ee-testes-0"}

			BeforeEach(func() {
				f.KubeStateSet("")
				for _, nodeName := range nodeNames {
					err := createTerraformStateSecret("d8-node-terraform-state-"+nodeName, TerraformNodeStateDataKey, newTerraformStateWithRootDiskSize, strings.Split(nodeName, "-")[0], "node.deckhouse.io/terraform-state")
					Expect(err).To(BeNil())
					err = createTerraformStateSecret("d8-node-state-"+nodeName+"-backup", TerraformNodeStateDataKey, oldTerraformStateWithRootDiskSize, strings.Split(nodeName, "-")[0], "node.deckhouse.io/terraform-state-backup")
					Expect(err).To(BeNil())
				}

				err := createTerraformStateSecret("d8-cluster-terraform-state", TerraformClusterStateDataKey, newTerraformStateWithRootDiskSize, "d8-cluster-terraform-state", "test")
				Expect(err).To(BeNil())

				err = createTerraformStateSecret("d8-cluster-terraform-state-backup", TerraformClusterStateDataKey, oldTerraformStateWithRootDiskSize, "d8-cluster-terraform-state", "test")
				Expect(err).To(BeNil())

				f.BindingContexts.Set(f.GenerateOnStartupContext())
				f.RunHook()
			})

			It("Hook should not fail", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("Hook should not change existing state", func() {
				for _, nodeName := range nodeNames {
					secret, err := dependency.TestDC.K8sClient.CoreV1().
						Secrets(TerraformStateNamespace).
						Get(context.TODO(), "d8-node-terraform-state-"+nodeName, metav1.GetOptions{})
					Expect(err).To(BeNil())
					Expect(secret.Data).NotTo(BeNil())
					Expect(secret.Data[TerraformNodeStateDataKey]).To(MatchJSON(newTerraformStateWithRootDiskSize))
					Expect(secret.Data[TestKey]).To(BeEquivalentTo(TestData))
				}

				secret, err := dependency.TestDC.K8sClient.CoreV1().
					Secrets(TerraformStateNamespace).
					Get(context.TODO(), "d8-cluster-terraform-state", metav1.GetOptions{})
				Expect(err).To(BeNil())
				Expect(secret.Data).NotTo(BeNil())
				Expect(secret.Data[TerraformClusterStateDataKey]).To(MatchJSON(newTerraformStateWithRootDiskSize))
				Expect(secret.Data[TestKey]).To(BeEquivalentTo(TestData))
			})

			It("Hook should generate proper log messages", func() {
				for _, nodeName := range nodeNames {
					Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Secret backup with name %s/%s already exists! Migration is not needed for Secret/%s/%s", TerraformStateNamespace, "d8-node-state-"+nodeName+"-backup", TerraformStateNamespace, "d8-node-terraform-state-"+nodeName))
				}

				Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Secret backup with name %s/%s already exists! Migration is not needed for Secret/%s/%s", TerraformStateNamespace, "d8-cluster-terraform-state-backup", TerraformStateNamespace, "d8-cluster-terraform-state"))
			})
		})
	})
})

const (
	TestKey                           = "test-key"
	TestData                          = "my test data"
	oldTerraformStateWithRootDiskSize = `
{
  "version": 4,
  "terraform_version": "0.13.4",
  "resources": [
    {
      "module": "module.kubernetes_data",
      "type": "openstack_blockstorage_volume_v2",
      "name": "kubernetes_data",
      "provider": "provider[\"registry.terraform.io/terraform-provider-openstack/openstack\"]",
      "instances": [
        {
          "schema_version": 0,
          "attributes": {
            "volume_type": "ceph-ssd"
          },
          "private": "eyJlMmJmYjczMC1lY2FhLTExZTYtOGY4OC0zNDM2M2JjN2M0YzAiOnsiY3JlYXRlIjo2MDAwMDAwMDAwMDAsImRlbGV0ZSI6NjAwMDAwMDAwMDAwfX0=",
          "dependencies": [
            "data.openstack_compute_availability_zones_v2.zones",
            "module.volume_zone.data.openstack_blockstorage_availability_zones_v3.zones"
          ]
        }
      ]
    },
    {
      "module": "module.kubernetes_data",
      "type": "openstack_compute_volume_attach_v2",
      "name": "kubernetes_data",
      "provider": "provider[\"registry.terraform.io/terraform-provider-openstack/openstack\"]",
      "instances": [
        {
          "schema_version": 0,
          "attributes": {
            "device": "/dev/vdb",
            "id": "b66cc0e3-0839-4b21-8e93-a45540ac36d0/58d980ff-3126-48c8-b193-0493c92055db"
          },
          "private": "eyJlMmJmYjczMC1lY2FhLTExZTYtOGY4OC0zNDM2M2JjN2M0YzAiOnsiY3JlYXRlIjo2MDAwMDAwMDAwMDAsImRlbGV0ZSI6NjAwMDAwMDAwMDAwfX0=",
          "dependencies": [
            "module.kubernetes_data.openstack_blockstorage_volume_v2.kubernetes_data",
            "module.master.openstack_blockstorage_volume_v2.master",
            "module.master.openstack_compute_instance_v2.master"
          ]
        }
      ]
    },
    {
      "module": "module.master",
      "type": "openstack_blockstorage_volume_v2",
      "name": "master",
      "provider": "provider[\"registry.terraform.io/terraform-provider-openstack/openstack\"]",
      "instances": [
        {
          "schema_version": 0,
          "attributes": {
            "volume_type": "ceph-ssd"
          },
          "private": "eyJlMmJmYjczMC1lY2FhLTExZTYtOGY4OC0zNDM2M2JjN2M0YzAiOnsiY3JlYXRlIjo2MDAwMDAwMDAwMDAsImRlbGV0ZSI6NjAwMDAwMDAwMDAwfX0=",
          "dependencies": [
            "data.openstack_compute_availability_zones_v2.zones",
            "module.master.data.openstack_images_image_v2.master",
            "module.volume_zone.data.openstack_blockstorage_availability_zones_v3.zones"
          ]
        }
      ]
    },
    {
      "module": "module.master",
      "type": "openstack_compute_floatingip_associate_v2",
      "name": "master",
      "provider": "provider[\"registry.terraform.io/terraform-provider-openstack/openstack\"]",
      "instances": [
        {
          "index_key": 0,
          "schema_version": 0,
          "attributes": {
            "fixed_ip": "",
            "floating_ip": "95.217.68.251",
            "id": "95.217.68.251/b66cc0e3-0839-4b21-8e93-a45540ac36d0/",
            "instance_id": "b66cc0e3-0839-4b21-8e93-a45540ac36d0",
            "region": "HetznerFinland",
            "timeouts": null,
            "wait_until_associated": true
          },
          "private": "eyJlMmJmYjczMC1lY2FhLTExZTYtOGY4OC0zNDM2M2JjN2M0YzAiOnsiY3JlYXRlIjo2MDAwMDAwMDAwMDB9fQ==",
          "dependencies": [
            "module.master.data.openstack_images_image_v2.master",
            "module.master.openstack_blockstorage_volume_v2.master",
            "module.master.openstack_compute_floatingip_v2.master"
          ]
        }
      ]
    },
    {
      "module": "module.master",
      "type": "openstack_compute_instance_v2",
      "name": "master",
      "provider": "provider[\"registry.terraform.io/terraform-provider-openstack/openstack\"]",
      "instances": [
        {
          "schema_version": 0,
          "attributes": {
            "admin_pass": null,
            "block_device": [
              {
                "boot_index": 0,
                "delete_on_termination": true,
                "destination_type": "volume"
              }
            ],
            "config_drive": false
          },
          "private": "eyJlMmJmYjczMC1lY2FhLTExZTYtOGY4OC0zNDM2M2JjN2M0YzAiOnsiY3JlYXRlIjoxODAwMDAwMDAwMDAwLCJkZWxldGUiOjE4MDAwMDAwMDAwMDAsInVwZGF0ZSI6MTgwMDAwMDAwMDAwMH19",
          "dependencies": [
            "module.master.openstack_blockstorage_volume_v2.master",
            "openstack_networking_port_v2.master_internal_without_security"
          ]
        }
      ]
    }
  ]
}

`
	newTerraformStateWithRootDiskSize = `
{
  "version": 4,
  "terraform_version": "0.13.4",
  "resources": [
    {
      "module": "module.kubernetes_data",
      "type": "openstack_blockstorage_volume_v3",
      "name": "kubernetes_data",
      "provider": "provider[\"registry.terraform.io/terraform-provider-openstack/openstack\"]",
      "instances": [
        {
          "schema_version": 0,
          "attributes": {
            "enable_online_resize": true,
            "multiattach": null,
            "volume_type": "ceph-ssd"
          },
          "private": "eyJlMmJmYjczMC1lY2FhLTExZTYtOGY4OC0zNDM2M2JjN2M0YzAiOnsiY3JlYXRlIjo2MDAwMDAwMDAwMDAsImRlbGV0ZSI6NjAwMDAwMDAwMDAwfX0=",
          "dependencies": [
            "data.openstack_compute_availability_zones_v2.zones",
            "module.volume_zone.data.openstack_blockstorage_availability_zones_v3.zones"
          ]
        }
      ]
    },
    {
      "module": "module.kubernetes_data",
      "type": "openstack_compute_volume_attach_v2",
      "name": "kubernetes_data",
      "provider": "provider[\"registry.terraform.io/terraform-provider-openstack/openstack\"]",
      "instances": [
        {
          "schema_version": 0,
          "attributes": {
            "device": "/dev/vdb",
            "id": "b66cc0e3-0839-4b21-8e93-a45540ac36d0/58d980ff-3126-48c8-b193-0493c92055db"
          },
          "private": "eyJlMmJmYjczMC1lY2FhLTExZTYtOGY4OC0zNDM2M2JjN2M0YzAiOnsiY3JlYXRlIjo2MDAwMDAwMDAwMDAsImRlbGV0ZSI6NjAwMDAwMDAwMDAwfX0=",
          "dependencies": [
            "module.kubernetes_data.openstack_blockstorage_volume_v3.kubernetes_data",
            "module.master.openstack_blockstorage_volume_v3.master",
            "module.master.openstack_compute_instance_v2.master"
          ]
        }
      ]
    },
    {
      "module": "module.master",
      "type": "openstack_blockstorage_volume_v3",
      "name": "master",
      "provider": "provider[\"registry.terraform.io/terraform-provider-openstack/openstack\"]",
      "instances": [
        {
          "schema_version": 0,
          "attributes": {
            "enable_online_resize": true,
            "multiattach": null,
            "volume_type": "ceph-ssd"
          },
          "private": "eyJlMmJmYjczMC1lY2FhLTExZTYtOGY4OC0zNDM2M2JjN2M0YzAiOnsiY3JlYXRlIjo2MDAwMDAwMDAwMDAsImRlbGV0ZSI6NjAwMDAwMDAwMDAwfX0=",
          "dependencies": [
            "data.openstack_compute_availability_zones_v2.zones",
            "module.master.data.openstack_images_image_v2.master",
            "module.volume_zone.data.openstack_blockstorage_availability_zones_v3.zones"
          ]
        }
      ]
    },
    {
      "module": "module.master",
      "type": "openstack_compute_floatingip_associate_v2",
      "name": "master",
      "provider": "provider[\"registry.terraform.io/terraform-provider-openstack/openstack\"]",
      "instances": [
        {
          "index_key": 0,
          "schema_version": 0,
          "attributes": {
            "fixed_ip": "",
            "floating_ip": "95.217.68.251",
            "id": "95.217.68.251/b66cc0e3-0839-4b21-8e93-a45540ac36d0/",
            "instance_id": "b66cc0e3-0839-4b21-8e93-a45540ac36d0",
            "region": "HetznerFinland",
            "timeouts": null,
            "wait_until_associated": true
          },
          "private": "eyJlMmJmYjczMC1lY2FhLTExZTYtOGY4OC0zNDM2M2JjN2M0YzAiOnsiY3JlYXRlIjo2MDAwMDAwMDAwMDB9fQ==",
          "dependencies": [
            "module.master.data.openstack_images_image_v2.master",
            "module.master.openstack_blockstorage_volume_v3.master",
            "module.master.openstack_compute_floatingip_v2.master"
          ]
        }
      ]
    },
    {
      "module": "module.master",
      "type": "openstack_compute_instance_v2",
      "name": "master",
      "provider": "provider[\"registry.terraform.io/terraform-provider-openstack/openstack\"]",
      "instances": [
        {
          "schema_version": 0,
          "attributes": {
            "admin_pass": null,
            "block_device": [
              {
                "boot_index": 0,
                "delete_on_termination": true,
                "destination_type": "volume"
              }
            ],
            "config_drive": false
          },
          "private": "eyJlMmJmYjczMC1lY2FhLTExZTYtOGY4OC0zNDM2M2JjN2M0YzAiOnsiY3JlYXRlIjoxODAwMDAwMDAwMDAwLCJkZWxldGUiOjE4MDAwMDAwMDAwMDAsInVwZGF0ZSI6MTgwMDAwMDAwMDAwMH19",
          "dependencies": [
            "module.master.openstack_blockstorage_volume_v3.master",
            "openstack_networking_port_v2.master_internal_without_security"
          ]
        }
      ]
    }
  ]
}
`
)
