package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/romana/rlog"
	v1 "k8s.io/api/core/v1"
	rbacapi "k8s.io/api/rbac/v1beta1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InitHelm запускает установку tiller-a.
func InitHelm() {
	svcList, err := KubernetesClient.CoreV1().Services(HelmTillerNamespace()).List(meta_v1.ListOptions{})
	if err != nil {
		rlog.Errorf("HELM-INIT: %s", err)
		os.Exit(1)
	}

	helmInitialized := false
	for _, item := range svcList.Items {
		if item.Name == "tiller-deploy" {
			helmInitialized = true
			break
		}
	}

	if !helmInitialized {
		rlog.Infof("HELM-INIT Initializing tiller in namespace %s", HelmTillerNamespace())

		_, err := KubernetesClient.CoreV1().Namespaces().Get(HelmTillerNamespace(), meta_v1.GetOptions{})
		if err != nil && errors.IsNotFound(err) {
			ns := v1.Namespace{}
			ns.Name = HelmTillerNamespace()

			_, err = KubernetesClient.CoreV1().Namespaces().Create(&ns)
			if err != nil {
				rlog.Errorf("HELM-INIT: %s", err)
				os.Exit(1)
			}
		} else if err != nil {
			rlog.Errorf("HELM-INIT: %s", err)
			os.Exit(1)
		}

		// Взято из https://github.com/kubernetes/helm/blob/master/docs/service_accounts.md#example-service-account-with-cluster-admin-role

		serviceAccount := v1.ServiceAccount{}
		serviceAccount.Name = "tiller"

		_, err = KubernetesClient.CoreV1().ServiceAccounts(HelmTillerNamespace()).Create(&serviceAccount)
		if err != nil && !errors.IsAlreadyExists(err) {
			rlog.Errorf("HELM-INIT Unable to create tiller ServiceAccount: %s", err)
			os.Exit(1)
		}

		clusterRoleBinding := rbacapi.ClusterRoleBinding{}
		clusterRoleBinding.Name = fmt.Sprintf("%s-tiller", HelmTillerNamespace())
		clusterRoleBinding.Labels = make(map[string]string)
		clusterRoleBinding.Labels["antiopa-namespace"] = HelmTillerNamespace()
		clusterRoleBinding.RoleRef.APIGroup = "rbac.authorization.k8s.io"
		clusterRoleBinding.RoleRef.Kind = "ClusterRole"
		clusterRoleBinding.RoleRef.Name = "cluster-admin"
		clusterRoleBinding.Subjects = []rbacapi.Subject{
			rbacapi.Subject{Kind: "ServiceAccount", Name: "tiller", Namespace: HelmTillerNamespace()},
		}

		_, err = KubernetesClient.RbacV1beta1().ClusterRoleBindings().Create(&clusterRoleBinding)
		if err != nil && errors.IsAlreadyExists(err) {
			_, err = KubernetesClient.RbacV1beta1().ClusterRoleBindings().Update(&clusterRoleBinding)
			if err != nil {
				rlog.Errorf("HELM-INIT Unable to update ClusterRoleBinding %s: %s", clusterRoleBinding.Name, err)
				os.Exit(1)
			}
		} else if err != nil {
			rlog.Errorf("HELM-INIT Unable to create ClusterRoleBinding %s: %s", clusterRoleBinding.Name, err)
			os.Exit(1)
		}

		stdout, stderr, err := HelmCmd("init", "--service-account", "tiller")
		if err != nil {
			rlog.Errorf("HELM-INIT: %s", err)
			os.Exit(1)
		}
		rlog.Infof("HELM-INIT Tiller initialization done: %v %v", stdout, stderr)
	}

	// Ожидаем в течении 2х минут готовности helm
	helmReady := false
	for i := 0; i < 120; i++ {
		stdout, stderr, err := HelmCmd("ls")
		if err != nil {
			if stderr == "Error: could not find a ready tiller pod" {
				time.Sleep(1)
			} else {
				rlog.Errorf("HELM-INIT: Helm not ready: %s\n%s %s", err, stdout, stderr)
				os.Exit(1)
			}
		} else {
			helmReady = true
			break
		}
	}
	if !helmReady {
		rlog.Errorf("HELM-INIT: Helm readiness timeout: could not find a ready tiller pod")
		os.Exit(1)
	}

	stdout, stderr, err := HelmCmd("version")
	if err != nil {
		rlog.Errorf("HELM-INIT Unable to get helm version: %v\n%v %v", err, stdout, stderr)
		os.Exit(1)
	}
	rlog.Infof("HELM-INIT helm version:\n%v %v", stdout, stderr)

	rlog.Info("HELM-INIT Successfully initialized")
}

// HelmTillerNamespace возвращает имя namespace, куда устаналивается tiller
// Можно ставить в другой namespace, можно в тот же, где сама antiopa.
// TODO Есть переменная TILLER_NAMESPACE - можно её поставить ещё на этапе деплоя
func HelmTillerNamespace() string {
	return KubernetesAntiopaNamespace
	//return fmt.Sprintf("%s-tiller", KubernetesAntiopaNamespace)
}

// HelmCmd запускает helm с переданными аргументами
// Перед запуском устанавливает переменную среды TILLER_NAMESPACE
// чтобы antiopa работала со своим tiller-ом
func HelmCmd(args ...string) (stdout string, stderr string, err error) {
	cmd := exec.Command("/usr/local/bin/helm", args...)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("TILLER_NAMESPACE=%s", HelmTillerNamespace()),
	)
	var stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	err = cmd.Run()
	stdout = strings.TrimSpace(stdoutBuf.String())
	stderr = strings.TrimSpace(stderrBuf.String())

	return
}
