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

const (
	SAPrivateKeyFileName = "sa.key"
	SAPublicKeyFileName  = "sa.pub"
)

// FileLayout returns the mapping between the flat name of every PKI artifact this
// package can produce (e.g. "etcd-ca.crt") and its actual relative file path inside
// a PKI directory (e.g. "etcd/ca.crt"). It covers the full default cert tree
// (see defaultCertTreeScheme) plus the SA key pair.
//
// This is the single source of truth for flattening/unflattening the full PKI bundle,
// used both when materializing a flat key/value map onto disk and when reading a
// generated PKI bundle back into a flat map.
func FileLayout() map[string]string {
	layout := make(map[string]string)

	for _, root := range rootCertNames() {
		addCertFiles(layout, string(root))
	}
	for _, leaf := range leafCertNames() {
		addCertFiles(layout, string(leaf))
	}

	layout[SAPrivateKeyFileName] = SAPrivateKeyFileName
	layout[SAPublicKeyFileName] = SAPublicKeyFileName

	return layout
}

func rootCertNames() []RootCertBaseName {
	return []RootCertBaseName{
		CACertBaseName,
		FrontProxyCACertBaseName,
		EtcdCACertBaseName,
	}
}

func leafCertNames() []LeafCertBaseName {
	return []LeafCertBaseName{
		ApiserverCertBaseName,
		ApiserverKubeletClientCertBaseName,
		FrontProxyClientCertBaseName,
		EtcdServerCertBaseName,
		EtcdPeerCertBaseName,
		EtcdHealthcheckClientCertBaseName,
		ApiserverEtcdClientCertBaseName,
	}
}

func addCertFiles(layout map[string]string, baseName string) {
	flat := FlatBaseName(baseName)
	layout[flat+".crt"] = baseName + ".crt"
	layout[flat+".key"] = baseName + ".key"
}
