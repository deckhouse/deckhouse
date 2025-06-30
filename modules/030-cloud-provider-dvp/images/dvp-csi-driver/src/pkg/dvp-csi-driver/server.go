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

package dvpcsidriver

import (
	"context"
	"encoding/json"
	"sync"

	"dvp-csi-driver/internal/endpoint"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
	"google.golang.org/grpc"
	"k8s.io/klog/v2"
)

func NewNonBlockingGRPCServer() *nonBlockingGRPCServer {
	return &nonBlockingGRPCServer{}
}

// NonBlocking server
type nonBlockingGRPCServer struct {
	wg      sync.WaitGroup
	server  *grpc.Server
	cleanup func()
}

func (s *nonBlockingGRPCServer) Start(
	endpoint string,
	ids csi.IdentityServer,
	cs csi.ControllerServer,
	ns csi.NodeServer,
	gcs csi.GroupControllerServer,
) {
	s.wg.Add(1)
	go s.serve(endpoint, ids, cs, ns, gcs)
}

func (s *nonBlockingGRPCServer) Wait() {
	s.wg.Wait()
}

func (s *nonBlockingGRPCServer) Stop() {
	s.server.GracefulStop()
	s.cleanup()
}

func (s *nonBlockingGRPCServer) ForceStop() {
	s.server.Stop()
	s.cleanup()
}

func (s *nonBlockingGRPCServer) serve(
	ep string,
	ids csi.IdentityServer,
	cs csi.ControllerServer,
	ns csi.NodeServer,
	gcs csi.GroupControllerServer,
) {
	listener, cleanup, err := endpoint.Listen(ep)
	if err != nil {
		klog.Fatalf("Failed to listen: %v", err)
	}

	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(logGRPC),
	}
	server := grpc.NewServer(opts...)
	s.server = server
	s.cleanup = cleanup

	if ids != nil {
		csi.RegisterIdentityServer(server, ids)
	}
	if cs != nil {
		csi.RegisterControllerServer(server, cs)
	}
	if ns != nil {
		csi.RegisterNodeServer(server, ns)
	}
	if gcs != nil {
		csi.RegisterGroupControllerServer(server, gcs)
	}

	klog.Infof("Listening for connections on address: %#v", listener.Addr())

	server.Serve(listener)
}

func logGRPC(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	pri := klog.Level(3)
	if info.FullMethod == "/csi.v1.Identity/Probe" {
		// This call occurs frequently, therefore it only gets log at level 5.
		pri = 5
	}
	klog.V(pri).Infof("GRPC call: %s", info.FullMethod)

	v5 := klog.V(5)
	if v5.Enabled() {
		v5.Infof("GRPC request: %s", protosanitizer.StripSecrets(req))
	}
	resp, err := handler(ctx, req)
	if err != nil {
		// Always log errors. Probably not useful though without the method name?!
		klog.Errorf("GRPC error: %v", err)
	}

	if v5.Enabled() {
		v5.Infof("GRPC response: %s", protosanitizer.StripSecrets(resp))

		// In JSON format, intentionally logging without stripping secret
		// fields due to below reasons:
		// - It's technically complicated because protosanitizer.StripSecrets does
		//   not construct new objects, it just wraps the existing ones with a custom
		//   String implementation. Therefore a simple json.Marshal(protosanitizer.StripSecrets(resp))
		//   will still include secrets because it reads fields directly
		//   and more complicated code would be needed.
		// - This is indeed for verification in mock e2e tests. though
		//   currently no test which look at secrets, but we might.
		//   so conceptually it seems better to me to include secrets.
		logGRPCJson(info.FullMethod, req, resp, err)
	}

	return resp, err
}

// logGRPCJson logs the called GRPC call details in JSON format
func logGRPCJson(method string, request, reply interface{}, err error) {
	// Log JSON with the request and response for easier parsing
	logMessage := struct {
		Method   string
		Request  interface{}
		Response interface{}
		// Error as string, for backward compatibility.
		// "" on no error.
		Error string
		// Full error dump, to be able to parse out full gRPC error code and message separately in a test.
		FullError error
	}{
		Method:    method,
		Request:   request,
		Response:  reply,
		FullError: err,
	}

	if err != nil {
		logMessage.Error = err.Error()
	}

	msg, err := json.Marshal(logMessage)
	if err != nil {
		logMessage.Error = err.Error()
	}
	klog.V(5).Infof("gRPCCall: %s\n", msg)
}
