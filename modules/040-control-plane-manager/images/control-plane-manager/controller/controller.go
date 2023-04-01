/*
Copyright 2023 Flant JSC

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
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetFormatter(&log.JSONFormatter{})

	config, err := NewConfig()
	if err != nil {
		log.Fatal(err)
	}

	if err := removeOrphanFiles(config); err != nil {
		log.Warn(err)
	}

	if err := annotateNode(config); err != nil {
		log.Fatal(err)
	}

	if err := waitNodeApproval(config); err != nil {
		log.Fatal(err)
	}

	if err := waitImageHolderContainers(config); err != nil {
		log.Fatal(err)
	}

	if err := checkEtcdManifest(config); err != nil {
		log.Fatal(err)
	}

	if err := checkKubeletConfig(); err != nil {
		log.Fatal(err)
	}

	if err := installKubeadmConfig(config); err != nil {
		log.Fatal(err)
	}

	if err := installBasePKIfiles(config); err != nil {
		log.Fatal(err)
	}

	if err := fillTmpDirWithPKIData(config); err != nil {
		log.Fatal(err)
	}

	if err := renewCertificates(config); err != nil {
		log.Fatal(err)
	}

	if err := renewKubeconfigs(config); err != nil {
		log.Fatal(err)
	}

	if err := updateRootKubeconfig(); err != nil {
		log.Fatal(err)
	}

	if err := installExtraFiles(config); err != nil {
		log.Fatal(err)
	}

	if err := convergeComponents(config); err != nil {
		log.Fatal(err)
	}

	// pause loop
	<-config.Quit
}
