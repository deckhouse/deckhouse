/*
Copyright 2026 Flant JSC

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

// Command toy-apiserver is a minimal aggregated API server
// (scopetoy.deckhouse.io/v1 gizmos) for trying scoped reads end to end.
package main

import (
	goflag "flag"
	"os"

	"github.com/spf13/pflag"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/klog/v2"

	"github.com/deckhouse/deckhouse/modules/140-user-authz/docs/internal/scope-aggregated-api-e2e/internal/toy"
)

func main() {
	o := toy.NewOptions()
	fs := pflag.CommandLine
	o.AddFlags(fs)
	klogFlags := goflag.NewFlagSet("klog", goflag.ExitOnError)
	klog.InitFlags(klogFlags)
	fs.AddGoFlagSet(klogFlags)
	pflag.Parse()
	if len(o.Seed) == 0 {
		o.Seed = toy.ParseSeed(os.Getenv("SCOPE_TOY_SEED"))
	}

	ctx := genericapiserver.SetupSignalContext()
	srv, err := o.New()
	if err != nil {
		klog.ErrorS(err, "building the server")
		os.Exit(1)
	}
	klog.InfoS("seeded gizmos", "namespaces", o.Seed)
	if err := srv.Run(ctx); err != nil {
		klog.ErrorS(err, "server stopped")
		os.Exit(1)
	}
}
