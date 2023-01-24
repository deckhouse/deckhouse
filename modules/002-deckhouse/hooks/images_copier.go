// Copyright 2021 Flant JSC
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

package hooks

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/google/go-containerregistry/pkg/authn"
	batchv1 "k8s.io/api/batch/v1"
	v1core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

const (
	copierNs             = "d8-system"
	copierConfSecretName = "images-copier-config"
	copierJobName        = "copy-images"

	copierConfImagesKey       = "d8-images.json"
	copierConfD8Repo          = "d8-repo.json"
	copierConfDestinationRepo = "dest-repo.json"
)

func copierLabels() map[string]string {
	return map[string]string{
		"heritage": "deckhouse",
		"app":      "d8-images-copier",
	}
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/deckhouse/images_copier",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "copier_secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{copierConfSecretName},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{copierNs},
				},
			},
			FilterFunc: applyImageCopierFilter,
		},

		{
			Name:       "copier_job_pods",
			ApiVersion: "v1",
			Kind:       "Pod",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: copierLabels(),
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{copierNs},
				},
			},
			FilterFunc: applyJobPodsFilter,
		},

		{
			Name:       "copier_job",
			ApiVersion: "batch/v1",
			Kind:       "Job",
			NameSelector: &types.NameSelector{
				MatchNames: []string{copierJobName},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{copierNs},
				},
			},
			FilterFunc: applyCopierJobFilter,
		},
	},
}, imageCopierHandler)

type registry struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Insecure bool   `json:"insecure"`
	Image    string `json:"image"`
}

type copierConf struct {
	DestRepo      registry
	DeckhouseRepo *registry

	DeckhouseInternalImages map[string]interface{}

	Annotations map[string]string
}

type dockerFileConfig struct {
	Auths map[string]authn.AuthConfig `json:"auths"`
}

type jobStatus struct {
	Running bool
	Success bool
	Fail    bool

	Annotations map[string]string
}

func (s *jobStatus) isFinished() bool {
	return s.Success || s.Fail
}

func applyJobPodsFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

func applyImageCopierFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cm v1core.Secret
	err := sdk.FromUnstructured(obj, &cm)
	if err != nil {
		return "", err
	}

	destRegistry := registry{}
	err = json.Unmarshal(cm.Data[copierConfDestinationRepo], &destRegistry)
	if err != nil {
		return nil, fmt.Errorf("cannot parse destenation repo config: %v", err)
	}

	var d8Registry *registry
	if repoStr, ok := cm.Data[copierConfD8Repo]; ok {
		err = json.Unmarshal(repoStr, &d8Registry)
		if err != nil {
			return nil, fmt.Errorf("cannot parse deckhouse repo config: %v", err)
		}
	}

	d8Images := make(map[string]interface{})

	if imagesStr, ok := cm.Data[copierConfImagesKey]; ok {
		err := json.Unmarshal(imagesStr, &d8Images)
		if err != nil {
			return nil, fmt.Errorf("cannot parse d8 images repo config: %v", err)
		}
	}

	return copierConf{
		DestRepo:                destRegistry,
		DeckhouseRepo:           d8Registry,
		DeckhouseInternalImages: d8Images,

		Annotations: cm.GetAnnotations(),
	}, nil
}

func applyCopierJobFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var job batchv1.Job
	err := sdk.FromUnstructured(obj, &job)
	if err != nil {
		return "", err
	}

	return jobStatus{
		Fail:    job.Status.Failed > 0,
		Success: job.Status.Succeeded > 0,
		Running: job.Status.Active > 0,

		Annotations: job.GetAnnotations(),
	}, nil
}

func createJob(input *go_hook.HookInput, annotations map[string]string) {
	const confDir = "/config"
	const secretVolumeMountName = "config"

	repo := input.Values.Get("global.modulesImages.registry.base").String()
	copierTag := input.Values.Get("global.modulesImages.tags.deckhouse.imagesCopier").String()

	podSpec := v1core.PodSpec{
		ImagePullSecrets: []v1core.LocalObjectReference{
			{
				Name: "deckhouse-registry",
			},
		},
		Containers: []v1core.Container{
			{
				Name:            "image-copier",
				ImagePullPolicy: v1core.PullAlways,
				Image:           fmt.Sprintf("%s:%s", repo, copierTag),
				Command: []string{
					"copy-images.sh",
					"--d8-repo-conf-file", fmt.Sprintf("%s/%s", confDir, copierConfD8Repo),
					"--dest-repo-conf-file", fmt.Sprintf("%s/%s", confDir, copierConfDestinationRepo),
					"--images-conf-file", fmt.Sprintf("%s/%s", confDir, copierConfImagesKey),
				},
				VolumeMounts: []v1core.VolumeMount{
					{
						Name:      secretVolumeMountName,
						ReadOnly:  true,
						MountPath: confDir,
					},
				},
			},
		},
		Volumes: []v1core.Volume{
			{
				Name: secretVolumeMountName,
				VolumeSource: v1core.VolumeSource{
					Secret: &v1core.SecretVolumeSource{
						SecretName:  copierConfSecretName,
						DefaultMode: pointer.Int32Ptr(0400),
					},
				},
			},
		},
		RestartPolicy: v1core.RestartPolicyNever,
	}

	job := &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: "batch/v1",
		},

		Spec: batchv1.JobSpec{
			Template: v1core.PodTemplateSpec{
				Spec: podSpec,
				ObjectMeta: metav1.ObjectMeta{
					Labels:      copierLabels(),
					Annotations: annotations,
					Namespace:   copierNs,
				},
			},
			BackoffLimit: pointer.Int32Ptr(0),
		},

		ObjectMeta: metav1.ObjectMeta{
			Name:        copierJobName,
			Labels:      copierLabels(),
			Annotations: annotations,
			Namespace:   copierNs,
		},
	}

	input.PatchCollector.Create(job)
}

