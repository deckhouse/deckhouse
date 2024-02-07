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
	"bufio"
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

	lclient "github.com/LINBIT/golinstor/client"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"k8s.io/utils/pointer"
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
	nodeName     string
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
		err := provisionStoragePools(ctx, lc, kc, opts.nodeName, opts.scanInterval)
		if errors.Is(err, context.Canceled) {
			// only occurs if the context was cancelled, and it only can be cancelled on SIGINT
			stop <- struct{}{}
			return
		}
		klog.Fatalln(err)
	}()

	// Blocks waiting signals from OS.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

	<-ch
	cancel()
	<-stop
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

func provisionStoragePools(ctx context.Context, lc *lclient.Client, kc kclient.Client, nodeName string, scanInterval int) error {
	candiCh := make(chan Candidate)
	errCh := make(chan error)
	ticker := time.NewTicker(time.Duration(scanInterval) * time.Second)

	go func() {
		seen := make(map[string]struct{})

		for {
			select {
			case <-ticker.C:
				candidates, err := getCandidates(nodeName)
				if err != nil {
					// only fatal error, cannot cmd
					errCh <- err
					return
				}
				for _, cand := range candidates {
					if _, yes := seen[cand.UUID+"+"+cand.StoragePool.StoragePoolName]; yes {
						continue
					}
					seen[cand.UUID+"+"+cand.StoragePool.StoragePoolName] = struct{}{}
					candiCh <- cand
				}
			case <-ctx.Done():
				return
			}
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

			involvedObject := v1.ObjectReference{
				Kind: "StoragePool",
				Name: cand.StoragePool.NodeName + "." + cand.StoragePool.StoragePoolName,
			}

			changed, err := syncNodeStoragePool(ctx, lc, cand)
			if err != nil {
				// only abortions possible here
				err2 := report(ctx, kc, false, nodeName, involvedObject, "Failed to sync LINSTOR storage pool: "+err.Error())
				if err2 != nil {
					klog.Fatalln("Failed to create event", err2)
				}
				return err
			}

			if changed {
				if err := report(ctx, kc, true, nodeName, involvedObject, "Created LINSTOR storage pool: "+nodeName+"/"+cand.StoragePool.StoragePoolName); err != nil {
					return err
				}
			} else {
				klog.Info("LINSTOR storage pool " + nodeName + "/" + cand.StoragePool.StoragePoolName + " is already configured")
			}

			scs, err := genKubernetesStorageClasses(ctx, lc, cand)
			if err != nil {
				// only abortions possible here
				err2 := report(ctx, kc, false, nodeName, involvedObject, "Failed to generate Kubernetes storage classes: "+err.Error())
				if err2 != nil {
					klog.Fatalln("Failed to create event", err2)
				}
				return err
			}

			for _, sc := range scs {
				involvedObject := v1.ObjectReference{
					APIVersion: "storage.k8s.io/v1",
					Kind:       "StorageClass",
					Name:       sc.GetName(),
				}
				changed, err := syncKubernetesStorageClass(ctx, kc, sc)
				if err != nil {
					// only abortions possible here
					err2 := report(ctx, kc, false, nodeName, involvedObject, "Failed to sync Kubernetes storage class: "+err.Error())
					if err2 != nil {
						klog.Fatalln("Failed to create event", err2)
					}
					return err
				}

				if changed {
					if err := report(ctx, kc, true, nodeName, involvedObject, "Created Kubernetes storage class: "+sc.GetName()); err != nil {
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
	oldSC := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: sc.GetName(),
		},
	}
	// Check old storage class
	err := kc.Get(ctx, types.NamespacedName{Name: sc.GetName()}, oldSC)
	if err != nil && !kerrors.IsNotFound(err) {
		return false, fmt.Errorf("Failed to get Kubernetes storage class: %w", err)
	} else {
		// Found old storage class, check if it is actual
		if allParametersAreSet(&sc, oldSC) {
			return false, nil
		} else {
			// Append old labels and annotations
			appendOldParameters(&sc, oldSC)
			// Delete old storage class
			kc.Delete(ctx, oldSC)
		}
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

// Log and send creation creation event to Kubernetes
func report(ctx context.Context, kc kclient.Client, successful bool, nodeName string, involvedObject v1.ObjectReference, message string) error {
	var eventType, reason string
	if successful {
		eventType = v1.EventTypeNormal
		reason = "Created"
	} else {
		eventType = v1.EventTypeWarning
		reason = "Failed"
	}
	klog.Info(message)
	event := newKubernetesEvent(nodeName, involvedObject, eventType, reason, message)
	return kc.Create(ctx, &event)
}

type Candidate struct {
	Name        string
	UUID        string
	SkipReason  string
	StoragePool lclient.StoragePool
}

type CandidateHandler struct {
	Name       lclient.ProviderKind
	Command    []string
	ParserFunc func(nodeName, out string) ([]Candidate, error)
}

// Collects all storage pool candidates from the node
func getCandidates(nodeName string) ([]Candidate, error) {
	var candidates []Candidate
	var candidateHandlers = []CandidateHandler{
		{
			Name:       lclient.LVM,
			Command:    []string{"vgs", "-oname,uuid,tags", "--separator=;", "--noheadings", "--config=" + lvmConfig},
			ParserFunc: parseLVMVolumeGroups,
		},
		{
			Name:       lclient.LVM_THIN,
			Command:    []string{"lvs", "-oname,vg_name,lv_attr,uuid,tags", "--separator=;", "--noheadings", "--config=" + lvmConfig},
			ParserFunc: parseLVMThinPools,
		},
	}

	for _, handler := range candidateHandlers {
		cmd := exec.Command(handler.Command[0], handler.Command[1:]...)
		var outs, errs bytes.Buffer
		cmd.Stdout = &outs
		cmd.Stderr = &errs
		err := cmd.Run()
		if err != nil {
			return nil, err
		}
		// Getting storage pools
		cs, err := handler.ParserFunc(nodeName, outs.String())
		if err != nil {
			return nil, fmt.Errorf("failed to read %s storage pools: %s. Error was: %s", handler.Name, err, errs.String())
		}
		candidates = append(candidates, cs...)
	}

	return candidates, nil
}

func parseLVMThinPools(nodeName, out string) ([]Candidate, error) {
	var sps []Candidate
	scanner := bufio.NewScanner(strings.NewReader(out))
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		line := scanner.Text()
		var skipReason string
		// Parsing line
		if line == "" {
			continue
		}

		// Example line:
		// "  data;linstor_data;twi---tz--;aDJhKS-fdhT-94VT-MxG8-8WMY-3SwO-2An0gR;linstor-ssd"
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
		name, err := parseNameFromLVMTags(tags)
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
	scanner := bufio.NewScanner(strings.NewReader(out))
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		line := scanner.Text()
		var skipReason string
		// Parsing line
		if line == "" {
			continue
		}

		// Example line:
		// "  linstor_data;BQ5CtV-2arB-FUA8-oynj-XWk2-1pFa-urUSxO;linstor-some-data"
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
		name, err := parseNameFromLVMTags(tags)
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

func parseNameFromLVMTags(tags []string) (string, error) {
	var foundNames []string
	for _, tag := range tags {
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

func newKubernetesEvent(nodeName string, involvedObject v1.ObjectReference, eventType, reason, message string) v1.Event {
	eventTime := metav1.Now()
	event := v1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    v1.NamespaceDefault,
			GenerateName: involvedObject.Name + ".",
			Labels: map[string]string{
				"app": "linstor-pools-importer",
			},
		},
		Reason:         reason,
		Message:        message,
		InvolvedObject: involvedObject,
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
	volBindMode := storagev1.VolumeBindingWaitForFirstConsumer
	reclaimPolicy := v1.PersistentVolumeReclaimDelete
	return storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s-r%d", linstorPrefix, sp.StoragePoolName, r),
			Annotations: map[string]string{
				"cdi.kubevirt.io/clone-strategy": "csi-clone",
			},
		},
		Provisioner:          "linstor.csi.linbit.com",
		VolumeBindingMode:    &volBindMode,
		AllowVolumeExpansion: pointer.Bool(true),
		ReclaimPolicy:        &reclaimPolicy,
		Parameters: map[string]string{
			"linstor.csi.linbit.com/storagePool":                                                 sp.StoragePoolName,
			"linstor.csi.linbit.com/placementCount":                                              fmt.Sprintf("%d", r),
			"property.linstor.csi.linbit.com/DrbdOptions/auto-quorum":                            "suspend-io",
			"property.linstor.csi.linbit.com/DrbdOptions/Resource/on-no-data-accessible":         "suspend-io",
			"property.linstor.csi.linbit.com/DrbdOptions/Resource/on-suspended-primary-outdated": "force-secondary",
			"property.linstor.csi.linbit.com/DrbdOptions/Net/rr-conflict":                        "retry-connect",
		},
	}
}

func allParametersAreSet(sc, oldSC *storagev1.StorageClass) bool {
	if oldSC.VolumeBindingMode != sc.VolumeBindingMode {
		return false
	}
	for k := range sc.Parameters {
		if oldSC.Parameters[k] != sc.Parameters[k] {
			return false
		}
	}
	return true
}

func appendOldParameters(sc, oldSC *storagev1.StorageClass) {
	for k, v := range oldSC.Parameters {
		if _, ok := sc.Parameters[k]; !ok {
			if sc.Parameters == nil {
				sc.Parameters = map[string]string{}
			}
			sc.Parameters[k] = v
		}
	}
	for k, v := range oldSC.Labels {
		if sc.Labels == nil {
			sc.Labels = map[string]string{}
		}
		sc.Labels[k] = v
	}
	for k, v := range oldSC.Annotations {
		if sc.Annotations == nil {
			sc.Annotations = map[string]string{}
		}
		sc.Annotations[k] = v
	}
}
