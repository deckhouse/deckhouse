package nodetemplate

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

func (r *Reconciler) reconcileNode(ctx context.Context, node *corev1.Node, ng *v1.NodeGroup) (bool, error) {
	base := node.DeepCopy()
	working := node.DeepCopy()

	isClusterAPINode := hasKey(working.Annotations, clusterAPIAnnotationKey)

	if ng.Spec.NodeType == v1.NodeTypeCloudEphemeral {
		fixCloudNodeTaints(working, ng)
		if isClusterAPINode {
			if err := applyNodeTemplate(working, ng); err != nil {
				return false, err
			}
		}
	} else {
		if err := applyNodeTemplate(working, ng); err != nil {
			return false, err
		}
	}

	if ng.Name == "master" {
		if working.Labels == nil {
			working.Labels = make(map[string]string)
		}
		working.Labels[controlPlaneTaintKey] = ""
		working.Labels[masterNodeRoleKey] = ""
		if len(working.Spec.Taints) > 0 {
			working.Spec.Taints = fixMasterTaints(working.Spec.Taints, getTemplateTaints(ng))
		}
	}

	if shouldDisableScaleDown(ng.Spec.NodeType) {
		if working.Annotations == nil {
			working.Annotations = make(map[string]string)
		}
		working.Annotations["cluster-autoscaler.kubernetes.io/scale-down-disabled"] = "true"
	}

	if !nodeChanged(base, working) {
		return false, nil
	}

	working.Status = base.Status
	if err := r.Client.Patch(ctx, working, client.MergeFrom(base)); err != nil {
		return false, err
	}

	return true, nil
}
