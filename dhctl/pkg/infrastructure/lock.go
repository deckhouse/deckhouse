// Copyright 2025 Flant JSC
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

package infrastructure

import (
	"os"
	"path/filepath"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func deleteLockFile(fileForDelete, logPrefix, nextActionLogString string, logger log.Logger) (pursue bool, err error) {
	err = os.Remove(fileForDelete)
	if err != nil {
		if !os.IsNotExist(err) {
			return false, err
		}

		logger.LogDebugF("%s file %s not found. %s\n", logPrefix, fileForDelete, nextActionLogString)
		return true, nil
	}

	logger.LogDebugF("%s file %s was found and deleted. \n", logPrefix, fileForDelete)
	return false, nil
}

func releaseInfrastructureProviderLock(dhctlDir, modulesDir, module, desiredModuleDir string, logger log.Logger) error {
	logger.LogDebugF("Releasing infrastructure provider lock. dhctl dir: %s; modules dir %s; module: %s; desired module dir: %s .\n",
		dhctlDir, modulesDir, module, desiredModuleDir)
	defer logger.LogDebugF("Releasing infrastructure provider lock finished.\n")

	// terraform and tofu use same file name for lock file
	const lockFile = ".terraform.lock.hcl"

	// first, we will process terraform case. Terraform 0.14 version save lock file in same location where terraform runs
	terraformLockFile := filepath.Join(dhctlDir, lockFile)
	logger.LogDebugF("Terraform lock file %s\n", terraformLockFile)

	_, err := deleteLockFile(terraformLockFile, "Terraform lock", "", logger)
	if err != nil {
		return err
	}

	// we need to continue processing for tofu because commander can work in next sequence
	// - converge tofu cluster
	// - converge terraform cluster
	// - converge tofu cluster
	// in this case we release lock from terraform because terraform lock was present, but tofu lock presents from
	// first run also present and is not deleted. So, we should continue to delete tofu locks in all cases
	log.DebugLn("Try to delete tofu lock files regardless of existing terraform lock.")

	// next, we will process tofu case. Latest terraform version and opentofu can save lock in modules dir (not in desired
	// module where tofu will run) and in desired module. I do not understand because this behavior happens

	tofuModulesLockFile := filepath.Join(modulesDir, module, lockFile)
	logger.LogDebugF("Tofu modules lock file %s\n", tofuModulesLockFile)

	pursue, err := deleteLockFile(tofuModulesLockFile, "Tofu modules lock", "Try to delete tofu lock file in module.", logger)
	if err != nil {
		return err
	}

	if !pursue {
		logger.LogDebugF("Tofu modules lock file %s was deleted. Hence we do not need delete tofu 'in module' lock file.\n", tofuModulesLockFile)
		return nil
	}

	tofuInModuleLockFile := filepath.Join(desiredModuleDir, lockFile)
	logger.LogDebugF("Tofu 'in module' lock file %s\n", tofuInModuleLockFile)

	_, err = deleteLockFile(tofuInModuleLockFile, "Tofu 'in module' lock", "", logger)
	if err != nil {
		return err
	}

	return nil
}
