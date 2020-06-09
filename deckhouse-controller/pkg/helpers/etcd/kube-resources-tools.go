package etcd

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"time"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/pkg/transport"

	"k8s.io/api/core/v1"
	"k8s.io/kubectl/pkg/scheme"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/protobuf"
)

func setName(meta *metav1.ObjectMeta, name string) *metav1.ObjectMeta {
	meta.Name = name
	return meta
}

func setNamespace(meta *metav1.ObjectMeta, namespace string) *metav1.ObjectMeta {
	meta.Namespace = namespace
	return meta
}

func MoveService(etcdEndpoint, etcdCaFile, etcdCertFile, etcdKeyFile, etcdServiceNamespace, etcdServiceName, etcdServiceNewNamespace, etcdServiceNewName string) error {

	var tlsConfig *tls.Config
	if len(etcdCertFile) != 0 || len(etcdKeyFile) != 0 || len(etcdCaFile) != 0 {
		tlsInfo := transport.TLSInfo{
			CertFile:      etcdCertFile,
			KeyFile:       etcdKeyFile,
			TrustedCAFile: etcdCaFile,
		}
		var err error
		tlsConfig, err = tlsInfo.ClientConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: unable to create client config: %v\n", err)
			os.Exit(1)
		}
	}

	config := clientv3.Config{
		Endpoints:   []string{etcdEndpoint},
		TLS:         tlsConfig,
		DialTimeout: 5 * time.Second,
	}
	client, err := clientv3.New(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: unable to connect to etcd: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	decoder := scheme.Codecs.UniversalDeserializer()

	key := fmt.Sprintf("/registry/services/specs/%s/%s", etcdServiceNamespace, etcdServiceName)

	resp, _ := clientv3.NewKV(client).Get(context.Background(), key)
	obj, _, _ := decoder.Decode(resp.Kvs[0].Value, nil, nil)
	newKey := ""

	switch o := obj.(type) {
	case *v1.Service:
		setName(&o.ObjectMeta, etcdServiceNewName)
		setNamespace(&o.ObjectMeta, etcdServiceNewNamespace)
		newKey = fmt.Sprintf("/registry/services/specs/%s/%s", etcdServiceNewNamespace, etcdServiceNewName)
	default:
		fmt.Printf("I don't know about type %T!\n", o)
	}

	protoSerializer := protobuf.NewSerializer(scheme.Scheme, scheme.Scheme)
	newObj := new(bytes.Buffer)
	protoSerializer.Encode(obj, newObj)

	_, err = clientv3.NewKV(client).Put(context.Background(), newKey, newObj.String())
	if err == nil {
		_, err = clientv3.NewKV(client).Delete(context.Background(), key)
		if err != nil {
			fmt.Printf("failed to delete key %s %s\n", newKey, err)
		}
	} else {
		fmt.Printf("put to key %s %s\n", newKey, err)
	}

	return nil
}
