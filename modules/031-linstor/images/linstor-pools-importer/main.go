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
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	lclient "github.com/LINBIT/golinstor/client"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	lvmConfig      = `devices {filter=["r|^/dev/drbd*|"]}`
	linstorPrefix  = "linstor"
	maxReplicasNum = 3
)

// Print version
func printVersion() {
	klog.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	klog.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
}

type options struct {
	namespace    string
	nodeName     string
	podName      string
	scanInterval int
}

func main() {

	// Parse inputs
	opts, err := parseInputs()
	if err != nil {
		klog.Fatalln("Failed to parse inputs", err)
	}

	// Print version
	printVersion()

	// Create Kubernetes client
	kc, err := createKubeClient()
	if err != nil {
		klog.Fatalln("...", err)
	}

	// Create context
	ctx, cancel := context.WithCancel(context.Background())

	// Get owner
	owner, err := getOwner(ctx, kc, opts.namespace, opts.podName)
	if err != nil {
		klog.Fatalln("failed to get owner pod:", err)
	}

	// Get LINSTOR client
	lc, err := lclient.NewClient()
	if err != nil {
		klog.Fatalln("failed to create LINSTOR client:", err)
	}

	klog.Infof("Starting main loop (scanInterval: %d seconds)", opts.scanInterval)

	// Main loop: configure storage pools and create storage classes
	stop := make(chan struct{})
	go func() {
		defer cancel()
		err := provisionStoragePools(ctx, lc, kc, &owner, opts.nodeName, time.Duration(opts.scanInterval)*time.Second)
		if errors.Is(err, context.Canceled) {
			// only occurs if the context was cancelled, and it only can be cancelled on SIGINT
			stop <- struct{}{}
			return
		}
		klog.Fatalln(err)
	}()

	// Blocks waiting signals from OS.
	shutdown(func() {
		cancel()
		<-stop
		os.Exit(0)
	})
}

// Parse inputs
func parseInputs() (options, error) {
	var opts options
	opts.scanInterval = *flag.Int("scan-interval", 10, "Delay in seconds between scanning attempts")
	klog.Infof("scanInterval: %d", opts.scanInterval)

	opts.nodeName = os.Getenv("NODE_NAME")
	if opts.nodeName == "" {
		return opts, fmt.Errorf("Required NODE_NAME env variable is not specified!")
	}
	klog.Infof("nodeName: %s", opts.nodeName)

	opts.podName = os.Getenv("POD_NAME")
	if opts.podName == "" {
		return opts, fmt.Errorf("Required POD_NAME env variable is not specified!")
	}

	klog.Infof("podName: %s", opts.podName)

	opts.namespace = os.Getenv("NAMESPACE")
	if opts.namespace == "" {
		return opts, fmt.Errorf("Required NAMESPACE env variable is not specified!")
	}
	klog.Infof("namespace: %s", opts.namespace)
	flag.Parse()
	return opts, nil
}

// Create Kubernetes client
func createKubeClient() (kclient.Client, error) {
	var kc kclient.Client
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)
	// Get a config to talk to the apiserver
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return kc, fmt.Errorf("Failed to get kubernetes config: %w", err)
	}
	kc, err = kclient.New(config, kclient.Options{})
	if err != nil {
		return kc, fmt.Errorf("Failed to create Kubernetes client: %w", err)
	}
	return kc, nil
}

// Get owner
func getOwner(ctx context.Context, kc kclient.Client, namespace, podName string) (v1.ObjectReference, error) {
	var owner v1.ObjectReference
	var pod v1.Pod
	err := kc.Get(ctx, types.NamespacedName{Name: podName, Namespace: namespace}, &pod)
	if err != nil {
		return owner, fmt.Errorf("look up owner(s) of pod %s/%s: %v", namespace, podName, err)
	}
	owner = v1.ObjectReference{
		APIVersion: "v1",
		Kind:       "Pod",
		Name:       pod.GetName(),
		Namespace:  pod.GetNamespace(),
		UID:        pod.GetUID(),
	}
	klog.Infof("using %s/%s as owner for Kubernetes events", owner.Kind, owner.Name)
	return owner, nil
}

