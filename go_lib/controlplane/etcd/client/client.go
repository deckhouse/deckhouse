package client

import clientv3 "go.etcd.io/etcd/client/v3"

func New() (*clientv3.Client, error) {
	// create kube client -> get etcdpodslist

	panic("not implemented")
}

func getPodsList() (PodList, error) {
	// new kubeClient
	// podList()
	panic("not implemented")
}
