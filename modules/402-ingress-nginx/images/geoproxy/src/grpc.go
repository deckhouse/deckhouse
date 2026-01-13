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
// geoip
// https://github.com/oschwald/geoip2-golang

package geodownloader

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/netip"
	"runtime/debug"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extProcPb "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	healthPb "google.golang.org/grpc/health/grpc_health_v1"
	"k8s.io/klog/v2"
)

// var geoCacheKey = []byte("key")

type (
	GRPCServer struct {
		extProcPb.UnimplementedExternalProcessorServer
		geoDB    *GeoDB
		noopMode bool
	}
	healthServer struct {
		healthPb.UnimplementedHealthServer
	}
)

func NewGRPCServer(geoDb *GeoDB, noopMode bool) *GRPCServer {
	return &GRPCServer{
		geoDB:    geoDb,
		noopMode: noopMode,
	}
}

func (s *healthServer) Check(ctx context.Context, in *healthPb.HealthCheckRequest) (*healthPb.HealthCheckResponse, error) {
	klog.V(4).Infof("Handling grpc Check request: %s", in.String())
	return &healthPb.HealthCheckResponse{Status: healthPb.HealthCheckResponse_SERVING}, nil
}

func (s *healthServer) Watch(in *healthPb.HealthCheckRequest, srv healthPb.Health_WatchServer) error {
	return status.Error(codes.Unimplemented, "Watch is not implemented")
}

func (s *healthServer) List(ctx context.Context, in *healthPb.HealthListRequest) (*healthPb.HealthListResponse, error) {
	klog.V(4).Infof("Handling grpc List request: %s", in.String())
	return &healthPb.HealthListResponse{
		Statuses: map[string]*healthPb.HealthCheckResponse{
			"": {Status: healthPb.HealthCheckResponse_SERVING},
		},
	}, nil
}

func (g *GRPCServer) Process(processServer extProcPb.ExternalProcessor_ProcessServer) error {
	ctx := processServer.Context()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		req, err := processServer.Recv()
		if err == io.EOF {
			return nil
		}

		if err != nil {
			return status.Errorf(codes.Unknown, "cannot receive stream request: %v", err)
		}

		klog.V(5).Info("Got ext_proc stream message")

		resp := &extProcPb.ProcessingResponse{}

		switch req.Request.(type) {
		case *extProcPb.ProcessingRequest_RequestHeaders:
			// Always respond to avoid blocking the request in Envoy.
			resp = &extProcPb.ProcessingResponse{
				Response: &extProcPb.ProcessingResponse_RequestHeaders{
					RequestHeaders: &extProcPb.HeadersResponse{
						Response: &extProcPb.CommonResponse{},
					},
				},
			}

			if g.noopMode {
				break // Empty Response for measure ExtProc delay
			}

			rh := req.GetRequestHeaders()
			if rh == nil {
				break
			}

			hdrs := rh.GetHeaders().GetHeaders()
			if klog.V(5).Enabled() {
				klog.V(5).Infof("Request headers: %s", fmt.Sprint(hdrs))
			}

			extAddr, srcHeader, ok := clientIPFromHeaders(hdrs)
			if !ok || extAddr == "" {
				klog.V(5).Info("GeoIP: client ip not found in headers")
				break
			}
			klog.V(5).Infof("GeoIP: client_ip=%q src=%s", extAddr, srcHeader)

			if g.geoDB == nil {
				klog.V(5).Info("GeoIP: GeoDB not initialized")
				break
			}

			setHeaders, cacheHit, err := g.geoDB.GetGeoHeaders(extAddr)
			if err != nil {
				klog.V(4).Infof("GeoIP lookup failed (client_ip=%q, src=%s): %v", extAddr, srcHeader, err)
				break
			}

			if klog.V(5).Enabled() {
				klog.V(5).Infof("GeoIP: set_headers=%s cache=%t", fmt.Sprint(setHeaders), cacheHit)
			}

			if len(setHeaders) > 0 {
				resp = &extProcPb.ProcessingResponse{
					Response: &extProcPb.ProcessingResponse_RequestHeaders{
						RequestHeaders: &extProcPb.HeadersResponse{
							Response: &extProcPb.CommonResponse{
								HeaderMutation: &extProcPb.HeaderMutation{
									SetHeaders: setHeaders,
								},
							},
						},
					},
				}
			}
		}

		if err := processServer.Send(resp); err != nil {
			klog.V(4).Infof("ext_proc send error: %v", err)
		}
	}
}

func (g *GRPCServer) StartGRPCGeoIPService(servicePosrt string) error {
	debug.SetGCPercent(20)

	// grpc server init
	lis, err := net.Listen("tcp", servicePosrt)
	if err != nil {
		klog.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()

	extProcPb.RegisterExternalProcessorServer(s, g)
	healthPb.RegisterHealthServer(s, &healthServer{})

	return s.Serve(lis)
}

func headerValue(hdrs []*corev3.HeaderValue, name string) (string, bool) {
	for _, hv := range hdrs {
		if hv == nil {
			continue
		}
		if !strings.EqualFold(hv.GetKey(), name) {
			continue
		}
		if v := hv.GetValue(); v != "" {
			return v, true
		}
		if raw := hv.GetRawValue(); len(raw) > 0 {
			return string(raw), true
		}
		return "", true
	}
	return "", false
}

func clientIPFromHeaders(hdrs []*corev3.HeaderValue) (ip string, srcHeader string, ok bool) {
	for _, headerName := range []string{
		"x-envoy-external-address",
		"x-forwarded-for",
		"x-original-forwarded-for",
		"x-real-ip",
	} {
		val, found := headerValue(hdrs, headerName)
		if !found {
			continue
		}

		normalized := normalizeIP(val)
		if normalized == "" {
			continue
		}

		ip, err := netip.ParseAddr(normalized)
		if err != nil {
			klog.V(5).Infof("GeoIP: failed to parse client ip from %s=%q: %v", headerName, val, err)
			continue
		}

		return ip.String(), headerName, true
	}

	return "", "", false
}

func normalizeIP(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// XFF: "client, proxy1, proxy2"
	raw = strings.TrimSpace(strings.SplitN(raw, ",", 2)[0])

	// Fast path: plain IP.
	if net.ParseIP(raw) != nil {
		return raw
	}

	// Try host:port.
	if host, _, err := net.SplitHostPort(raw); err == nil {
		if net.ParseIP(host) != nil {
			return host
		}
	}

	return ""
}