func addD8ConfigToSecret(input *go_hook.HookInput) error {
	d8Registry, err := parseD8RegistryCredentials(input)
	if err != nil {
		return err
	}
	d8RegistryJSON, err := json.Marshal(d8Registry)
	if err != nil {
		return err
	}

	imagesJSON := input.Values.Get("global.modulesImages.tags").Raw

	apply := func(u *unstructured.Unstructured) (*unstructured.Unstructured, error) {
		var conf v1core.Secret
		err := sdk.FromUnstructured(u, &conf)
		if err != nil {
			return nil, err
		}

		conf.Data[copierConfImagesKey] = []byte(imagesJSON)
		conf.Data[copierConfD8Repo] = d8RegistryJSON

		return sdk.ToUnstructured(&conf)
	}

	input.PatchCollector.Filter(apply, "v1", "Secret", copierNs, copierConfSecretName)

	return nil
}

func parseD8RegistryCredentials(input *go_hook.HookInput) (*registry, error) {
	image := input.Values.Get("deckhouse.internal.currentReleaseImageName").String()
	registryHost := input.Values.Get("global.modulesImages.registry.address").String()
	dockerConfigEncoded := input.Values.Get("global.modulesImages.registry.dockercfg").String()
	dockerConfig, err := base64.StdEncoding.DecodeString(dockerConfigEncoded)
	if err != nil {
		return nil, err
	}

	var auth dockerFileConfig
	err = json.Unmarshal(dockerConfig, &auth)
	if err != nil {
		return nil, fmt.Errorf("cannot decode docker config JSON: %v", err)
	}
	creds, ok := auth.Auths[registryHost]
	if !ok {
		return nil, fmt.Errorf("no credentials for current registry")
	}

	var username string
	var password string

	if creds.Username != "" && creds.Password != "" {
		username = creds.Username
		password = creds.Password
	} else if creds.Auth != "" {
		auth, err := base64.StdEncoding.DecodeString(creds.Auth)
		if err != nil {
			return nil, fmt.Errorf(`cannot decode base64 "auth" field`)
		}
		parts := strings.Split(string(auth), ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf(`unexpected format of "auth" field`)
		}
		username = parts[0]
		password = parts[1]
	}

	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)

	if username == "" || password == "" {
		return nil, fmt.Errorf("not found creds in dockerconfig")
	}
	return &registry{
		Username: username,
		Password: password,
		Image:    image,
		Insecure: false,
	}, nil
}

func cleanupJob(input *go_hook.HookInput) {
	// begin remove pods
	for _, snap := range input.Snapshots["copier_job_pods"] {
		podName := snap.(string)
		input.PatchCollector.Delete("v1", "Pod", copierNs, podName)
	}

	input.PatchCollector.Delete("batch/v1", "Job", copierNs, copierJobName)
}

// imageCopierHandler
// run image copier job if copier secret was added
// cleanup secret and job if image copier ran successfully
// restart job if image copier ran fail and secret annotations changed
// cleanup if secret was deleted and has failed job
func imageCopierHandler(input *go_hook.HookInput) error {
	var job *jobStatus
	jobSnap := input.Snapshots["copier_job"]
	if len(jobSnap) > 0 {
		s := jobSnap[0].(jobStatus)
		job = &s
	}

	copierSnap := input.Snapshots["copier_secret"]
	if len(copierSnap) < 1 {
		if job != nil {
			input.LogEntry.Info("Image copier secret not found, but copier job found. Remove job")
			cleanupJob(input)
		}
		return nil
	}

	conf := copierSnap[0].(copierConf)
	if len(conf.DeckhouseInternalImages) == 0 || conf.DeckhouseRepo == nil {
		// secret was added,
		// add d8 registry configuration with modules
		// and create job
		err := addD8ConfigToSecret(input)
		if err != nil {
			return err
		}
	}

	// if job is not created but we have all configuration
	// crete job and exit
	if job == nil {
		createJob(input, conf.Annotations)
		return nil
	}

	// job is running do nothing
	if job.Running {
		return nil
	}

	if job.Success {
		input.LogEntry.Info("Image copier ran successfully. Cleanup")
		// begin remove secret
		input.PatchCollector.Delete("v1", "Secret", copierNs, copierConfSecretName)
		cleanupJob(input)
		return nil
	}

	if job.Fail {
		if reflect.DeepEqual(conf.Annotations, job.Annotations) {
			input.LogEntry.Error("Image copier was failed. See logs into image copier job pod for additional information")
			return nil
		}

		// if annotations were changed - restart job
		cleanupJob(input)
		createJob(input, conf.Annotations)
		return nil
	}

	return nil
}
