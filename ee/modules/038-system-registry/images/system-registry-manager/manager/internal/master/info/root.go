/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package info

import (
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

type Info struct {
	logger        *logrus.Entry
	masterNodes   []corev1.Node
	seaweedfsPods []SeaweedfsNodeInfo
	workers       []WorkerInfo
}

type MergeInfo struct {
	MasterNode   *corev1.Node
	SeaweedfsPod *SeaweedfsNodeInfo
	Worker       *WorkerInfo
}

func NewInfo(logger *logrus.Entry) Info {
	return Info{
		logger:        logger,
		masterNodes:   []corev1.Node{},
		seaweedfsPods: []SeaweedfsNodeInfo{},
		workers:       []WorkerInfo{},
	}
}

func (i *Info) MasterNodesInfoGet() (*corev1.NodeList, error) {
	mInfo := NewMastersInfo(i.logger)
	masterNodes, err := mInfo.MastersInfoGet()
	if masterNodes != nil {
		i.masterNodes = masterNodes.Items
	}
	return masterNodes, err
}

func (i *Info) MasterNodesInfoWaitByFunc(cmpFunc func(nodes *corev1.NodeList) bool) (*corev1.NodeList, error) {
	mInfo := NewMastersInfo(i.logger)
	masterNodes, err := mInfo.MastersInfoWaitByFunc(cmpFunc)
	if masterNodes != nil {
		i.masterNodes = masterNodes.Items
	}
	return masterNodes, err
}

func (i *Info) SeaweedfsPodsInfoGet() ([]SeaweedfsNodeInfo, error) {
	sInfo := NewSeaweedfsInfo(i.logger)
	seaweedfsPod, err := sInfo.SeaweedfsInfoGet()
	if seaweedfsPod != nil {
		i.seaweedfsPods = seaweedfsPod
	}
	return seaweedfsPod, err
}

func (i *Info) SeaweedfsPodsInfoWaitByFunc(cmpFunc func(pods *corev1.PodList) bool) ([]SeaweedfsNodeInfo, error) {
	sInfo := NewSeaweedfsInfo(i.logger)
	seaweedfsPod, err := sInfo.SeaweedfsInfoWaitByFunc(cmpFunc)
	if seaweedfsPod != nil {
		i.seaweedfsPods = seaweedfsPod
	}
	return seaweedfsPod, err
}

func (i *Info) WorkersInfoGet() ([]WorkerInfo, error) {
	wInfo := NewWorkersInfo(i.logger)
	workers, err := wInfo.WorkersGet()
	i.workers = workers
	return workers, err
}

func (i *Info) WorkersInfoWaitAll() ([]WorkerInfo, error) {
	wInfo := NewWorkersInfo(i.logger)
	workers, err := wInfo.WorkersWaitAll()
	i.workers = workers
	return workers, err
}

func (i *Info) AllInfoGet() (map[string]MergeInfo, error) {
	if _, err := i.MasterNodesInfoGet(); err != nil {
		return nil, err
	}
	if _, err := i.SeaweedfsPodsInfoGet(); err != nil {
		return nil, err
	}
	if _, err := i.WorkersInfoGet(); err != nil {
		return nil, err
	}
	return i.mergeByNode(), nil
}

func (i *Info) mergeByNode() map[string]MergeInfo {
	m := map[string]MergeInfo{}

	for _, node := range i.masterNodes {
		if mergeInfo, ok := m[node.Name]; ok {
			mergeInfo.MasterNode = &node
			m[node.Name] = mergeInfo
		} else {
			mergeInfo := MergeInfo{
				MasterNode: &node,
			}
			m[node.Name] = mergeInfo
		}
	}

	for _, seaweedfsPod := range i.seaweedfsPods {
		if mergeInfo, ok := m[seaweedfsPod.Pod.Spec.NodeName]; ok {
			mergeInfo.SeaweedfsPod = &seaweedfsPod
			m[seaweedfsPod.Pod.Spec.NodeName] = mergeInfo
		} else {
			mergeInfo := MergeInfo{
				SeaweedfsPod: &seaweedfsPod,
			}
			m[seaweedfsPod.Pod.Spec.NodeName] = mergeInfo
		}
	}

	for _, worker := range i.workers {
		if worker.NodeName == nil {
			continue
		}

		if mergeInfo, ok := m[*worker.NodeName]; ok {
			mergeInfo.Worker = &worker
			m[*worker.NodeName] = mergeInfo
		} else {
			mergeInfo := MergeInfo{
				Worker: &worker,
			}
			m[*worker.NodeName] = mergeInfo
		}
	}
	return m
}
