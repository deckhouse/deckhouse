package util

import (
	"fmt"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"os"
)

func GenerateTempKubeConfig(apiServerURL, token string) (string, func(), error) {
	cfg := api.NewConfig()

	cluster := api.NewCluster()
	cluster.Server = apiServerURL
	//cluster.InsecureSkipTLSVerify = true // For testing purposes only, consider using certificate validation in production

	context := api.NewContext()
	context.Cluster = "cluster"
	context.AuthInfo = "user"

	authInfo := api.NewAuthInfo()
	authInfo.Token = token

	cfg.Clusters["cluster"] = cluster
	cfg.Contexts["default"] = context
	cfg.AuthInfos["user"] = authInfo
	cfg.CurrentContext = "default"

	kubeconfigFile, err := os.CreateTemp("", "kubeconfig-")
	if err != nil {
		return "", nil, fmt.Errorf("error creating temp file: %w", err)
	}
	cleanupFunc := func() {
		os.Remove(kubeconfigFile.Name())
	}

	if err := clientcmd.WriteToFile(*cfg, kubeconfigFile.Name()); err != nil {
		defer cleanupFunc()
		return "", cleanupFunc, fmt.Errorf("error writing temp file %s: %w", err)
	}

	return kubeconfigFile.Name(), cleanupFunc, nil
}
