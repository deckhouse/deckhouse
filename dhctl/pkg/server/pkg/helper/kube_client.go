// Copyright 2024 Flant JSC
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

package helper

import (
	"context"
	"fmt"
	"os"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/util"
)

type ApiServerOptions struct {
	Token                    string
	InsecureSkipTLSVerify    bool
	CertificateAuthorityData []byte
}

func CreateKubeClient(ctx context.Context, apiServerUrl string, opts ApiServerOptions) (*client.KubernetesClient, func() error, error) {
	kubeConfig, cleanup, err := GenerateTempKubeConfig(apiServerUrl, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("generating kube config: %w", err)
	}

	kubeCl := client.NewKubernetesClient()

	if err := kubeCl.InitContext(ctx, &client.KubernetesInitParams{
		KubeConfig: kubeConfig,
	}); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("open kubernetes connection: %w", err)
	}

	return kubeCl, cleanup, nil
}

func GenerateTempKubeConfig(apiServerURL string, opts ApiServerOptions) (string, func() error, error) {
	cfg := api.NewConfig()

	cluster := api.NewCluster()
	cluster.Server = apiServerURL
	cluster.InsecureSkipTLSVerify = opts.InsecureSkipTLSVerify
	cluster.CertificateAuthorityData = opts.CertificateAuthorityData

	context := api.NewContext()
	context.Cluster = "cluster"

	if opts.Token != "" {
		context.AuthInfo = "user"
		authInfo := api.NewAuthInfo()
		authInfo.Token = opts.Token
		cfg.AuthInfos["user"] = authInfo
	}

	cfg.Clusters["cluster"] = cluster
	cfg.Contexts["default"] = context
	cfg.CurrentContext = "default"

	return util.WriteTempFile("", "kubeconfig-", func(f *os.File) error {
		if err := clientcmd.WriteToFile(*cfg, f.Name()); err != nil {
			return fmt.Errorf("error writing kubeconfig temp file %s: %w", f.Name(), err)
		}
		return nil
	})
}
