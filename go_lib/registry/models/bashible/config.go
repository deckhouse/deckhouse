/*
Copyright 2025 Flant JSC

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

package bashible

import (
	"fmt"
	"slices"

	validation "github.com/go-ozzo/ozzo-validation/v4"
)

var (
	_ validation.Validatable = Config{}
	_ validation.Validatable = ConfigHosts{}
	_ validation.Validatable = ConfigMirrorHost{}
)

// ConfigAgent says that the node agent of the controller-based implementation owns the
// registry configuration of the container runtime.
//
// Its presence is what silences the bashible step that writes per-registry drop-in
// directories: two writers in one directory is the confusion the new implementation
// exists to remove, and the handover has to be a property of the configuration rather
// than of the order the steps happen to run in.
type ConfigAgent struct {
	// Endpoint is where the agent serves the runtime, as "host:port".
	Endpoint string `json:"endpoint" yaml:"endpoint"`

	// DropInFile is the file the agent writes for the container runtime.
	//
	// Carried in the context so that a step which has to wait for the agent waits on the
	// same path the agent writes, rather than on a copy of it spelled out in a template.
	DropInFile string `json:"dropInFile,omitempty" yaml:"dropInFile,omitempty"`

	// Layout is a marshalled RegistryNodeSpec: the routing the agent should use before
	// it has ever reached the API server.
	//
	// Needed because the agent is on the path of every pull on the node, including the
	// pulls that bring up the control plane. A node bootstrapping into a new cluster
	// therefore has to be able to route before there is an API server to ask, and this
	// is the only channel that reaches a node that early.
	//
	// Carried as marshalled JSON rather than a typed field so that it survives every
	// serializer this configuration passes through on its way to the node — it is
	// written to a secret as YAML and read back — and so that a node never has to
	// interpret a schema its own binary does not define.
	Layout string `json:"layout,omitempty" yaml:"layout,omitempty"`
}

type Config struct {
	// Agent, when set, means the node agent owns the runtime's registry
	// configuration and the bashible step must not write it.
	Agent *ConfigAgent `json:"agent,omitempty" yaml:"agent,omitempty"`

	Mode           string                 `json:"mode" yaml:"mode"`
	Version        string                 `json:"version" yaml:"version"`
	ImagesBase     string                 `json:"imagesBase" yaml:"imagesBase"`
	ProxyEndpoints []string               `json:"proxyEndpoints,omitempty" yaml:"proxyEndpoints,omitempty"`
	Hosts          map[string]ConfigHosts `json:"hosts" yaml:"hosts"`
}

type ConfigHosts struct {
	Mirrors []ConfigMirrorHost `json:"mirrors" yaml:"mirrors"`
}

type ConfigMirrorHost struct {
	Host     string          `json:"host" yaml:"host"`
	Scheme   string          `json:"scheme" yaml:"scheme"`
	CA       string          `json:"ca,omitempty" yaml:"ca,omitempty"`
	Auth     ConfigAuth      `json:"auth,omitempty" yaml:"auth,omitempty"`
	Rewrites []ConfigRewrite `json:"rewrites,omitempty" yaml:"rewrites,omitempty"`
}

type ConfigAuth struct {
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
	Auth     string `json:"auth" yaml:"auth"`
}

type ConfigRewrite struct {
	From string `json:"from" yaml:"from"`
	To   string `json:"to" yaml:"to"`
}

func (c Config) Validate() error {
	return validation.ValidateStruct(&c,
		validation.Field(&c.Mode, validation.Required),
		validation.Field(&c.Version, validation.Required),
		validation.Field(&c.ImagesBase, validation.Required),
		validation.Field(&c.ProxyEndpoints, validation.Each(validation.Required)),
		// Hosts key must not be empty
		validation.Field(&c.Hosts, validation.Required),
		// Validate each host
		validation.Field(&c.Hosts, validation.Each(validation.Required)),
	)
}

func (h ConfigHosts) Validate() error {
	if err := validation.ValidateStruct(&h,
		// Mirrors must not be empty
		validation.Field(&h.Mirrors, validation.Required),
		// Validate each mirror
		validation.Field(&h.Mirrors, validation.Each(validation.Required)),
	); err != nil {
		return err
	}

	seen := make(map[string]struct{})
	for i, mirror := range h.Mirrors {
		key := mirror.UniqueKey()
		if _, ok := seen[key]; ok {
			return fmt.Errorf("mirror[%d] validation failed: has duplicate", i)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func (m ConfigMirrorHost) Validate() error {
	return validation.ValidateStruct(&m,
		validation.Field(&m.Host, validation.Required),
		validation.Field(&m.Scheme, validation.Required),
	)
}

func (m ConfigMirrorHost) UniqueKey() string {
	return m.Host + "|" + m.Scheme
}

func (c Config) ToContext() Context {
	ret := Context{
		Agent:          c.Agent.toContext(),
		Mode:           c.Mode,
		Version:        c.Version,
		ImagesBase:     c.ImagesBase,
		ProxyEndpoints: slices.Clone(c.ProxyEndpoints),
		Hosts:          make(map[string]ContextHosts, len(c.Hosts)),
	}

	for key, hosts := range c.Hosts {
		host := ContextHosts{
			Mirrors: make([]ContextMirrorHost, 0, len(hosts.Mirrors)),
		}

		for _, m := range hosts.Mirrors {
			mirror := ContextMirrorHost{
				Host:   m.Host,
				Scheme: m.Scheme,
				CA:     m.CA,
				Auth: ContextAuth{
					Username: m.Auth.Username,
					Password: m.Auth.Password,
					Auth:     m.Auth.Auth,
				},
			}

			for _, rw := range m.Rewrites {
				mirror.Rewrites = append(mirror.Rewrites, ContextRewrite(rw))
			}

			host.Mirrors = append(host.Mirrors, mirror)
		}

		ret.Hosts[key] = host
	}

	return ret
}

func (a *ConfigAgent) toContext() *ContextAgent {
	if a == nil {
		return nil
	}
	return &ContextAgent{Endpoint: a.Endpoint, DropInFile: a.DropInFile, Layout: a.Layout}
}
