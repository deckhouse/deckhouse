/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package steps

type InputParams struct {
	Certs struct {
		UpdateOrCreate bool
	}
	Manifests struct {
		UpdateOrCreate bool
	}
	StaticPods struct {
		UpdateOrCreate       bool
		MasterPeers          []string
		CheckWithMasterPeers bool
	}
}

type FileCheck struct {
	NeedCreate bool
	NeedUpdate bool
}

func (fCheck *FileCheck) NeedCreateOrUpdate() bool {
	return fCheck.NeedCreate || fCheck.NeedUpdate
}

type FileBundle struct {
	Content  string
	DestPath string
}

type ManifestBundle struct {
	Check FileCheck
	File  FileBundle
}

type StaticPodBundle struct {
	Check FileCheck
	File  FileBundle
}

type CertBundle struct {
	Check FileCheck
	Key   FileBundle
	Cert  FileBundle
}

type FilesBundle struct {
	Manifests  []ManifestBundle
	StaticPods []StaticPodBundle
	Certs      []CertBundle
}

func (f *FilesBundle) ManifestsIsExist() bool {
	for _, manifest := range f.Manifests {
		if manifest.Check.NeedCreate {
			return false
		}
	}
	return true
}

func (f *FilesBundle) StaticPodsIsExist() bool {
	for _, staticPod := range f.StaticPods {
		if staticPod.Check.NeedCreate {
			return false
		}
	}
	return true
}

func (f *FilesBundle) CertificateIsExist() bool {
	for _, cert := range f.Certs {
		if cert.Check.NeedCreate {
			return false
		}
	}
	return true
}

func (f *FilesBundle) ManifestsWaitToUpdate() bool {
	for _, manifest := range f.Manifests {
		if manifest.Check.NeedUpdate {
			return true
		}
	}
	return false
}

func (f *FilesBundle) StaticPodsWaitToUpdate() bool {
	for _, staticPod := range f.StaticPods {
		if staticPod.Check.NeedUpdate {
			return true
		}
	}
	return false
}

func (f *FilesBundle) CertificatesWaitToUpdate() bool {
	for _, cert := range f.Certs {
		if cert.Check.NeedUpdate {
			return true
		}
	}
	return false
}
