package docker_registry_manager

import (
	//"fmt"
	//"github.com/deckhouse/deckhouse/antiopa/kube"
	//"github.com/romana/rlog"
	//"github.com/stretchr/testify/assert"
	//"os"
	//"os/exec"
	//"strings"
	"testing"
)

func TestRegistryManager_SetOrCheckAntiopaImageId(t *testing.T) {
	t.SkipNow()
	// TODO DockerRegistryManager refactored
	//ImageUpdated = make(chan string)
	//
	//AntiopaImageDigest = "sha256:6e5ba1192843bda054090a1f7a8481054a0b1038457b3acb9043628e0443ed50"
	//imageId := "sha256:8386dcece0405dba12f18321103ad8a8d31bd0ea80279c8e7fbdaab0bc104066"
	//
	//go SetOrCheckAntiopaImageDigest(imageId, nil)
	//
	//res := <-ImageUpdated
	//assert.True(t, res != "")
	//fmt.Printf("digest: %s\n", res)
}

// This test requires cluster and registry
func TestRegistryManager_CheckImageId(t *testing.T) {
	t.SkipNow()
	// TODO DockerRegistryManager refactored
	//os.Setenv("RLOG_LOG_LEVEL", "DEBUG")
	//rlog.UpdateEnv()
	//
	//// get pod name from kubectl invocation
	////"kubectl -n antiopa get po -o json | jq '.items[].metadata.name | if test(\"antiopa\") then . else empty end' -r"
	//args := []string{"-c", "kubectl -n antiopa get po -o json | jq '.items[].metadata.name | if test(\"antiopa\") then . else empty end' -r"}
	//out, err := exec.Command("bash", args...).Output()
	//assert.NoError(t, err, "kubectl get pod name invocation problem")
	//hostname := strings.TrimSpace(string(out))
	//assert.True(t, hostname != "")
	//rlog.Infof("antiopa pod name is: '%s'", hostname)
	//
	//args = []string{"-c", "kubectl -n antiopa get po -o json | jq '.items[] | select(.metadata.name | test(\"antiopa\")) | .status.containerStatuses[].imageID | split(\"sha256:\") | \"sha256:\"+.[1]' -r"}
	//out, err = exec.Command("bash", args...).Output()
	//assert.NoError(t, err, "kubectl get image id invocation problem")
	//imageId := strings.TrimSpace(string(out))
	//assert.True(t, imageId != "")
	//rlog.Infof("antiopa pod imageID: '%s'", imageId)
	//
	//kube.InitKube()
	//InitRegistryManager(hostname)
	//
	//// first step — get id from kube, parse image name.
	//rlog.Debugf("Registry Manager Test: STEP 1")
	//CheckIsImageUpdated()
	//assert.Equal(t, AntiopaImageDigest, imageId)
	//assert.NotEmpty(t, AntiopaImageInfo.Registry)
	//assert.NotEmpty(t, AntiopaImageInfo.Repository)
	//assert.NotEmpty(t, AntiopaImageInfo.Tag)
	//
	//// second step — get id from registry and signal for restart
	//// set http url for image (for local registry as container)
	//// and reset DockerRegistry
	//rlog.Debugf("Registry Manager Test: STEP 2")
	//DockerRegistry = nil
	//DockerRegistryInfo[AntiopaImageInfo.Registry] = map[string]string{
	//	"url": fmt.Sprintf("http://%s", AntiopaImageInfo.Registry),
	//}
	//// Change AntiopaImageId to zeros
	//AntiopaImageDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	//
	//go CheckIsImageUpdated()
	//registryImageId := <-ImageUpdated
	//assert.NotNil(t, DockerRegistry)
	//assert.NotEmpty(t, registryImageId)
	//assert.Equal(t, imageId, registryImageId, "image digest from registry must be equal to image digest from kube")
}
