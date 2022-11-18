package resources

import (
	"encoding/json"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

type nodegroupChecker struct {
	kubeCl *client.KubernetesClient
	ngName string
}

func (n *nodegroupChecker) IsReady() (bool, error) {
	return false, nil
}

func (n *nodegroupChecker) Name() string {
	return fmt.Sprintf("NodeGroup %s readiness check", n.ngName)
}

func TryToGetEphemeralNodeGroupChecker(kubeCl *client.KubernetesClient, r *template.Resource) (*nodegroupChecker, error) {
	if !(r.GVK.Kind == "NodeGroup" && r.GVK.Group == "deckhouse.io" && r.GVK.Version == "v1") {
		log.Debugf("TryToGetEphemeralNodeGroupChecker: skip GVK (%s %s %s)",
			r.GVK.Version, r.GVK.Group, r.GVK.Kind)
		return nil, nil
	}

	content, err := r.Object.MarshalJSON()
	if err != nil {
		log.Errorf("Can not marshal nodegroup %s: %v", r.Object.GetName(), err)
		return nil, err
	}

	var ng NodeGroup

	err = json.Unmarshal(content, &ng)
	if err != nil {
		log.Errorf("Can not unmarshal nodegroup %s: %v", r.Object.GetName(), err)
		return nil, err
	}

	if ng.Spec.NodeType != "CloudEphemeral" {
		log.Debugf("Skip nodegroup %s, because type %s is not supported", ng.GetName(), ng.Spec.NodeType)
		return nil, nil
	}

	if ng.Spec.CloudInstances.MinPerZone == nil || ng.Spec.CloudInstances.MaxPerZone == nil {
		log.Debugf("Skip nodegroup %s, because type min and max per zone is not set", ng.GetName())
		return nil, nil
	}

	if *ng.Spec.CloudInstances.MinPerZone < 0 || *ng.Spec.CloudInstances.MaxPerZone < 1 {
		log.Debugf("Skip nodegroup %s, because type min (%d) and max (%d) per zone is incorrect",
			ng.GetName(), *ng.Spec.CloudInstances.MinPerZone, *ng.Spec.CloudInstances.MaxPerZone)
		return nil, nil
	}

	return &nodegroupChecker{
		kubeCl: kubeCl,
		ngName: ng.GetName(),
	}, nil
}
