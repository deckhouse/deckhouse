# Patching CDI or Kubevirt

Kubevirt instructions are the same, just use repository `https://github.com/kubevirt/kubevirt.git`.

### Generate CRD

Clone CDI sources using `$version` from cdi-artifact/werf.inc.yaml, apply patches and call `make manifests`.

```bash
mkdir tmp
export VERSION="1.58.0"
git clone --depth 1 --branch v${VERSION} https://github.com/kubevirt/containerized-data-importer.git tmp/cdi
cd tmp/cdi
git apply ../../images/cdi-artifact/patches/*.patch
make manifests
yq e '. | select(.kind == "CustomResourceDefinition")' _out/manifests/release/cdi-operator.yaml > ../../crds/cdi.yaml
cd ../../
rm -rf tmp/cdi
```

### Work with patches

Use task to start working on a new patch:

```bash
task patch:new
```

Use task to change an existing patch:

```bash
task patch:edit -- 006-customizer.patch
```


### Porting patches to newer version of CDI

We will bump CDI version in the future and patches will require porting. Apply existing patches using --ignore-space-change and --ignore-whitespace to ignore trivial conflicts and create patches again.

#### Prepare
```bash
mkdir tmp
export VERSION="1.58.1"
git clone --depth 1 --branch v${VERSION} https://github.com/kubevirt/containerized-data-importer.git tmp/cdi
cd tmp/cdi
git checkout -b update-patches
```

#### Generate updated patches
```bash
git apply --ignore-space-change --ignore-whitespace ../../images/cdi-artifact/patches/000-bundle-images.patch ## if patch failed - use --reject
git add . && git commit -m "patch1"
git apply --ignore-space-change --ignore-whitespace ../../images/cdi-artifact/patches/003-apiserver-node-selector-and-tolerations.patch ## if patch failed - use --reject
git add . && git commit -m "patch2"
git log 
git diff --patch "<TAG COMMIT>" "<Your first commit - patch1>" > 000-bundle-images.patch
git diff --patch "<Your first commit - patch1>" "<Your second commit - patch2>" > 003-apiserver-node-selector-and-tolerations.patch
```
#### Copy new patches
```bash
cp 000-bundle-images.patch ../../images/cdi-artifact/patches/000-bundle-images.patch
cp 003-apiserver-node-selector-and-tolerations.patch ../../images/cdi-artifact/patches/003-apiserver-node-selector-and-tolerations.patch
```

#### Сlean
```bash
cd ../../
rm -rf tmp/cdi
```

### Generate new patch from PR

Incorporate PR from upstream as a patch.

#### Prepare
```bash
mkdir tmp
export VERSION="1.58.0"
git clone https://github.com/kubevirt/containerized-data-importer.git tmp/cdi
cd tmp/cdi
git checkout tags/v${VERSION}
git switch -c update-patches
export PULL_REQUEST_ID=2563 ## pr id for 004-replicas.patch
git fetch origin pull/$PULL_REQUEST_ID/head:patch3
```
#### Generate patches
```bash
git merge patch3 update-patches
git log 
git diff --patch "<TAG COMMIT>" HEAD > 004-replicas.patch
```
#### Copy new patches
```bash
cp 004-replicas.patch ../../images/cdi-artifact/patches/004-replicas.patch
git add ../../images/cdi-artifact/patches/004-replicas.patch
```
#### Сlean
```bash
cd ../../
rm -rf tmp/cdi
```
