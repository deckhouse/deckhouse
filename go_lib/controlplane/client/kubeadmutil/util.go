package kubeadmutil

import (
	"fmt"
	"sort"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/sets"

	errors "github.com/deckhouse/deckhouse/go_lib/controlplane/client/errors"
	kubeadmapi "github.com/deckhouse/deckhouse/go_lib/controlplane/client/kubeadmapi"

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

// // SplitConfigDocuments reads the YAML/JSON bytes per-document, unmarshals the TypeMeta information from each document
// // and returns a map between the GroupVersionKind of the document and the document bytes
// func SplitConfigDocuments(documentBytes []byte) (kubeadmapi.DocumentMap, error) {
// 	gvkmap := kubeadmapi.DocumentMap{}
// 	knownKinds := map[string]bool{}
// 	errs := []error{}
// 	buf := bytes.NewBuffer(documentBytes)
// 	reader := utilyaml.NewYAMLReader(bufio.NewReader(buf))
// 	for {
// 		// Read one YAML document at a time, until io.EOF is returned
// 		b, err := reader.Read()
// 		if err == io.EOF {
// 			break
// 		} else if err != nil {
// 			return nil, err
// 		}
// 		if len(b) == 0 {
// 			break
// 		}
// 		// Deserialize the TypeMeta information of this byte slice
// 		gvk, err := yamlserializer.DefaultMetaFactory.Interpret(b)
// 		if err != nil {
// 			return nil, err
// 		}
// 		if len(gvk.Group) == 0 || len(gvk.Version) == 0 || len(gvk.Kind) == 0 {
// 			return nil, errors.Errorf("invalid configuration for GroupVersionKind %+v: kind and apiVersion is mandatory information that must be specified", gvk)
// 		}

// 		// Check whether the kind has been registered before. If it has, throw an error
// 		if known := knownKinds[gvk.Kind]; known {
// 			errs = append(errs, errors.Errorf("invalid configuration: kind %q is specified twice in YAML file", gvk.Kind))
// 			continue
// 		}
// 		knownKinds[gvk.Kind] = true

// 		// Save the mapping between the gvk and the bytes that object consists of
// 		gvkmap[*gvk] = b
// 	}
// 	if err := errorsutil.NewAggregate(errs); err != nil {
// 		return nil, err
// 	}
// 	return gvkmap, nil
// }

// // DefaultedInitConfiguration takes a versioned init config (often populated by flags), defaults it and converts it into internal InitConfiguration
// func DefaultedInitConfiguration() (*etcdconfig.EtcdConfig, error) {
// 	internalcfg := &etcdconfig.EtcdConfig{}

// 	return internalcfg, nil
// }

// // LoadInitConfigurationFromFile loads a supported versioned InitConfiguration from a file, converts it into internal config, defaults it and verifies it.
// func LoadInitConfigurationFromFile(cfgPath string) (*etcdconfig.EtcdConfig, error) {
// 	klog.V(1).Infof("loading configuration from %q", cfgPath)

// 	b, err := os.ReadFile(cfgPath)
// 	if err != nil {
// 		return nil, errors.Wrapf(err, "unable to read config from %q ", cfgPath)
// 	}

// 	return BytesToInitConfiguration(b)
// }

// // LoadOrDefaultInitConfiguration takes a path to a config file and a versioned configuration that can serve as the default config
// // If cfgPath is specified, the versioned configs will always get overridden with the one in the file (specified by cfgPath).
// // The external, versioned configuration is defaulted and converted to the internal type.
// // Right thereafter, the configuration is defaulted again with dynamic values (like IP addresses of a machine, etc)
// // Lastly, the internal config is validated and returned.
// func LoadOrDefaultInitConfiguration(cfgPath string) (*etcdconfig.EtcdConfig, error) {
// 	var (
// 		config *etcdconfig.EtcdConfig
// 		err    error
// 	)
// 	if cfgPath != "" {
// 		// Loads configuration from config file, if provided
// 		config, err = LoadInitConfigurationFromFile(cfgPath)
// 	} else {
// 		config, err = DefaultedInitConfiguration()
// 	}
// 	if err == nil {
// 		kubeadmapi.SetActiveTimeouts(config.Timeouts)
// 	}
// 	return config, err
// }

// // BytesToInitConfiguration converts a byte slice to an internal, defaulted and validated InitConfiguration object.
// // The map may contain many different YAML/JSON documents. These documents are parsed one-by-one
// // and well-known ComponentConfig GroupVersionKinds are stored inside of the internal InitConfiguration struct.
// // The resulting InitConfiguration is then dynamically defaulted and validated prior to return.
// func BytesToInitConfiguration(b []byte) (*etcdconfig.EtcdConfig, error) {
// 	// Split the YAML/JSON documents in the file into a DocumentMap
// 	gvkmap, err := SplitConfigDocuments(b)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return documentMapToInitConfiguration(gvkmap, false, false, false)
// }

// // validateSupportedVersion checks if the supplied GroupVersion is not on the lists of old unsupported or deprecated GVs.
// // If it is, an error is returned.
// func validateSupportedVersion(gvk schema.GroupVersionKind, allowDeprecated, allowExperimental bool) error {
// 	// The support matrix will look something like this now and in the future:
// 	// v1.10 and earlier: v1alpha1
// 	// v1.11: v1alpha1 read-only, writes only v1alpha2 config
// 	// v1.12: v1alpha2 read-only, writes only v1alpha3 config. Errors if the user tries to use v1alpha1
// 	// v1.13: v1alpha3 read-only, writes only v1beta1 config. Errors if the user tries to use v1alpha1 or v1alpha2
// 	// v1.14: v1alpha3 convert only, writes only v1beta1 config. Errors if the user tries to use v1alpha1 or v1alpha2
// 	// v1.15: v1beta1 read-only, writes only v1beta2 config. Errors if the user tries to use v1alpha1, v1alpha2 or v1alpha3
// 	// v1.22: v1beta2 read-only, writes only v1beta3 config. Errors if the user tries to use v1beta1 and older
// 	// v1.27: only v1beta3 config. Errors if the user tries to use v1beta2 and older
// 	// v1.31: v1beta3 read-only, writes only v1beta4 config, errors if the user tries to use older APIs.
// 	oldKnownAPIVersions := map[string]string{
// 		"kubeadm.k8s.io/v1alpha1": "v1.11",
// 		"kubeadm.k8s.io/v1alpha2": "v1.12",
// 		"kubeadm.k8s.io/v1alpha3": "v1.14",
// 		"kubeadm.k8s.io/v1beta1":  "v1.15",
// 		"kubeadm.k8s.io/v1beta2":  "v1.22",
// 	}

// 	// Experimental API versions are present here until released. Can be used only if allowed.
// 	experimentalAPIVersions := map[string]string{}

// 	// Deprecated API versions are supported until removed. They throw a warning.
// 	deprecatedAPIVersions := map[string]struct{}{
// 		"kubeadm.k8s.io/v1beta3": {},
// 	}

// 	gvString := gvk.GroupVersion().String()

// 	if useKubeadmVersion := oldKnownAPIVersions[gvString]; useKubeadmVersion != "" {
// 		return errors.Errorf("your configuration file uses an old API spec: %q (kind: %q). Please use kubeadm %s instead and run 'kubeadm config migrate --old-config old-config-file --new-config new-config-file', which will write the new, similar spec using a newer API version.", gvString, gvk.Kind, useKubeadmVersion)
// 	}

// 	if _, present := deprecatedAPIVersions[gvString]; present && !allowDeprecated {
// 		klog.Warningf("your configuration file uses a deprecated API spec: %q (kind: %q). Please use 'kubeadm config migrate --old-config old-config-file --new-config new-config-file', which will write the new, similar spec using a newer API version.", gvString, gvk.Kind)
// 	}

// 	if _, present := experimentalAPIVersions[gvString]; present && !allowExperimental {
// 		return errors.Errorf("experimental API spec: %q (kind: %q) is not allowed. You can use the --%s flag if the command supports it.", gvString, gvk.Kind, constants.AllowExperimentalAPI)
// 	}

// 	return nil
// }

// // documentMapToInitConfiguration converts a map of GVKs and YAML/JSON documents to defaulted and validated configuration object.
// func documentMapToInitConfiguration(gvkmap kubeadmapi.DocumentMap, allowDeprecated, allowExperimental, strictErrors bool) (*etcdconfig.EtcdConfig, error) {
// 	var initcfg *etcdconfig.EtcdConfig
// 	// var clustercfg *kubeadmapi.ClusterConfiguration

// 	// Sort the GVKs deterministically by GVK string.
// 	// This allows ClusterConfiguration to be decoded first.
// 	gvks := make([]schema.GroupVersionKind, 0, len(gvkmap))
// 	for gvk := range gvkmap {
// 		gvks = append(gvks, gvk)
// 	}
// 	sort.Slice(gvks, func(i, j int) bool {
// 		return gvks[i].String() < gvks[j].String()
// 	})

// 	for _, gvk := range gvks {
// 		fileContent := gvkmap[gvk]

// 		// first, check if this GVK is supported and possibly not deprecated
// 		if err := validateSupportedVersion(gvk, allowDeprecated, allowExperimental); err != nil {
// 			return nil, err
// 		}

// 		// verify the validity of the JSON/YAML
// 		if err := VerifyUnmarshalStrict([]*runtime.Scheme{kubeadmapi.Scheme componentconfigs.Scheme}, gvk, fileContent); err != nil {
// 			if !strictErrors {
// 				klog.Warning(err.Error())
// 			} else {
// 				return nil, err
// 			}
// 		}

// 		if GroupVersionKindsHasInitConfiguration(gvk) {
// 			// Set initcfg to an empty struct value the deserializer will populate
// 			initcfg = &kubeadmapi.InitConfiguration{}
// 			// Decode the bytes into the internal struct. Under the hood, the bytes will be unmarshalled into the
// 			// right external version, defaulted, and converted into the internal version.
// 			if err := runtime.DecodeInto( kubeadmscheme.Codecs.UniversalDecoder(), fileContent initcfg, nil); err != nil {
// 				return nil, err
// 			}
// 			continue
// 		}
// 		if kubeadmutil.GroupVersionKindsHasClusterConfiguration(gvk) {
// 			// Set clustercfg to an empty struct value the deserializer will populate
// 			clustercfg = &kubeadmapi.ClusterConfiguration{}
// 			// Decode the bytes into the internal struct. Under the hood, the bytes will be unmarshalled into the
// 			// right external version, defaulted, and converted into the internal version.
// 			if err := runtime.DecodeInto(kubeadmscheme.Codecs.UniversalDecoder(), fileContent, clustercfg); err != nil {
// 				return nil, err
// 			}
// 			continue
// 		}

// 		// If the group is neither a kubeadm core type or of a supported component config group, we dump a warning about it being ignored
// 		if !componentconfigs.Scheme.IsGroupRegistered(gvk.Group) {
// 			klog.Warningf("[config] WARNING: Ignored configuration document with GroupVersionKind %v\n", gvk)
// 		}
// 	}

// 	// Enforce that InitConfiguration and/or ClusterConfiguration has to exist among the configuration documents
// 	if initcfg == nil && clustercfg == nil {
// 		return nil, errors.New("no InitConfiguration or ClusterConfiguration kind was found in the configuration file")
// 	}

// 	// If InitConfiguration wasn't given, default it by creating an external struct instance, default it and convert into the internal type
// 	if initcfg == nil {
// 		extinitcfg := &kubeadmapiv1.InitConfiguration{}
// 		kubeadmscheme.Scheme.Default(extinitcfg)
// 		// Set initcfg to an empty struct value the deserializer will populate
// 		initcfg = &kubeadmapi.InitConfiguration{}
// 		if err := kubeadmscheme.Scheme.Convert(extinitcfg, initcfg, nil); err != nil {
// 			return nil, err
// 		}
// 	}
// 	// If ClusterConfiguration was given, populate it in the InitConfiguration struct
// 	if clustercfg != nil {
// 		initcfg.ClusterConfiguration = *clustercfg

// 		// TODO: Workaround for missing v1beta3 ClusterConfiguration timeout conversion. Remove this conversion once the v1beta3 is removed
// 		if clustercfg.APIServer.TimeoutForControlPlane.Duration != 0 && clustercfg.APIServer.TimeoutForControlPlane.Duration != kubeadmconstants.ControlPlaneComponentHealthCheckTimeout {
// 			initcfg.Timeouts.ControlPlaneComponentHealthCheck.Duration = clustercfg.APIServer.TimeoutForControlPlane.Duration
// 		}
// 	} else {
// 		// Populate the internal InitConfiguration.ClusterConfiguration with defaults
// 		extclustercfg := &kubeadmapiv1.ClusterConfiguration{}
// 		kubeadmscheme.Scheme.Default(extclustercfg)
// 		if err := kubeadmscheme.Scheme.Convert(extclustercfg, &initcfg.ClusterConfiguration, nil); err != nil {
// 			return nil, err
// 		}
// 	}

// 	// Load any component configs
// 	if err := componentconfigs.FetchFromDocumentMap(&initcfg.ClusterConfiguration, gvkmap); err != nil {
// 		return nil, err
// 	}

// 	// Applies dynamic defaults to settings not provided with flags
// 	if err := SetInitDynamicDefaults(initcfg, skipCRIDetect); err != nil {
// 		return nil, err
// 	}

// 	// Validates cfg (flags/configs + defaults + dynamic defaults)
// 	if err := validation.ValidateInitConfiguration(initcfg).ToAggregate(); err != nil {
// 		return nil, err
// 	}

// 	return initcfg, nil
// }

// // VerifyUnmarshalStrict takes a slice of schemes, a JSON/YAML byte slice and a GroupVersionKind
// // and verifies if the schema is known and if the byte slice unmarshals with strict mode.
// func VerifyUnmarshalStrict(schemes []*runtime.Scheme, gvk schema.GroupVersionKind, bytes []byte) error {
// 	var scheme *runtime.Scheme
// 	for _, s := range schemes {
// 		if _, err := s.New(gvk); err == nil {
// 			scheme = s
// 			break
// 		}
// 	}
// 	if scheme == nil {
// 		return errors.Errorf("unknown configuration %#v", gvk)
// 	}

// 	opt := json.SerializerOptions{Yaml: true, Pretty: false, Strict: true}
// 	serializer := json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme, scheme, opt)
// 	_, _, err := serializer.Decode(bytes, &gvk, nil)
// 	if err != nil {
// 		return errors.Wrapf(err, "error unmarshaling configuration %#v", gvk)
// 	}

// 	return nil
// }

// // GroupVersionKindsHasKind returns whether the following gvk slice contains the kind given as a parameter
// func GroupVersionKindsHasKind(gvks []schema.GroupVersionKind, kind string) bool {
// 	for _, gvk := range gvks {
// 		if gvk.Kind == kind {
// 			return true
// 		}
// 	}
// 	return false
// }

// // GroupVersionKindsHasInitConfiguration returns whether the following gvk slice contains a InitConfiguration object
// func GroupVersionKindsHasInitConfiguration(gvks ...schema.GroupVersionKind) bool {
// 	return GroupVersionKindsHasKind(gvks, constants.InitConfigurationKind)
// }

// ToClientSet converts a KubeConfig object to a client
// func ToClientSet(config *clientcmdapi.Config) (clientset.Interface, error) {
// 	overrides := clientcmd.ConfigOverrides{Timeout: "10s"}
// 	clientConfig, err := clientcmd.NewDefaultClientConfig(*config, &overrides).ClientConfig()
// 	if err != nil {
// 		return nil, errors.Wrap(err, "failed to create API client configuration from kubeconfig")
// 	}

// 	client, err := clientset.NewForConfig(clientConfig)
// 	if err != nil {
// 		return nil, errors.Wrap(err, "failed to create API client")
// 	}
// 	return client, nil
// }
