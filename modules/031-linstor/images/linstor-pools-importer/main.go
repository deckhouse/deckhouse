/*
Copyright 2022 Flant JSC

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
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)

	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"

	lclient "github.com/LINBIT/golinstor/client"
)

const (
	lvmConfig      = `devices {filter=["r|^/dev/drbd*|"]}`
	linstorPrefix  = "linstor"
	maxReplicasNum = 3
)

var (
	nodeName  = os.Getenv("NODE_NAME")
	podName   = os.Getenv("POD_NAME")
	namespace = os.Getenv("NAMESPACE")
)

var supportedProviderKinds = []lclient.ProviderKind{lclient.LVM, lclient.LVM_THIN}

func printVersion() {
	klog.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	klog.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
}

func main() {

	delaySeconds := flag.Int("delay", 10, "Delay in seconds between scanning attempts")

	flag.Parse()
	printVersion()

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)

	if nodeName == "" {
		klog.Fatalln("Required NODE_NAME env variable is not specified!")
	}
	klog.Infof("nodeName: %+v", nodeName)
	if podName == "" {
		klog.Fatalln("Required POD_NAME env variable is not specified!")
	}
	klog.Infof("podName: %+v", podName)
	if namespace == "" {
		klog.Fatalln("Required NAMESPACE env variable is not specified!")
	}
	klog.Infof("namespace: %+v", namespace)

	ctx := context.TODO()

	// Get a config to talk to the apiserver
	config, err := clientConfig.ClientConfig()
	if err != nil {
		klog.Errorln("Failed to get kubernetes config:", err)
	}

	kc, err := kclient.New(config, kclient.Options{})
	if err != nil {
		klog.Fatalln("Failed to create Kubernetes client:", err)
	}

	var pod v1.Pod
	err = kc.Get(ctx, types.NamespacedName{Name: podName, Namespace: namespace}, &pod)
	if err != nil {
		klog.Fatalf("look up owner(s) of pod: %v", err)
	}
	owner := v1.ObjectReference{
		APIVersion: "v1",
		Kind:       "Pod",
		Name:       pod.GetName(),
		Namespace:  pod.GetNamespace(),
		UID:        pod.GetUID(),
	}
	klog.Infof("using %s/%s as owner for Kubernetes events", owner.Kind, owner.Name)

	lc, err := lclient.NewClient()
	if err != nil {
		klog.Fatalln("failed to create LINSTOR client:", err)
	}

	klog.Infof("Starting main loop (delay: %d seconds)", *delaySeconds)
	candidatesChannel := runCandidatesLoop(ctx, getCandidates, time.Duration(*delaySeconds)*time.Second)

	for candidate := range candidatesChannel {
		klog.Infof("Got %s candidate: %+v\n", candidate.GetProviderKind(), candidate)
		// Create storage pool in LINSTOR
		storagePool, err := makeLinstorStoragePool(candidate)
		if err != nil {
			klog.Fatalln("failed to generate LINSTOR storage pool:", err)
		}
		_, err = lc.Nodes.Get(ctx, nodeName)
		if err != nil {
			klog.Fatalln("Failed to get LINSTOR node", err)
		}
		_, err = lc.Nodes.GetStoragePool(ctx, nodeName, storagePool.StoragePoolName)
		if err != nil { // TODO check for 404
			err = lc.Nodes.CreateStoragePool(ctx, nodeName, storagePool)
			if err != nil {
				event := makeKubernetesEvent(&owner, v1.EventTypeWarning, "Failed", "Failed to create LINSTOR storage pool"+err.Error())
				err = kc.Create(ctx, &event)
				if err != nil {
					klog.Errorln("Failed to create event", err)
				}
				klog.Fatalln("Failed to create LINSTOR storage pool", err)
			}
			event := makeKubernetesEvent(&owner, v1.EventTypeNormal, "Created", "Created LINSTOR storage pool: "+nodeName+"/"+storagePool.StoragePoolName)
			if err = kc.Create(ctx, &event); err != nil {
				klog.Fatalln("Failed to create event", err)
			}
		}

		// Get the maximum number of available replicas
		opts := lclient.ListOpts{StoragePool: []string{storagePool.StoragePoolName}}
		sp, err := lc.Nodes.GetStoragePoolView(ctx, &opts)
		if err != nil {
			klog.Fatalln("Failed to list LINSTOR storage pools", err)
		}
		replicasNum := len(sp)
		if replicasNum > maxReplicasNum {
			replicasNum = maxReplicasNum
		}

		// Create StorageClasses in Kubernetes
		for r := 1; r <= replicasNum; r++ {
			storageClass, err := makeKubernetesStorageClass(candidate, r)
			if err != nil {
				klog.Fatalln("failed to generate Kubernetes storage class:", err)
			}
			err = kc.Get(ctx, types.NamespacedName{Name: storageClass.GetName()}, &storageClass)
			if err == nil { // TODO check for 404
				continue
			}
			err = kc.Create(ctx, &storageClass)
			if err != nil {
				event := makeKubernetesEvent(&owner, v1.EventTypeWarning, "Failed", "Failed to create Kubernetes storage class"+err.Error())
				if err := kc.Create(ctx, &event); err != nil {
					klog.Errorln("Failed to create event", err)
				}
				klog.Fatalln("Failed to create Kubernetes storageClass", err)
			}
			event := makeKubernetesEvent(&owner, v1.EventTypeNormal, "Created", "Created Kubernetes storage class: "+storageClass.Name)
			err = kc.Create(ctx, &event)
			if err != nil {
				klog.Fatalln("Failed to create event", err)
			}
		}
	}
}

// Makes loop over storage pool candidates, retruns channel of changed ones
func runCandidatesLoop(ctx context.Context, f func() ([]StoragePoolCandidate, error), delay time.Duration) <-chan StoragePoolCandidate {
	ch := make(chan StoragePoolCandidate)
	var oldCandidates []StoragePoolCandidate
	go func() {
		for {
			select {
			case <-ctx.Done():
				close(ch)
				return
			default:
				candidates, err := f()
				if err != nil {
					klog.Fatalln("failed to load candidates:", err)
				}
			LOOP:
				for _, candidate := range candidates {
					name := candidate.GetName()
					if name == "" {
						klog.Fatalln("storage pool name can't be empty")
					}
					for _, oldCandidate := range oldCandidates {
						if name == oldCandidate.GetName() {
							continue LOOP
						}
					}
					ch <- candidate
				}
				oldCandidates = candidates
				time.Sleep(delay)
			}
		}
	}()
	return ch
}

// Collects all storage pool candidates from the node
func getCandidates() ([]StoragePoolCandidate, error) {
	var candidates []StoragePoolCandidate

	// Getting LVM volume groups
	var vgs VolumeGroups
	err := vgs.LoadCandidates()
	if err != nil {
		return nil, fmt.Errorf("failed to get LVM volume groups: %s", err)
	}
	for _, vg := range vgs {
		candidates = append(candidates, vg)
	}

	// Getting LVM thin pools
	var tps ThinPools
	err = tps.LoadCandidates()
	if err != nil {
		return nil, fmt.Errorf("failed to get LVM thin pools: %s", err)
	}
	for _, tp := range tps {
		candidates = append(candidates, tp)
	}

	return candidates, nil
}

type StoragePoolCandidates interface {
	LoadCandidates() error
}

// Defines any type of LINSTOR storage pool candidate
type StoragePoolCandidate interface {
	GetName() string
	GetProviderKind() lclient.ProviderKind
	GetProps() map[string]string
}

type ThinPools []ThinPool
type ThinPool struct {
	Name   string
	VGName string
	Tags   []string
}

func (a *ThinPools) LoadCandidates() error {
	cmd := exec.Command("lvs", "-oname,vg_name,lv_attr,tags", "--separator=;", "--noheadings", "--config="+lvmConfig)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return err
	}
	tps, err := parseThinPools(out.String())
	for _, tp := range tps {
		*a = append(*a, tp)
	}
	if err != nil {
		return err
	}
	return nil
}
func parseThinPools(out string) ([]ThinPool, error) {
	var tps []ThinPool

	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		a := strings.Split(line, ";")
		if len(a) != 4 {
			return nil, fmt.Errorf("wrong line: %s", line)
		}
		name, vgName, lvAttr, tags := strings.TrimSpace(a[0]), a[1], a[2], strings.Split(a[3], ",")
		if name == "" {
			return nil, fmt.Errorf("name can't be empty")
		}
		if vgName == "" {
			return nil, fmt.Errorf("vgName can't be empty")
		}
		if lvAttr == "" {
			return nil, fmt.Errorf("lvAttr can't be empty")
		}
		if lvAttr[0:1] != "t" {
			continue
		}
		if tags[0] == "" {
			continue
		}
		tps = append(tps, ThinPool{
			Name:   name,
			VGName: vgName,
			Tags:   tags,
		})
	}

	return tps, nil
}

func (a ThinPool) GetName() string {
	for _, tag := range a.Tags {
		t := strings.Split(tag, "-")
		if t[0] == linstorPrefix && t[1] != "" {
			return t[1]
		}
	}
	return ""
}
func (a ThinPool) GetProviderKind() lclient.ProviderKind { return lclient.LVM_THIN }
func (a ThinPool) GetProps() map[string]string {
	return map[string]string{
		"StorDriver/LvmVg":    a.VGName,
		"StorDriver/ThinPool": a.Name,
	}
}

type VolumeGroups []VolumeGroup
type VolumeGroup struct {
	Name string
	Tags []string
}

func (a *VolumeGroups) LoadCandidates() error {
	cmd := exec.Command("vgs", "-oname,tags", "--separator=;", "--noheadings", "--config="+lvmConfig)

	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return err
	}
	vgs, err := parseVolumeGroups(out.String())
	for _, vg := range vgs {
		*a = append(*a, vg)
	}
	if err != nil {
		return err
	}
	return nil
}
func parseVolumeGroups(out string) ([]VolumeGroup, error) {
	var vgs []VolumeGroup

	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		a := strings.Split(line, ";")
		if len(a) != 2 {
			return nil, fmt.Errorf("wrong line: %s", line)
		}
		name, tags := strings.TrimSpace(a[0]), strings.Split(a[1], ",")
		if name == "" {
			return nil, fmt.Errorf("name can't be empty")
		}
		if tags[0] == "" {
			continue
		}
		vgs = append(vgs, VolumeGroup{
			Name: name,
			Tags: tags,
		})
	}

	return vgs, nil
}

func (a VolumeGroup) GetName() string {
	for _, tag := range a.Tags {
		t := strings.Split(tag, "-")
		if t[0] == linstorPrefix && t[1] != "" {
			return t[1]
		}
	}
	return ""
}
func (a VolumeGroup) GetProviderKind() lclient.ProviderKind { return lclient.LVM }
func (a VolumeGroup) GetProps() map[string]string {
	return map[string]string{
		"StorDriver/LvmVg": a.Name,
	}
}

func makeLinstorStoragePool(c StoragePoolCandidate) (lclient.StoragePool, error) {
	sp := lclient.StoragePool{
		StoragePoolName: c.GetName(),
		ProviderKind:    c.GetProviderKind(),
		Props:           c.GetProps(),
	}
	if c.GetName() == "" {
		return sp, fmt.Errorf("storage pool name can't be empty")
	}
	if sp.Props == nil {
		return sp, fmt.Errorf("storage pool properties can't be empty")
	}
	suported := false
	for _, k := range supportedProviderKinds {
		if sp.ProviderKind == k {
			suported = true
			break
		}
	}
	if !suported {
		return sp, fmt.Errorf("storage pool providerKind %s is not supported", sp.ProviderKind)
	}

	return sp, nil
}

func makeKubernetesEvent(owner *v1.ObjectReference, eventType, reason, message string) v1.Event {
	eventTime := metav1.Now()
	event := v1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    owner.Namespace,
			GenerateName: owner.Name,
			Labels: map[string]string{
				"app": "linstor-pools-importer",
			},
		},
		Reason:         reason,
		Message:        message,
		InvolvedObject: *owner,
		Source: v1.EventSource{
			Component: "linstor-pools-importer",
			Host:      nodeName,
		},
		Count:          1,
		FirstTimestamp: eventTime,
		LastTimestamp:  eventTime,
		Type:           v1.EventTypeNormal,
	}
	return event
}

func makeKubernetesStorageClass(c StoragePoolCandidate, r int) (storagev1.StorageClass, error) {
	volBindMode := storagev1.VolumeBindingImmediate
	allowVolumeExpansion := true
	reclaimPolicy := v1.PersistentVolumeReclaimDelete
	name := c.GetName()
	if name == "" {
		return storagev1.StorageClass{}, fmt.Errorf("storage pool name can't be empty")
	}
	return storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s-r%d", linstorPrefix, name, r),
		},
		Provisioner:          "linstor.csi.linbit.com",
		VolumeBindingMode:    &volBindMode,
		AllowVolumeExpansion: &allowVolumeExpansion,
		ReclaimPolicy:        &reclaimPolicy,
		Parameters: map[string]string{
			"linstor.csi.linbit.com/storagePool":    name,
			"linstor.csi.linbit.com/placementCount": fmt.Sprintf("%d", r),
		},
	}, nil
}
