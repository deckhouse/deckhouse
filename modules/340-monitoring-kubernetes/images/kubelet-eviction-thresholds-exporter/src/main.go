/*
Copyright 2023 Flant JSC

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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/sys/unix"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubelet/config/v1beta1"
)

const (
	nodeFsBytesAvailableEvictionSignal  = "nodefs.available"
	nodeFsInodesFreeEvictionSignal      = "nodefs.inodesFree"
	imageFsBytesAvailableEvictionSignal = "imagefs.available"
	imageFsInodesFreeEvictionSignal     = "imagefs.inodesFree"

	hostPath = "/host"
)

var (
	containerdConfigRootDirRegex = regexp.MustCompile(`^\s*root\s*=\s*(.+)\s*$`)
)

type KubeletConfig struct {
	KubeletConfiguration v1beta1.KubeletConfiguration `json:"kubeletconfig"`
}

func main() {
	err := generateMetrics()
	if err != nil {
		log.Fatal(err)
	}

	// TODO: signal handling
	ticker := time.NewTicker(5 * time.Minute)
	done := make(chan bool)
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			err := generateMetrics()
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}

func generateMetrics() error {
	containerRuntimeVersion, kubeletConfig, err := getContainerRuntimeAndKubeletConfig()
	if err != nil {
		log.Fatal(err)
	}

	runtimeRootDir, err := getRuntimeRootDir(strings.Split(containerRuntimeVersion, ":")[0])
	if err != nil {
		log.Fatal(err)
	}
	kubeletRootDir, err := getKubeletRootDir()
	if err != nil {
		log.Fatal(err)
	}

	nodeFsBytes, nodeFsInodes, err := getBytesAndInodeStatsFromPath(realpath(filepath.Join(hostPath, kubeletRootDir)))
	if err != nil {
		log.Fatal(err)
	}

	imageFsBytes, imageFsInodes, err := getBytesAndInodeStatsFromPath(realpath(filepath.Join(hostPath, runtimeRootDir)))
	if err != nil {
		log.Fatal(err)
	}

	nodefsMountpoint, err := getMountpoint(filepath.Join(hostPath, kubeletRootDir))
	if err != nil {
		log.Printf("Error getting nodefs mountpoint: %s", err)
	}
	imagefsMountpoint, err := getMountpoint(filepath.Join(hostPath, runtimeRootDir))
	if err != nil {
		log.Printf("Error getting imagefs mountpoint: %s", err)
	}

	softEvictionMap := kubeletConfig.KubeletConfiguration.EvictionSoft
	hardEvictionMap := kubeletConfig.KubeletConfiguration.EvictionHard

	fd, err := os.OpenFile("/var/run/node-exporter-textfile/kubelet-eviction.prom", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer func(fd *os.File) {
		_ = fd.Close()
	}(fd)

	evictionHardNodeFsBytesAvailable, err := extractPercent(nodeFsBytes, nodeFsBytesAvailableEvictionSignal, hardEvictionMap)
	if err != nil {
		log.Fatal(err)
	}
	if len(evictionHardNodeFsBytesAvailable) != 0 {
		_, err = fmt.Fprintf(fd, "kubelet_eviction_nodefs_bytes{mountpoint=\"%s\", type=\"hard\"} %s\n", nodefsMountpoint, evictionHardNodeFsBytesAvailable)
		if err != nil {
			return err
		}
	}
	evictionHardNodeFsInodesAvailable, err := extractPercent(nodeFsInodes, nodeFsInodesFreeEvictionSignal, hardEvictionMap)
	if err != nil {
		log.Fatal(err)
	}
	if len(evictionHardNodeFsInodesAvailable) != 0 {
		_, err = fmt.Fprintf(fd, "kubelet_eviction_nodefs_inodes{mountpoint=\"%s\", type=\"hard\"} %s\n", nodefsMountpoint, evictionHardNodeFsInodesAvailable)
		if err != nil {
			return err
		}
	}
	evictionHardImageFsBytesAvailable, err := extractPercent(imageFsBytes, imageFsBytesAvailableEvictionSignal, hardEvictionMap)
	if err != nil {
		log.Fatal(err)
	}
	if len(evictionHardImageFsBytesAvailable) != 0 {
		_, err = fmt.Fprintf(fd, "kubelet_eviction_imagefs_bytes{mountpoint=\"%s\", type=\"hard\"} %s\n", imagefsMountpoint, evictionHardImageFsBytesAvailable)
		if err != nil {
			return err
		}
	}
	evictionHardImagesFsInodesAvailable, err := extractPercent(imageFsInodes, imageFsInodesFreeEvictionSignal, hardEvictionMap)
	if err != nil {
		log.Fatal(err)
	}
	if len(evictionHardImagesFsInodesAvailable) != 0 {
		_, err = fmt.Fprintf(fd, "kubelet_eviction_imagefs_inodes{mountpoint=\"%s\", type=\"hard\"} %s\n", imagefsMountpoint, evictionHardImagesFsInodesAvailable)
		if err != nil {
			return err
		}
	}
	evictionSoftNodeFsBytesAvailable, err := extractPercent(nodeFsBytes, nodeFsBytesAvailableEvictionSignal, softEvictionMap)
	if err != nil {
		log.Fatal(err)
	}
	if len(evictionSoftNodeFsBytesAvailable) != 0 {
		_, err = fmt.Fprintf(fd, "kubelet_eviction_nodefs_bytes{mountpoint=\"%s\", type=\"soft\"} %s\n", nodefsMountpoint, evictionSoftNodeFsBytesAvailable)
		if err != nil {
			return err
		}
	}
	evictionSoftNodeFsInodesAvailable, err := extractPercent(nodeFsInodes, nodeFsInodesFreeEvictionSignal, softEvictionMap)
	if err != nil {
		log.Fatal(err)
	}
	if len(evictionSoftNodeFsInodesAvailable) != 0 {
		_, err = fmt.Fprintf(fd, "kubelet_eviction_nodefs_inodes{mountpoint=\"%s\", type=\"soft\"} %s\n", nodefsMountpoint, evictionSoftNodeFsInodesAvailable)
		if err != nil {
			return err
		}
	}
	evictionSoftImageFsBytesAvailable, err := extractPercent(imageFsBytes, imageFsBytesAvailableEvictionSignal, softEvictionMap)
	if err != nil {
		log.Fatal(err)
	}
	if len(evictionSoftImageFsBytesAvailable) != 0 {
		_, err = fmt.Fprintf(fd, "kubelet_eviction_imagefs_bytes{mountpoint=\"%s\", type=\"soft\"} %s\n", imagefsMountpoint, evictionSoftImageFsBytesAvailable)
		if err != nil {
			return err
		}
	}
	evictionSoftImagesFsInodesAvailable, err := extractPercent(imageFsInodes, imageFsInodesFreeEvictionSignal, softEvictionMap)
	if err != nil {
		log.Fatal(err)
	}
	if len(evictionSoftImagesFsInodesAvailable) != 0 {
		_, err = fmt.Fprintf(fd, "kubelet_eviction_imagefs_inodes{mountpoint=\"%s\", type=\"soft\"} %s\n", imagefsMountpoint, evictionSoftImagesFsInodesAvailable)
		if err != nil {
			return err
		}
	}

	return nil
}

func extractPercent(realResource uint64, signal string, evictionMap map[string]string) (string, error) {
	if evictionMap == nil {
		return "", nil
	}

	evictionSignalValue, ok := evictionMap[signal]
	if !ok {
		return "", nil
	}

	return parseThresholdStatement(realResource, evictionSignalValue)
}

func parseThresholdStatement(realResource uint64, val string) (string, error) {
	if strings.HasSuffix(val, "%") {
		return strings.TrimSuffix(val, "%"), nil
	}

	quantity, err := resource.ParseQuantity(val)
	if err != nil {
		return "", err
	}
	if quantity.Sign() < 0 || quantity.IsZero() {
		return "", fmt.Errorf("eviction threshold must be positive: %s", &quantity)
	}

	intQuantity, _ := quantity.AsInt64()

	ret, _ := new(big.Rat).Quo(new(big.Rat).SetInt64(intQuantity), new(big.Rat).SetUint64(realResource)).Float64()

	return fmt.Sprintf("%.2f", ret*100), nil
}

func getBytesAndInodeStatsFromPath(path string) (bytesTotal uint64, inodeTotal uint64, err error) {
	var stat unix.Statfs_t

	err = unix.Statfs(path, &stat)
	if err != nil {
		return 0, 0, fmt.Errorf("statfs on %s: %w", path, err)
	}

	bytesTotal = stat.Blocks * uint64(stat.Bsize)
	inodeTotal = stat.Files

	return
}

func getKubeletRootDir() (string, error) {
	procs, err := process.Processes()
	if err != nil {
		return "", fmt.Errorf("error getting processes: %s", err)
	}

	for _, p := range procs {
		cmdLine, err := p.CmdlineSlice()
		if err != nil {

			// Skip errors, as they are likely due to the process having terminated
			continue
		}

		if len(cmdLine) == 0 {
			continue
		}

		if !strings.Contains(cmdLine[0], "kubelet") {
			continue
		}

		const rootDirPrefix = "--root-dir="
		for _, arg := range cmdLine {
			if strings.HasPrefix(arg, rootDirPrefix) {
				return strings.TrimPrefix(arg, rootDirPrefix), nil
			}
		}
	}

	return "/var/lib/kubelet", nil
}

func getRuntimeRootDir(runtime string) (string, error) {
	switch runtime {
	case "containerd":
		return getContainerdRootDir()
	}

	return "", fmt.Errorf(`unknown container runtime: "%s". Known containers runtime: "containerd"`, runtime)
}

func getContainerdRootDir() (string, error) {
	containerdConfig, err := os.ReadFile("/etc/containerd/config.toml")
	if err != nil {
		log.Printf("error reading containerd config: %v, using default /var/lib/containerd", err)
		return "/var/lib/containerd", nil
	}

	matches := containerdConfigRootDirRegex.FindSubmatch(containerdConfig)
	if len(matches) != 2 {
		log.Println("containerd config does not contain root dir option, using default /var/lib/containerd")
		return "/var/lib/containerd", nil
	}

	return string(matches[1]), err
}

func getMountpoint(path string) (string, error) {
	if ln, err := os.Readlink(path); err == nil {
		path = ln
	}

	pi, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("failed to get file info for path %s: %w", path, err)
	}

	dev := pi.Sys().(*syscall.Stat_t).Dev

	for path != "/" {
		_path := filepath.Dir(path)

		_pi, err := os.Stat(_path)
		if err != nil {
			return "", fmt.Errorf("failed to get file info for path %s: %w", _path, err)
		}

		if dev != _pi.Sys().(*syscall.Stat_t).Dev {
			break
		}

		path = _path
	}

	if path == hostPath {
		return "/", nil
	}

	return strings.TrimPrefix(path, hostPath), nil
}

func getContainerRuntimeAndKubeletConfig() (string, *KubeletConfig, error) {
	var (
		containerRuntimeVersion string
		kubeletConfig           *KubeletConfig
	)

	myNodeName, ok := os.LookupEnv("MY_NODE_NAME")
	if !ok {
		return "", nil, errors.New("no MY_NODE_NAME env")
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		return "", nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	nodeObj, err := clientset.CoreV1().Nodes().Get(ctx, myNodeName, metav1.GetOptions{})
	if err != nil {
		return "", nil, err
	}

	containerRuntimeVersion = nodeObj.Status.NodeInfo.ContainerRuntimeVersion

	request := clientset.CoreV1().RESTClient().Get().Resource("nodes").Name(myNodeName).SubResource("proxy").Suffix("configz")
	responseBytes, err := request.DoRaw(context.Background())
	if err != nil {
		return "", nil, fmt.Errorf("failed to get config from Node %q: %s", myNodeName, err)
	}

	err = json.Unmarshal(responseBytes, &kubeletConfig)
	if err != nil {
		return "", nil, fmt.Errorf("can't unmarshal kubelet config %s: %s", responseBytes, err)
	}

	return containerRuntimeVersion, kubeletConfig, nil
}

func realpath(path string) string {
	realpath, err := os.Readlink(path)
	if err == nil {
		// path is a symlink
		realpath = filepath.Join(hostPath, realpath)
	} else {
		realpath = path
	}

	return realpath
}
