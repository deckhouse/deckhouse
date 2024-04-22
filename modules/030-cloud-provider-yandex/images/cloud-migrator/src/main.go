/*
Copyright 2024 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"flag"
	"os"

	log "github.com/sirupsen/logrus"
)

func main() {
	var (
		kubeConfig  string
		folderID    string
		saKeyJSON   string
		clusterUUID string
	)

	folderID = os.Getenv("YC_FOLDER_ID")
	kubeConfig = os.Getenv("KUBE_CONFIG")
	saKeyJSON = os.Getenv("YC_SA_KEY_JSON")
	clusterUUID = os.Getenv("CLUSTER_UUID")

	flag.StringVar(&kubeConfig, "kube-config", kubeConfig, "Path to kube-config")
	flag.StringVar(&folderID, "folder-id", folderID, "Yandex folder id")
	flag.StringVar(&saKeyJSON, "sa-key-json", saKeyJSON, "Yandex SA key in JSON format")
	flag.StringVar(&saKeyJSON, "cluster-uuid", clusterUUID, "Cluster UUID")

	flag.Parse()

	var formatter log.Formatter = &log.TextFormatter{
		DisableColors:   true,
		TimestampFormat: "2006-01-02 15:04:05",
		FullTimestamp:   true,
	}
	formatter = &log.JSONFormatter{}

	logger := log.New()
	logger.SetLevel(log.InfoLevel)
	logger.SetFormatter(formatter)

	if folderID == "" {
		logger.Fatal("Cannot get YC_FOLDER_ID")
	}

	if saKeyJSON == "" {
		logger.Fatal("Cannot get YC_SA_KEY_JSON")
	}

	if clusterUUID == "" {
		logger.Fatal("Cannot get CLUSTER_UUID")
	}

	// init kube clients
	client, err := InitClient(kubeConfig)
	if err != nil {
		logger.Fatal(err)
	}

	d := NewDiskMigrator(log.NewEntry(logger), client, folderID, saKeyJSON, clusterUUID)
	err = d.MigrateDisks(context.Background())
	if err != nil {
		logger.Fatal(err)
	}
}
