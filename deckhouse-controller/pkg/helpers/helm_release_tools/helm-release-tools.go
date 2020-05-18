package helm_release_tools

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"k8s.io/api/core/v1"
	jsonserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/kubectl/pkg/scheme"
)

func getDataFromStdin() string {
	info, err := os.Stdin.Stat()
	if err != nil {
		panic(err)
	}

	if info.Mode()&os.ModeNamedPipe == 0 {
		fmt.Println("ERROR: PIPE is empty")
		os.Exit(1)
	}

	reader := bufio.NewReader(os.Stdin)
	var output []rune

	for {
		input, _, err := reader.ReadRune()
		if err != nil && err == io.EOF {
			break
		}
		output = append(output, input)
	}

	var data string

	for j := 0; j < len(output); j++ {
		data = data + string(output[j])
	}

	return data
}

// ReleaseRename ...
func ReleaseRename(newReleseName string) error {
	if len(newReleseName) == 0 {
		return fmt.Errorf("Example usage: kubectl get cm example.v1 -o jsonpath={.data.release} | deckhouse-controller helper helm set-release-name xyz")
	}

	decoded, _ := DecodeRelease(getDataFromStdin())
	decoded.Name = newReleseName
	encoded, _ := EncodeRelease(decoded)

	fmt.Print(encoded)

	return nil
}

func SetReleaseStatusDeployed() error {
	decoder := scheme.Codecs.UniversalDeserializer()

	obj, _, _ := decoder.Decode([]byte(getDataFromStdin()), nil, nil)
	cm := obj.(*v1.ConfigMap)

	release, _ := DecodeRelease(cm.Data["release"])
	cm.ObjectMeta.Labels["STATUS"] = "DEPLOYED"
	// https://github.com/helm/helm/blob/release-2.16/pkg/proto/hapi/release/status.pb.go#L47
	release.Info.Status.Code = 1

	encoded, _ := EncodeRelease(release)

	cm.Data["release"] = encoded

	jsonEncoder := jsonserializer.NewSerializer(jsonserializer.DefaultMetaFactory, scheme.Scheme, scheme.Scheme, true)

	_ = jsonEncoder.Encode(cm, os.Stdout)

	return nil
}

func ReleaseInfo() error {
	decoder := scheme.Codecs.UniversalDeserializer()

	obj, _, _ := decoder.Decode([]byte(getDataFromStdin()), nil, nil)
	cm := obj.(*v1.ConfigMap)

	release, _ := DecodeRelease(cm.Data["release"])
	fmt.Printf("Name: %v\nNamespace: %v\nStatus: %v\n", release.Name, release.Namespace, release.Info.Status.Code)

	return nil
}