func provisionStoragePools(ctx context.Context, lc *lclient.Client, kc kclient.Client, owner *v1.ObjectReference, nodeName string, scanInterval time.Duration) error {
	candiCh := make(chan Candidate)
	errCh := make(chan error)

	go func() {
		seen := make(map[string]struct{})

		for {
			candidates, err := getCandidates(nodeName)
			if err != nil {
				// only fatal error, cannot cmd
				errCh <- err
				return
			}
			for _, cand := range candidates {
				if _, yes := seen[cand.UUID]; yes {
					continue
				}
				seen[cand.UUID] = struct{}{}
				candiCh <- cand
			}
			time.Sleep(scanInterval)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case err := <-errCh:
			return fmt.Errorf("Cannot get storage pool candidates: %w", err)

		case cand := <-candiCh:
			if cand.SkipReason != "" {
				klog.Infof("Skip %s as it %s", cand.Name, cand.SkipReason)
				continue
			}
			klog.Infof("Processing %s", cand.Name)

			changed, err := syncNodeStoragePool(ctx, lc, cand)
			if err != nil {
				// only abortions possible here
				err2 := reportFailed(ctx, kc, nodeName, owner, "Failed to sync LINSTOR storage pool: "+err.Error())
				if err2 != nil {
					klog.Fatalln("Failed to create event", err2)
				}
				return err
			}

			if changed {
				if err := reportCreated(ctx, kc, nodeName, owner, "Created LINSTOR storage pool: "+nodeName+"/"+cand.StoragePool.StoragePoolName); err != nil {
					return err
				}
			} else {
				klog.Info("LINSTOR storage pool " + nodeName + "/" + cand.StoragePool.StoragePoolName + " is already configured")
			}

			scs, err := genKubernetesStorageClasses(ctx, lc, cand)
			if err != nil {
				// only abortions possible here
				err2 := reportFailed(ctx, kc, nodeName, owner, "Failed to generate Kubernetes storage classes: "+err.Error())
				if err2 != nil {
					klog.Fatalln("Failed to create event", err2)
				}
				return err
			}

			for _, sc := range scs {
				changed, err := syncKubernetesStorageClass(ctx, kc, sc)
				if err != nil {
					// only abortions possible here
					err2 := reportFailed(ctx, kc, nodeName, owner, "Failed to sync Kubernetes storage class: "+err.Error())
					if err2 != nil {
						klog.Fatalln("Failed to create event", err2)
					}
					return err
				}

				if changed {
					if err := reportCreated(ctx, kc, nodeName, owner, "Created Kubernetes storage class: "+sc.GetName()); err != nil {
						return err
					}
				} else {
					klog.Info("Kubernetes storage class " + sc.GetName() + " is already configured")
				}
			}

		}
	}
}

func genKubernetesStorageClasses(ctx context.Context, lc *lclient.Client, cand Candidate) ([]storagev1.StorageClass, error) {
	var scs []storagev1.StorageClass
	// Get the maximum number of available replicas
	opts := lclient.ListOpts{
		StoragePool: []string{cand.StoragePool.StoragePoolName},
		Limit:       maxReplicasNum,
	}
	sps, err := lc.Nodes.GetStoragePoolView(ctx, &opts)
	if err != nil {
		return nil, fmt.Errorf("Failed to list LINSTOR storage pools: %w", err)
	}
	replicasNum := len(sps)
	if replicasNum > maxReplicasNum {
		replicasNum = maxReplicasNum
	}

	// Create StorageClasses in Kubernetes
	for r := 1; r <= replicasNum; r++ {
		scs = append(scs, newKubernetesStorageClass(&cand.StoragePool, r))
	}
	return scs, nil
}

func syncKubernetesStorageClass(ctx context.Context, kc kclient.Client, sc storagev1.StorageClass) (bool, error) {
	// Check old storage class https://t.me/meta_tractor
	err := kc.Get(ctx, types.NamespacedName{Name: sc.GetName()}, &sc)
	if err == nil {
		return false, nil
	}
	if !kerrors.IsNotFound(err) {
		return false, fmt.Errorf("Failed to get Kubernetes storage class: %w", err)
	}
	// Create new storage class
	err = kc.Create(ctx, &sc)
	if err != nil {
		if kerrors.IsAlreadyExists(err) {
			return false, nil
		} else {
			return false, fmt.Errorf("Failed to create Kubernetes storage class: %w", err)
		}
	}
	return true, nil
}

