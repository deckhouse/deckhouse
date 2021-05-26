package hooks

import (
	"fmt"
	"regexp"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/flant/shell-operator/pkg/metric_storage/operation"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

func getDeploymentImage(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	deployment := &appsv1.Deployment{}
	err := sdk.FromUnstructured(obj, deployment)
	if err != nil {
		return nil, fmt.Errorf("cannot convert deckhouse deployment to deployment: %v", err)
	}

	return deployment.Spec.Template.Spec.Containers[0].Image, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "deckhouse",
			ApiVersion: "apps/v1",
			Kind:       "Deployment",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"deckhouse"},
			},
			FilterFunc: getDeploymentImage,
		},
	},
}, parseDeckhouseImage)

func parseDeckhouseImage(input *go_hook.HookInput) error {
	const (
		deckhouseImagePath = "deckhouse.internal.currentReleaseImageName"
		repoPath           = "global.modulesImages.registry"
	)

	deckhouseSnapshot := input.Snapshots["deckhouse"]
	if len(deckhouseSnapshot) != 1 {
		return fmt.Errorf("deckhouse was not able to find an image of itself")
	}

	image := deckhouseSnapshot[0].(string)
	input.Values.Set(deckhouseImagePath, image)

	repo := input.Values.Get(repoPath).String()
	repoPattern := fmt.Sprintf("^%s[:,/](.*)$", regexp.QuoteMeta(repo))
	repoRegex, err := regexp.Compile(repoPattern)
	if err != nil {
		return fmt.Errorf("cannot complie regex %q", repoPattern)
	}

	captureGroups := repoRegex.FindStringSubmatch(image)

	metricResult := float64(1)
	if len(captureGroups) == 2 {
		switch captureGroups[1] {
		case "alpha", "beta", "early-access", "stable", "rock-solid":
			metricResult = 0
		}
	}

	*input.Metrics = append(*input.Metrics, operation.MetricOperation{
		Name:   "d8_deckhouse_is_not_on_release_channel",
		Action: "set",
		Value:  pointer.Float64Ptr(metricResult),
	})
	return nil
}
