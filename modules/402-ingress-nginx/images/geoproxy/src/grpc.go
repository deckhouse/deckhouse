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

// Implemented GRPC Service for Istio EnvoyFilter
// https://github.com/ekkinox/ext-proc-demo/blob/main/ext-proc/main.go

package geodownloader

import (
	"context"
	"log"
	"net"
	"runtime/debug"

	"github.com/coocood/freecache"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	extProcPb "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	healthPb "google.golang.org/grpc/health/grpc_health_v1"
)

var (
	geoCache    *freecache.Cache
	geoCacheKey = []byte("key")
)

type (
	server struct {
		extProcPb.UnimplementedExternalProcessorServer
	}
	healthServer struct {
		healthPb.UnimplementedHealthServer
	}
)

func (s *healthServer) Check(ctx context.Context, in *healthPb.HealthCheckRequest) (*healthPb.HealthCheckResponse, error) {
	log.Printf("Handling grpc Check request + %s", in.String())
	return &healthPb.HealthCheckResponse{Status: healthPb.HealthCheckResponse_SERVING}, nil
}

func (s *healthServer) Watch(in *healthPb.HealthCheckRequest, srv healthPb.Health_WatchServer) error {
	return status.Error(codes.Unimplemented, "Watch is not implemented")
}

func (s *healthServer) List(ctx context.Context, in *healthPb.HealthListRequest) (*healthPb.HealthListResponse, error) {
	log.Printf("Handling grpc List request + %s", in.String())
	return &healthPb.HealthListResponse{
		Statuses: map[string]*healthPb.HealthCheckResponse{
			"": {Status: healthPb.HealthCheckResponse_SERVING},
		},
	}, nil
}

func (s *server) Process(processServer extProcPb.ExternalProcessor_ProcessServer) error {
	// TODO implement HEADERS serve
	// https://github.com/oschwald/geoip2-golang
	return nil
}

func StartGRPCGeoIPService(servicePosrt string) error {
	// cache init
	geoCache = freecache.NewCache(1024)
	debug.SetGCPercent(20)

	// grpc server init
	lis, err := net.Listen("tcp", servicePosrt)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()

	extProcPb.RegisterExternalProcessorServer(s, &server{})
	healthPb.RegisterHealthServer(s, &healthServer{})

	return s.Serve(lis)
}
