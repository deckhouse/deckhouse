package helm

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"github.com/romana/rlog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kblabels "k8s.io/apimachinery/pkg/labels"

	"github.com/deckhouse/deckhouse/antiopa/executor"
	"github.com/deckhouse/deckhouse/antiopa/kube"
)

type HelmClient interface {
	TillerNamespace() string
	CommandEnv() []string
	Cmd(args ...string) (string, string, error)
	DeleteSingleFailedRevision(releaseName string) error
	LastReleaseStatus(releaseName string) (string, string, error)
	UpgradeRelease(releaseName string, chart string, valuesPaths []string, namespace string) error
	DeleteRelease(releaseName string) error
	ListReleases() ([]string, error)
	ListReleasesNames() ([]string, error)
	IsReleaseExists(releaseName string) (bool, error)
}

type CliHelm struct {
	tillerNamespace string
}

// InitHelm запускает установку tiller-a.
func Init(tillerNamespace string) (HelmClient, error) {
	rlog.Info("Helm: run helm init")

	helm := &CliHelm{tillerNamespace: tillerNamespace}

	err := helm.InitTiller()
	if err != nil {
		return nil, err
	}

	stdout, stderr, err := helm.Cmd("version")
	if err != nil {
		return nil, fmt.Errorf("unable to get helm version: %v\n%v %v", err, stdout, stderr)
	}
	rlog.Infof("Helm: helm version:\n%v %v", stdout, stderr)

	rlog.Info("Helm: successfully initialized")

	return helm, nil
}

func (helm *CliHelm) InitTiller() error {
	antiopaDeploy, err := kube.KubernetesClient.AppsV1beta1().Deployments(kube.KubernetesAntiopaNamespace).Get(kube.AntiopaDeploymentName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("cannot fetch antiopa deployment to gather settings for tiller deployment: %s", err)
	}

	cmd := make([]string, 0)
	cmd = append(cmd,
		"init",
		"--service-account", "antiopa",
		"--upgrade", "--wait", "--skip-refresh",
	)

	nodeSelectors := make([]string, 0)
	for k, v := range antiopaDeploy.Spec.Template.Spec.NodeSelector {
		nodeSelectors = append(nodeSelectors, fmt.Sprintf("%s=%s", k, v))
	}
	if len(nodeSelectors) > 0 {
		cmd = append(cmd, fmt.Sprintf("--node-selectors=%s", strings.Join(nodeSelectors, ",")))
	}

	override := make([]string, 0)
	for i, spec := range antiopaDeploy.Spec.Template.Spec.Tolerations {
		override = append(override, fmt.Sprintf("spec.template.spec.tolerations[%d].key=%s", i, spec.Key))
		override = append(override, fmt.Sprintf("spec.template.spec.tolerations[%d].operator=%s", i, spec.Operator))
		override = append(override, fmt.Sprintf("spec.template.spec.tolerations[%d].value=%s", i, spec.Value))
		override = append(override, fmt.Sprintf("spec.template.spec.tolerations[%d].effect=%s", i, spec.Effect))

		if spec.TolerationSeconds != nil {
			override = append(override, fmt.Sprintf("spec.template.spec.tolerations[%d].tolerationSeconds=%s", i, *spec.TolerationSeconds))
		}
	}
	if len(override) > 0 {
		cmd = append(cmd, fmt.Sprintf("--override=%s", strings.Join(override, ",")))
	}

	stdout, stderr, err := helm.Cmd(cmd...)
	if err != nil {
		return fmt.Errorf("%s\n%s\n%s", err, stdout, stderr)
	}
	rlog.Infof("Helm: tiller initialization done: %v %v", stdout, stderr)

	return nil
}

func (helm *CliHelm) TillerNamespace() string {
	return helm.tillerNamespace
}

func (helm *CliHelm) CommandEnv() []string {
	res := make([]string, 0)
	res = append(res, fmt.Sprintf("TILLER_NAMESPACE=%s", helm.TillerNamespace()))
	return res
}

// Запускает helm с переданными аргументами.
// Перед запуском устанавливает переменную среды TILLER_NAMESPACE,
// чтобы antiopa работала со своим tiller-ом.
func (helm *CliHelm) Cmd(args ...string) (stdout string, stderr string, err error) {
	binPath := "/usr/local/bin/helm"
	cmd := exec.Command(binPath, args...)
	cmd.Env = append(os.Environ(), helm.CommandEnv()...)

	var stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	err = executor.Run(cmd)
	stdout = strings.TrimSpace(stdoutBuf.String())
	stderr = strings.TrimSpace(stderrBuf.String())

	return
}

func (helm *CliHelm) DeleteSingleFailedRevision(releaseName string) (err error) {
	revision, status, err := helm.LastReleaseStatus(releaseName)
	if err != nil {
		if revision == "0" {
			// revision 0 is not an error. just skip deletion.
			rlog.Infof("helm: release '%s' not found. No cleanup required.", releaseName)
			return nil
		}
		rlog.Debugf("helm.LastReleaseStatus of '%s' return error: %s", releaseName, err)
		return err
	}

	//  No interest of revisions older than 1
	if revision == "1" && status == "FAILED" {
		// delete and purge!
		err = helm.DeleteRelease(releaseName)
		if err != nil {
			rlog.Infof("Error deleting first failed release '%s': %v", releaseName, err)
			return err
		}
		rlog.Infof("  Single failed release for '%s' deleted", releaseName)
	} else {
		rlog.Debugf("Release '%s' has revision '%s' with status %s", releaseName, revision, status)
	}

	return
}