func syncNodeStoragePool(ctx context.Context, lc *lclient.Client, cand Candidate) (bool, error) {
	// Check old storage pool
	_, err := lc.Nodes.Get(ctx, cand.StoragePool.NodeName)
	if err != nil {
		return false, fmt.Errorf("Failed to get LINSTOR node: %w", err)
	}
	_, err = lc.Nodes.GetStoragePool(ctx, cand.StoragePool.NodeName, cand.StoragePool.StoragePoolName)
	if err == nil {
		return false, nil
	}
	if err != lclient.NotFoundError {
		return false, fmt.Errorf("Failed to get LINSTOR storage pool: %w", err)
	}

	// Create new storage pool
	err = lc.Nodes.CreateStoragePool(ctx, cand.StoragePool.NodeName, cand.StoragePool)
	if err != nil {
		return false, fmt.Errorf("Failed to create LINSTOR storage pool: %w", err)
	}
	return true, nil
}

// shutdown waits for SIGINT or SIGTERM and runs a callback function.
//
// First signal start a callback function, which should call os.Exit(0).
// Next signal will force exit with os.Exit(128 + signalValue). If no cb is given, the exist is also forced.
func shutdown(cb func()) {
	exitGracefully := cb != nil

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

	for {
		sig := <-ch

		if exitGracefully {
			exitGracefully = false
			klog.Infof("Shutdown called with %q", sig.String())
			go cb()
			continue
		}

		klog.Infof("Forced shutdown with %q", sig.String())
		signum := 0
		if v, ok := sig.(syscall.Signal); ok {
			signum = int(v)
		}
		os.Exit(128 + signum)
	}
}

// Log and send creation event to Kubernetes
func reportCreated(ctx context.Context, kc kclient.Client, nodeName string, owner *v1.ObjectReference, message string) error {
	klog.Info(message)
	event := newKubernetesEvent(nodeName, owner, v1.EventTypeNormal, "Created", message)
	return kc.Create(ctx, &event)
}

// Log and send failed event to Kubernetes
func reportFailed(ctx context.Context, kc kclient.Client, nodeName string, owner *v1.ObjectReference, message string) error {
	klog.Info(message)
	event := newKubernetesEvent(nodeName, owner, v1.EventTypeWarning, "Failed", message)
	return kc.Create(ctx, &event)
}

type VolumeGroups struct{}

func getLVMThinCandidates(nodeName string) ([]Candidate, error) {
	cmd := exec.Command("lvs", "-oname,vg_name,lv_attr,uuid,tags", "--separator=;", "--noheadings", "--config="+lvmConfig)

	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	return parseLVMThinPools(nodeName, out.String())
}

type ThinPools struct{}

func getLVMCandidates(nodeName string) ([]Candidate, error) {
	cmd := exec.Command("vgs", "-oname,uuid,tags", "--separator=;", "--noheadings", "--config="+lvmConfig)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	return parseLVMVolumeGroups(nodeName, out.String())
}

type Candidate struct {
	Name        string
	UUID        string
	SkipReason  string
	StoragePool lclient.StoragePool
}

// Collects all storage pool candidates from the node
func getCandidates(nodeName string) ([]Candidate, error) {
	var candidates []Candidate

	// Getting LVM storage pools
	cs, err := getLVMCandidates(nodeName)
	if err != nil {
		return nil, fmt.Errorf("failed to read LVM storage pools: %s", err)
	}
	candidates = append(candidates, cs...)

	// Getting LVM thin storage pools
	cs, err = getLVMThinCandidates(nodeName)
	if err != nil {
		return nil, fmt.Errorf("failed to read LVMthin storage pools: %s", err)
	}
	candidates = append(candidates, cs...)

	return candidates, nil
}

