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

type rootCertName string

const (
	caCertName           rootCertName = "ca"
	frontProxyCaCertName rootCertName = "front-proxy-ca"
	etcdCaCertName       rootCertName = "etcd-ca"
)

type leafCertName string

const (
	apiserverCertName              leafCertName = "apiserver"
	apiserverKubeletClientCertName leafCertName = "apiserver-kubelet-client"
	frontProxyClientCertName       leafCertName = "front-proxy-client"
	etcdServerCertName             leafCertName = "etcd-server"
	etcdPeerCertName               leafCertName = "etcd-peer"
	etcdHealthcheckClientCertName  leafCertName = "etcd-healthcheck-client"
	apiserverEtcdClientCertName    leafCertName = "apiserver-etcd-client"
)
