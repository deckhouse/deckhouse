/*
Copyright 2026 Flant JSC

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

// Package testenv holds controller-agnostic helpers for envtest-based integration suites.
// It boots a real apiserver, wires registered controllers into a manager, and provides
// common test plumbing (unique names, finalizer stripping, kubectl-style dumps, a pause
// hook). It is free of Ginkgo/Gomega so it can back any test runner.
//
// # Bootstrap
//
// [Start] boots an envtest apiserver and returns a client; stop it in AfterSuite.
// [BinaryAssetsDir] locates kubebuilder assets (KUBEBUILDER_ASSETS overrides); use
// [AssetsAvailable] to Skip when missing. [NewManager] builds a manager with metrics
// and leader-election off, wiring controllers registered via register.RegisterController.
//
// # CRD paths
//
// [ControllerCRDPaths] and [NodeManagerCRDPaths] resolve CRD YAMLs from
// node-controller/crds and 040-node-manager/crds by walking up to the module root.
//
// # Test plumbing
//
// [UniqueName] yields unique DNS-safe names. [RemoveFinalizers] strips finalizers with
// re-get/retry. [SetupLogger] silences logs (ENVTEST_LOGS=1 turns them on). [DebugEnabled]
// gates dumps from [KubectlDumpNodeObjects]. [PauseForKubectl] blocks for manual kubectl.
package testenv
