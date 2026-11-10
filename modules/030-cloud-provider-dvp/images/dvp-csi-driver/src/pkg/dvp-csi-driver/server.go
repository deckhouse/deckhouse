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
	"errors"
	"reflect"
	"sync"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"k8s.io/klog/v2"

	"dvp-csi-driver/internal/endpoint"
)

const strippedLogValue = "***stripped***"

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

	if err := server.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		klog.Errorf("Failed to serve: %v", err)
	}
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
		v5.Infof("GRPC request: %s", protosanitizer.StripSecrets(sanitizeGRPCLogValue(req)))
	}
	resp, err := handler(ctx, req)
	if err != nil {
		// Always log errors. Probably not useful though without the method name?!
		klog.Errorf("GRPC error: %v", err)
	}

	if v5.Enabled() {
		v5.Infof("GRPC response: %s", protosanitizer.StripSecrets(sanitizeGRPCLogValue(resp)))
		logGRPCJson(info.FullMethod, req, resp, err)
	}

	return resp, err
}

func sanitizeGRPCLogValue(value interface{}) interface{} {
	if value == nil || isNil(value) {
		return value
	}

	msg, ok := value.(proto.Message)
	if !ok {
		return value
	}

	cloned := proto.Clone(msg)
	if cloned == nil {
		return nil
	}

	return sanitizeProtoMessage(cloned.ProtoReflect())
}

func sanitizeProtoMessage(msg protoreflect.Message) map[string]interface{} {
	sanitized := make(map[string]interface{})
	msg.Range(func(field protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		name := field.TextName()
		switch {
		case isCSISecretField(field):
			sanitized[name] = strippedLogValue
		case isCSIMountFlagsField(field):
			sanitized[name] = stripRepeatedString(v.List())
		default:
			sanitized[name] = sanitizeProtoValue(field, v)
		}
		return true
	})
	return sanitized
}

func sanitizeProtoValue(field protoreflect.FieldDescriptor, v protoreflect.Value) interface{} {
	if field.IsList() {
		list := v.List()
		result := make([]interface{}, list.Len())
		for i := 0; i < list.Len(); i++ {
			result[i] = sanitizeProtoSingleValue(field, list.Get(i))
		}
		return result
	}

	if field.IsMap() {
		result := make(map[string]interface{})
		v.Map().Range(func(key protoreflect.MapKey, value protoreflect.Value) bool {
			result[key.String()] = sanitizeProtoSingleValue(field.MapValue(), value)
			return true
		})
		return result
	}

	return sanitizeProtoSingleValue(field, v)
}

func sanitizeProtoSingleValue(field protoreflect.FieldDescriptor, v protoreflect.Value) interface{} {
	switch field.Kind() {
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return sanitizeProtoMessage(v.Message())
	case protoreflect.EnumKind:
		enumValue := field.Enum().Values().ByNumber(v.Enum())
		if enumValue != nil {
			return string(enumValue.Name())
		}
		return int32(v.Enum())
	default:
		return v.Interface()
	}
}

func stripRepeatedString(list protoreflect.List) []string {
	result := make([]string, list.Len())
	for i := 0; i < list.Len(); i++ {
		result[i] = strippedLogValue
	}
	return result
}

func isCSISecretField(field protoreflect.FieldDescriptor) bool {
	value := proto.GetExtension(field.Options(), csi.E_CsiSecret)
	secret, ok := value.(bool)
	return ok && secret
}

func isCSIMountFlagsField(field protoreflect.FieldDescriptor) bool {
	return field.Name() == "mount_flags" && field.ContainingMessage().FullName() == "csi.v1.VolumeCapability.MountVolume"
}

func isNil(value interface{}) bool {
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}

// logGRPCJson logs the called GRPC call details in JSON format
func logGRPCJson(method string, request, reply interface{}, err error) {
	// Log JSON with the request and response for easier parsing
	logMessage := struct {
		Method    string      `json:"method"`
		Request   interface{} `json:"request"`
		Response  interface{} `json:"response"`
		Error     string      `json:"error"`      // Backward compatibility; empty on no error.
		FullError interface{} `json:"full_error"` // Full gRPC error for tests.
	}{
		Method:    method,
		Request:   sanitizeGRPCLogValue(request),
		Response:  sanitizeGRPCLogValue(reply),
		FullError: err,
	}

	if err != nil {
		logMessage.Error = err.Error()
	}

	msg, err := json.Marshal(logMessage)
	if err != nil {
		logMessage.Request = "<<json marshal failed>>"
		logMessage.Response = "<<json marshal failed>>"
		logMessage.Error = err.Error()
		logMessage.FullError = logMessage.Error
		msg, _ = json.Marshal(logMessage)
	}
	klog.V(5).Infof("gRPCCall: %s\n", msg)
}