// Get last known revision and status
// helm history output:
// REVISION	UPDATED                 	STATUS    	CHART                 	DESCRIPTION
// 1        Fri Jul 14 18:25:00 2017	SUPERSEDED	symfony-demo-0.1.0    	Install complete
func (helm *CliHelm) LastReleaseStatus(releaseName string) (revision string, status string, err error) {
	stdout, stderr, err := helm.Cmd("history", releaseName, "--max", "1")

	if err != nil {
		errLine := strings.Split(stderr, "\n")[0]
		if strings.Contains(errLine, "Error:") && strings.Contains(errLine, "not found") {
			// Bad module name or no releases installed
			err = fmt.Errorf("No release '%s' found\n%v %v", releaseName, stdout, stderr)
			revision = "0"
			return
		}

		err = fmt.Errorf("Cannot get history for release '%s'\n%v %v", releaseName, stdout, stderr)
		return
	}

	historyLines := strings.Split(stdout, "\n")
	lastLine := historyLines[len(historyLines)-1]
	fields := regexp.MustCompile("\\t").Split(lastLine, 5)
	revision = strings.TrimSpace(fields[0])
	status = strings.TrimSpace(fields[2])
	return
}

func (helm *CliHelm) UpgradeRelease(releaseName string, chart string, valuesPaths []string, namespace string) error {
	args := make([]string, 0)
	args = append(args, "upgrade")
	args = append(args, "--install")
	args = append(args, releaseName)
	args = append(args, chart)

	if namespace != "" {
		args = append(args, "--namespace")
		args = append(args, namespace)
	}

	for _, valuesPath := range valuesPaths {
		args = append(args, "--values")
		args = append(args, valuesPath)
	}

	rlog.Infof("Running helm upgrade for release '%s' with chart '%s' in namespace '%s' ...", releaseName, chart, namespace)
	stdout, stderr, err := helm.Cmd(args...)
	if err != nil {
		return fmt.Errorf("helm upgrade failed: %s:\n%s %s", err, stdout, stderr)
	}
	rlog.Infof("Helm upgrade for release '%s' with chart '%s' in namespace '%s' successful:\n%s\n%s", releaseName, chart, namespace, stdout, stderr)

	return nil
}

func (helm *CliHelm) DeleteRelease(releaseName string) (err error) {
	rlog.Debugf("Running helm delete --purge for '%s' release", releaseName)

	stdout, stderr, err := helm.Cmd("delete", "--purge", releaseName)
	if err != nil {
		return fmt.Errorf("helm delete --purge %s invocation error: %v\n%v %v", releaseName, err, stdout, stderr)
	}

	return
}

func (helm *CliHelm) IsReleaseExists(releaseName string) (bool, error) {
	revision, _, err := helm.LastReleaseStatus(releaseName)
	if err != nil && revision == "0" {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}

// Возвращает все известные релизы в виде строк "<имя_релиза>.v<номер_версии>"
// helm ищет ConfigMap-ы по лейблу OWNER=TILLER и получает данные о релизе из ключа "release"
// https://github.com/kubernetes/helm/blob/8981575082ea6fc2a670f81fb6ca5b560c4f36a7/pkg/storage/driver/cfgmaps.go#L88
func (helm *CliHelm) ListReleases() ([]string, error) {
	lsel := kblabels.Set{"OWNER": "TILLER"}.AsSelector()
	cmList, err := kube.KubernetesClient.CoreV1().
		ConfigMaps(kube.KubernetesAntiopaNamespace).
		List(metav1.ListOptions{LabelSelector: lsel.String()})
	if err != nil {
		rlog.Debugf("helm releases ConfigMaps list failed: %s", err)
		return nil, err
	}

	releases := make([]string, 0)
	for _, cm := range cmList.Items {
		if _, has_key := cm.Data["release"]; has_key {
			releases = append(releases, cm.Name)
		}
	}

	sort.Strings(releases)

	return releases, nil
}

// Список имён релизов без суффикса ".v<номер релиза>"
func (helm *CliHelm) ListReleasesNames() ([]string, error) {
	releases, err := helm.ListReleases()
	if err != nil {
		return []string{}, err
	}

	var releaseCmNamePattern = regexp.MustCompile(`^(.*).v[0-9]+$`)

	releasesNamesMap := map[string]bool{}
	for _, release := range releases {
		matchRes := releaseCmNamePattern.FindStringSubmatch(release)
		if matchRes != nil {
			releaseName := matchRes[1]
			releasesNamesMap[releaseName] = true
		}
	}

	releasesNames := make([]string, 0)
	for releaseName, _ := range releasesNamesMap {
		releasesNames = append(releasesNames, releaseName)
	}

	return releasesNames, nil
}