func parseLVMThinPools(nodeName, out string) ([]Candidate, error) {
	var sps []Candidate
	for _, line := range strings.Split(out, "\n") {
		var skipReason string
		// Parsing line
		if line == "" {
			continue
		}
		a := strings.Split(line, ";")
		if len(a) != 5 {
			return nil, fmt.Errorf("wrong line: %q", line)
		}
		lvName, vgName, lvAttr, uuid, tags := strings.TrimSpace(a[0]), a[1], a[2], a[3], strings.Split(a[4], ",")
		if lvName == "" {
			return nil, fmt.Errorf("LV name can't be empty (line: %q)", line)
		}
		if vgName == "" {
			return nil, fmt.Errorf("vgName can't be empty (line: %q)", line)
		}
		if lvAttr == "" {
			return nil, fmt.Errorf("lvAttr can't be empty (line: %q)", line)
		}
		if uuid == "" {
			return nil, fmt.Errorf("uuid can't be empty (line: %q)", line)
		}
		name, err := parseNameFromLVMTags(&tags)
		switch {
		case lvAttr[0:1] != "t":
			skipReason = "is not a thin pool"
		case err != nil:
			skipReason = "has no propper tag set: " + err.Error()
		}

		sps = append(sps, Candidate{
			Name:       "LVM Logical Volume " + vgName + "/" + lvName,
			UUID:       uuid,
			SkipReason: skipReason,
			StoragePool: lclient.StoragePool{
				StoragePoolName: name,
				NodeName:        nodeName,
				ProviderKind:    lclient.LVM_THIN,
				Props: map[string]string{
					"StorDriver/LvmVg":    vgName,
					"StorDriver/ThinPool": lvName,
				},
			}})
	}
	return sps, nil
}

func parseLVMVolumeGroups(nodeName, out string) ([]Candidate, error) {
	var sps []Candidate
	for _, line := range strings.Split(out, "\n") {
		var skipReason string
		// Parsing line
		if line == "" {
			continue
		}
		a := strings.Split(line, ";")
		if len(a) != 3 {
			return nil, fmt.Errorf("wrong line: %q", line)
		}
		vgName, uuid, tags := strings.TrimSpace(a[0]), a[1], strings.Split(a[2], ",")
		if vgName == "" {
			return nil, fmt.Errorf("VG name can't be empty (line: %q)", line)
		}
		if uuid == "" {
			return nil, fmt.Errorf("uuid can't be empty (line: %q)", line)
		}
		name, err := parseNameFromLVMTags(&tags)
		if err != nil {
			skipReason = "has no propper tag set: " + err.Error()
		}

		sps = append(sps, Candidate{
			Name:       "LVM Volume Group " + vgName,
			UUID:       uuid,
			SkipReason: skipReason,
			StoragePool: lclient.StoragePool{
				StoragePoolName: name,
				NodeName:        nodeName,
				ProviderKind:    lclient.LVM,
				Props: map[string]string{
					"StorDriver/LvmVg": vgName,
				},
			},
		})
	}
	return sps, nil
}

func parseNameFromLVMTags(tags *[]string) (string, error) {
	var foundNames []string
	for _, tag := range *tags {
		t := strings.Split(tag, "-")
		if t[0] == linstorPrefix && t[1] != "" {
			foundNames = append(foundNames, strings.TrimPrefix(tag, linstorPrefix+"-"))
		}
	}
	switch len(foundNames) {
	case 0:
		return "", errors.New("can't find tag with prefix " + linstorPrefix)
	case 1:
		return foundNames[0], nil
	default:
		return "", errors.New("found more than one tag with prefix " + linstorPrefix)
	}
}

func newKubernetesEvent(nodeName string, owner *v1.ObjectReference, eventType, reason, message string) v1.Event {
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

func newKubernetesStorageClass(sp *lclient.StoragePool, r int) storagev1.StorageClass {
	volBindMode := storagev1.VolumeBindingImmediate
	allowVolumeExpansion := true
	reclaimPolicy := v1.PersistentVolumeReclaimDelete
	return storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s-r%d", linstorPrefix, sp.StoragePoolName, r),
		},
		Provisioner:          "linstor.csi.linbit.com",
		VolumeBindingMode:    &volBindMode,
		AllowVolumeExpansion: &allowVolumeExpansion,
		ReclaimPolicy:        &reclaimPolicy,
		Parameters: map[string]string{
			"linstor.csi.linbit.com/storagePool":    sp.StoragePoolName,
			"linstor.csi.linbit.com/placementCount": fmt.Sprintf("%d", r),
		},
	}
}
