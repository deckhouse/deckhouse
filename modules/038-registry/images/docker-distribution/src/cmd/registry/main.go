package main

import (
	_ "net/http/pprof"

	"github.com/docker/distribution/registry"
	_ "github.com/docker/distribution/registry/auth/token"
	_ "github.com/docker/distribution/registry/proxy"
	_ "github.com/docker/distribution/registry/storage/driver/filesystem"
	_ "github.com/docker/distribution/registry/storage/driver/inmemory"
	_ "github.com/docker/distribution/registry/storage/driver/middleware/redirect"
)

func main() {
	registry.RootCmd.Execute()
}
