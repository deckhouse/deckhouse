/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package steps

import (
	"context"
	"github.com/stretchr/testify/assert"
	pkg_cfg "system-registry-manager/pkg/cfg"
	"testing"
)

func createBundleForTest(ctx context.Context, params *InputParams) (*FilesBundle, error) {
	manifestsSpec := pkg_cfg.NewManifestsSpecForTest()
	return CreateBundle(ctx, manifestsSpec, params)
}

func generateInputConfigForTest() error {
	return pkg_cfg.InitConfigForTests(pkg_cfg.FileConfig{
		HostName: "filehostname",
		HostIP:   "filemyip",
		PodName:  "filepodname",
		Manager: struct {
			Namespace      string "mapstructure:\"namespace\""
			DaemonsetName  string "mapstructure:\"daemonsetName\""
			ServiceName    string "mapstructure:\"serviceName\""
			WorkerPort     int    "mapstructure:\"workerPort\""
			LeaderElection struct {
				LeaseDurationSeconds int "mapstructure:\"leaseDurationSeconds\""
				RenewDeadlineSeconds int "mapstructure:\"renewDeadlineSeconds\""
				RetryPeriodSeconds   int "mapstructure:\"retryPeriodSeconds\""
			} "mapstructure:\"leaderElection\""
		}{
			Namespace:     "filenamespace",
			DaemonsetName: "filedaemonsetname",
			ServiceName:   "fileservicename",
			WorkerPort:    123,
			LeaderElection: struct {
				LeaseDurationSeconds int "mapstructure:\"leaseDurationSeconds\""
				RenewDeadlineSeconds int "mapstructure:\"renewDeadlineSeconds\""
				RetryPeriodSeconds   int "mapstructure:\"retryPeriodSeconds\""
			}{
				LeaseDurationSeconds: 7200,
				RenewDeadlineSeconds: 20,
				RetryPeriodSeconds:   8,
			},
		},
		// Add new fields
		Etcd: struct {
			Addresses []string `mapstructure:"addresses"`
		}{
			Addresses: []string{"etcd1.example.com", "etcd2.example.com"},
		},
		Registry: struct {
			RegistryMode     string "mapstructure:\"registryMode\""
			UpstreamRegistry struct {
				UpstreamRegistryHost     string "mapstructure:\"upstreamRegistryHost\""
				UpstreamRegistryScheme   string "mapstructure:\"upstreamRegistryScheme\""
				UpstreamRegistryCa       string "mapstructure:\"upstreamRegistryCa\""
				UpstreamRegistryPath     string "mapstructure:\"upstreamRegistryPath\""
				UpstreamRegistryUser     string "mapstructure:\"upstreamRegistryUser\""
				UpstreamRegistryPassword string "mapstructure:\"upstreamRegistryPassword\""
			} "mapstructure:\"upstreamRegistry\""
		}{
			RegistryMode: "Proxy",
			UpstreamRegistry: struct {
				UpstreamRegistryHost     string "mapstructure:\"upstreamRegistryHost\""
				UpstreamRegistryScheme   string "mapstructure:\"upstreamRegistryScheme\""
				UpstreamRegistryCa       string "mapstructure:\"upstreamRegistryCa\""
				UpstreamRegistryPath     string "mapstructure:\"upstreamRegistryPath\""
				UpstreamRegistryUser     string "mapstructure:\"upstreamRegistryUser\""
				UpstreamRegistryPassword string "mapstructure:\"upstreamRegistryPassword\""
			}{
				UpstreamRegistryHost:     "TestUpstreamRegistryHost",
				UpstreamRegistryScheme:   "TestUpstreamRegistryScheme",
				UpstreamRegistryCa:       "TestUpstreamRegistryCa",
				UpstreamRegistryPath:     "TestUpstreamRegistryPath",
				UpstreamRegistryUser:     "TestUpstreamRegistryUser",
				UpstreamRegistryPassword: "TestUpstreamRegistryPassword",
			},
		},
		Images: struct {
			SystemRegistry struct {
				DockerDistribution string "mapstructure:\"dockerDistribution\""
				DockerAuth         string "mapstructure:\"dockerAuth\""
				Seaweedfs          string "mapstructure:\"seaweedfs\""
			} "mapstructure:\"systemRegistry\""
		}{
			SystemRegistry: struct {
				DockerDistribution string "mapstructure:\"dockerDistribution\""
				DockerAuth         string "mapstructure:\"dockerAuth\""
				Seaweedfs          string "mapstructure:\"seaweedfs\""
			}{
				DockerDistribution: "distribution_image",
				DockerAuth:         "auth_image",
				Seaweedfs:          "seaweedfs_image",
			},
		},
	})
}

func TestCreateBundle(t *testing.T) {
	err := generateInputConfigForTest()
	assert.NoError(t, err)

	params := InputParams{
		Certs:     struct{ UpdateOrCreate bool }{UpdateOrCreate: true},
		Manifests: struct{ UpdateOrCreate bool }{UpdateOrCreate: true},
		StaticPods: struct {
			UpdateOrCreate       bool
			MasterPeers          []string
			CheckWithMasterPeers bool
		}{
			UpdateOrCreate:       true,
			CheckWithMasterPeers: true,
			MasterPeers:          []string{"123", "321"},
		},
	}
	_, err = createBundleForTest(context.Background(), &params)
	assert.NoError(t, err)
}
