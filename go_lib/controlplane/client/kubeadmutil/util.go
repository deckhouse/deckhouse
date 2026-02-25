package kubeadmutil

import (
	"fmt"
	"sort"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/sets"

	kubeadmapi "github.com/deckhouse/deckhouse/go_lib/controlplane/client/kubeadmapi"
	"github.com/go-errors/errors"

	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	netutils "k8s.io/utils/net"
)

// MergeKubeadmEnvVars merges values of environment variable slices.
// The values defined in later slices overwrite values in previous ones.
func MergeKubeadmEnvVars(envList ...[]kubeadmapi.EnvVar) []v1.EnvVar {
	m := make(map[string]v1.EnvVar)
	merged := []v1.EnvVar{}
	for _, envs := range envList {
		for _, env := range envs {
			m[env.Name] = env.EnvVar
		}
	}
	for _, v := range m {
		merged = append(merged, v)
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Name < merged[j].Name
	})
	return merged
}

// ArgumentsToCommand takes two Arg slices, one with the base arguments and one
// with optional override arguments. In the return list override arguments will precede base
// arguments. If an argument is present in the overrides, it will cause
// all instances of the same argument in the base list to be discarded, leaving
// only the instances of this argument in the overrides to be applied.
func ArgumentsToCommand(base []kubeadmapi.Arg, overrides []kubeadmapi.Arg) []string {
	var command []string
	// Copy the overrides arguments into a new slice.
	args := make([]kubeadmapi.Arg, len(overrides))
	copy(args, overrides)

	// overrideArgs is a set of args which will replace the args defined in the base
	overrideArgs := sets.New[string]()
	for _, arg := range overrides {
		overrideArgs.Insert(arg.Name)
	}

	for _, arg := range base {
		if !overrideArgs.Has(arg.Name) {
			args = append(args, arg)
		}
	}

	sort.Slice(args, func(i, j int) bool {
		if args[i].Name == args[j].Name {
			return args[i].Value < args[j].Value
		}
		return args[i].Name < args[j].Name
	})

	for _, arg := range args {
		command = append(command, fmt.Sprintf("--%s=%s", arg.Name, arg.Value))
	}

	return command
}

// MarshalToYaml marshals an object into yaml.
func MarshalToYaml(obj runtime.Object, gv schema.GroupVersion) ([]byte, error) {
	return MarshalToYamlForCodecs(obj, gv, clientsetscheme.Codecs)
}

// MarshalToYamlForCodecs marshals an object into yaml using the specified codec
// TODO: Is specifying the gv really needed here?
// TODO: Can we support json out of the box easily here?
func MarshalToYamlForCodecs(obj runtime.Object, gv schema.GroupVersion, codecs serializer.CodecFactory) ([]byte, error) {
	const mediaType = runtime.ContentTypeYAML
	info, ok := runtime.SerializerInfoForMediaType(codecs.SupportedMediaTypes(), mediaType)
	if !ok {
		return []byte{}, errors.Errorf("unsupported media type %q", mediaType)
	}

	encoder := codecs.EncoderForVersion(info.Serializer, gv)
	return runtime.Encode(encoder, obj)
}

// ParsePort parses a string representing a TCP port.
// If the string is not a valid representation of a TCP port, ParsePort returns an error.
func ParsePort(port string) (int, error) {
	portInt, err := netutils.ParsePort(port, true)
	if err == nil && (1 <= portInt && portInt <= 65535) {
		return portInt, nil
	}

	return 0, errors.New("port must be a valid number between 1 and 65535, inclusive")
}

// LoadOrDefaultConfigurationOptions holds the common LoadOrDefaultConfiguration options.
type LoadOrDefaultConfigurationOptions struct {
	// AllowExperimental indicates whether the experimental / work in progress APIs can be used.
	AllowExperimental bool
	// SkipCRIDetect indicates whether to skip the CRI socket detection when no CRI socket is provided.
	SkipCRIDetect bool
}
