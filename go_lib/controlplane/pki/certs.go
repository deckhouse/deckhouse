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

package pki

import "strings"

type RootCertBaseName string

const (
	CACertBaseName           RootCertBaseName = "ca"
	FrontProxyCACertBaseName RootCertBaseName = "front-proxy-ca"
	EtcdCACertBaseName       RootCertBaseName = "etcd/ca"
)

type LeafCertBaseName string

const (
	ApiserverCertBaseName              LeafCertBaseName = "apiserver"
	ApiserverKubeletClientCertBaseName LeafCertBaseName = "apiserver-kubelet-client"
	FrontProxyClientCertBaseName       LeafCertBaseName = "front-proxy-client"
	EtcdServerCertBaseName             LeafCertBaseName = "etcd/server"
	EtcdPeerCertBaseName               LeafCertBaseName = "etcd/peer"
	EtcdHealthcheckClientCertBaseName  LeafCertBaseName = "etcd/healthcheck-client"
	ApiserverEtcdClientCertBaseName    LeafCertBaseName = "apiserver-etcd-client"
)

// FlatBaseName converts a cert base name that may contain a directory component
// (e.g. "etcd/ca") into a flat, single-segment name (e.g. "etcd-ca"). Use it wherever
// a cert needs to be addressed by a key that cannot contain a path separator -
// for example a Kubernetes Secret data key.
func FlatBaseName(baseName string) string {
	return strings.ReplaceAll(baseName, "/", "-")
}
