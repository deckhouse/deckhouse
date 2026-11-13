// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"errors"
	"flag"
	"time"
)

// config is what the flags of the command line give.
type config struct {
	listenAddress, metricsAddress string
	// cacheDir is the cache volume shared with nginx, cacheSize its size.
	cacheDir, cacheSize string
	accessLogAddress    string
	// s3Dir holds the connection of the S3 bucket that keeps the rendered
	// versions.
	s3Dir, s3Size, s3Prefix string
	// The public site: modules come from these registries instead of the
	// Module and ModuleSource objects of a cluster.
	registries, registryAuths string
	// followEvery is how often a follower reads the targets the leader
	// recorded in S3.
	followEvery time.Duration
	// A developer's machine: modules come from the module repositories
	// under localDir, without a cluster or a registry.
	localDir, localVersion string
}

// parseFlags reads the flags of the command line.
func parseFlags() config {
	var c config
	flag.StringVar(&c.listenAddress, "address", ":8081", "Address to listen on")
	flag.StringVar(&c.metricsAddress, "metrics-address", ":9090", "Address to listen on metrics")
	flag.StringVar(&c.cacheDir, "ondemand-cache-dir", "", "Cache volume shared with nginx for module versions rendered on demand (required)")
	flag.StringVar(&c.accessLogAddress, "ondemand-access-log-address", "127.0.0.1:5514", "UDP address to receive the nginx syslog access log of on-demand pages on")
	flag.StringVar(&c.s3Dir, "ondemand-s3-dir", "", "Directory with the connection of the S3 bucket that keeps rendered module versions (connection/: the ConfigMap of the SystemBucket or the Secret of the user) and the STS token (token/) when the connection has no static keys; empty keeps them in the cache volume only")
	flag.StringVar(&c.cacheSize, "ondemand-cache-size", "", "Size of the cache volume given in --ondemand-cache-dir (its sizeLimit), for example 4Gi; the pinned versions may take it less the room of the versions readers open, and a metric reports that budget")
	flag.StringVar(&c.s3Size, "ondemand-s3-size", "", "Size of the S3 bucket given in --ondemand-s3-dir, or of its part under --ondemand-s3-prefix, for example 5Gi; required with --ondemand-s3-dir")
	flag.StringVar(&c.s3Prefix, "ondemand-s3-prefix", "", "Prefix of every key in the S3 bucket, ending with \"/\"; objects outside of it are never read, written or deleted")
	flag.StringVar(&c.registries, "ondemand-registries", "", "Comma-separated module registries (<host>/<path>, whose tags are the modules) of the public site; when set, modules come from them instead of the Module and ModuleSource objects of the cluster, and the first registry that has a module wins")
	flag.StringVar(&c.registryAuths, "ondemand-registry-auths", "", "File with the docker config JSON of --ondemand-registries; empty means anonymous access")
	flag.DurationVar(&c.followEvery, "ondemand-follow-interval", time.Minute, "How often a replica that does not hold the leader Lease reads the versions the leader recorded in S3")
	flag.StringVar(&c.localDir, "ondemand-local-modules-dir", "", "Directory with module repositories (each with module.yaml, docs/, openapi/, crds/), or of directories of them, served as the modules of the public site without a cluster or a registry: a developer's machine. A module is rendered anew once its files change")
	flag.StringVar(&c.localVersion, "ondemand-local-version", "v0.0.0-local", "The version every module of --ondemand-local-modules-dir is served as")
	flag.Parse()

	return c
}

// check refuses the flags that would be ignored without a word, and those
// that do not go together.
func (c config) check() error {
	switch {
	case c.cacheDir == "":
		return errors.New("--ondemand-cache-dir is required")
	case c.registries != "" && c.localDir != "":
		return errors.New("--ondemand-registries does not go with --ondemand-local-modules-dir")
	// Local modules need neither a cluster nor S3, whose leader Lease is a
	// cluster object.
	case c.localDir != "" && c.s3Dir != "":
		return errors.New("--ondemand-local-modules-dir does not go with --ondemand-s3-dir")
	case c.s3Dir == "" && (c.s3Size != "" || c.s3Prefix != ""):
		return errors.New("--ondemand-s3-size and --ondemand-s3-prefix need --ondemand-s3-dir")
	}
	return nil
}
