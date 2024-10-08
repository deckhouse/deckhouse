// Copyright 2024 Flant JSC
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

// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v4.25.2
// source: services.proto

package dhctl

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

const (
	DHCTL_Check_FullMethodName           = "/dhctl.DHCTL/Check"
	DHCTL_Bootstrap_FullMethodName       = "/dhctl.DHCTL/Bootstrap"
	DHCTL_Destroy_FullMethodName         = "/dhctl.DHCTL/Destroy"
	DHCTL_Abort_FullMethodName           = "/dhctl.DHCTL/Abort"
	DHCTL_Converge_FullMethodName        = "/dhctl.DHCTL/Converge"
	DHCTL_CommanderAttach_FullMethodName = "/dhctl.DHCTL/CommanderAttach"
	DHCTL_CommanderDetach_FullMethodName = "/dhctl.DHCTL/CommanderDetach"
)

// DHCTLClient is the client API for DHCTL service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type DHCTLClient interface {
	Check(ctx context.Context, opts ...grpc.CallOption) (DHCTL_CheckClient, error)
	Bootstrap(ctx context.Context, opts ...grpc.CallOption) (DHCTL_BootstrapClient, error)
	Destroy(ctx context.Context, opts ...grpc.CallOption) (DHCTL_DestroyClient, error)
	Abort(ctx context.Context, opts ...grpc.CallOption) (DHCTL_AbortClient, error)
	Converge(ctx context.Context, opts ...grpc.CallOption) (DHCTL_ConvergeClient, error)
	CommanderAttach(ctx context.Context, opts ...grpc.CallOption) (DHCTL_CommanderAttachClient, error)
	CommanderDetach(ctx context.Context, opts ...grpc.CallOption) (DHCTL_CommanderDetachClient, error)
}

type dHCTLClient struct {
	cc grpc.ClientConnInterface
}

func NewDHCTLClient(cc grpc.ClientConnInterface) DHCTLClient {
	return &dHCTLClient{cc}
}

func (c *dHCTLClient) Check(ctx context.Context, opts ...grpc.CallOption) (DHCTL_CheckClient, error) {
	stream, err := c.cc.NewStream(ctx, &DHCTL_ServiceDesc.Streams[0], DHCTL_Check_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &dHCTLCheckClient{stream}
	return x, nil
}

type DHCTL_CheckClient interface {
	Send(*CheckRequest) error
	Recv() (*CheckResponse, error)
	grpc.ClientStream
}

type dHCTLCheckClient struct {
	grpc.ClientStream
}

func (x *dHCTLCheckClient) Send(m *CheckRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *dHCTLCheckClient) Recv() (*CheckResponse, error) {
	m := new(CheckResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *dHCTLClient) Bootstrap(ctx context.Context, opts ...grpc.CallOption) (DHCTL_BootstrapClient, error) {
	stream, err := c.cc.NewStream(ctx, &DHCTL_ServiceDesc.Streams[1], DHCTL_Bootstrap_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &dHCTLBootstrapClient{stream}
	return x, nil
}

type DHCTL_BootstrapClient interface {
	Send(*BootstrapRequest) error
	Recv() (*BootstrapResponse, error)
	grpc.ClientStream
}

type dHCTLBootstrapClient struct {
	grpc.ClientStream
}

func (x *dHCTLBootstrapClient) Send(m *BootstrapRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *dHCTLBootstrapClient) Recv() (*BootstrapResponse, error) {
	m := new(BootstrapResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *dHCTLClient) Destroy(ctx context.Context, opts ...grpc.CallOption) (DHCTL_DestroyClient, error) {
	stream, err := c.cc.NewStream(ctx, &DHCTL_ServiceDesc.Streams[2], DHCTL_Destroy_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &dHCTLDestroyClient{stream}
	return x, nil
}

type DHCTL_DestroyClient interface {
	Send(*DestroyRequest) error
	Recv() (*DestroyResponse, error)
	grpc.ClientStream
}

type dHCTLDestroyClient struct {
	grpc.ClientStream
}

func (x *dHCTLDestroyClient) Send(m *DestroyRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *dHCTLDestroyClient) Recv() (*DestroyResponse, error) {
	m := new(DestroyResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *dHCTLClient) Abort(ctx context.Context, opts ...grpc.CallOption) (DHCTL_AbortClient, error) {
	stream, err := c.cc.NewStream(ctx, &DHCTL_ServiceDesc.Streams[3], DHCTL_Abort_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &dHCTLAbortClient{stream}
	return x, nil
}

type DHCTL_AbortClient interface {
	Send(*AbortRequest) error
	Recv() (*AbortResponse, error)
	grpc.ClientStream
}

type dHCTLAbortClient struct {
	grpc.ClientStream
}

func (x *dHCTLAbortClient) Send(m *AbortRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *dHCTLAbortClient) Recv() (*AbortResponse, error) {
	m := new(AbortResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *dHCTLClient) Converge(ctx context.Context, opts ...grpc.CallOption) (DHCTL_ConvergeClient, error) {
	stream, err := c.cc.NewStream(ctx, &DHCTL_ServiceDesc.Streams[4], DHCTL_Converge_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &dHCTLConvergeClient{stream}
	return x, nil
}

type DHCTL_ConvergeClient interface {
	Send(*ConvergeRequest) error
	Recv() (*ConvergeResponse, error)
	grpc.ClientStream
}

type dHCTLConvergeClient struct {
	grpc.ClientStream
}

func (x *dHCTLConvergeClient) Send(m *ConvergeRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *dHCTLConvergeClient) Recv() (*ConvergeResponse, error) {
	m := new(ConvergeResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *dHCTLClient) CommanderAttach(ctx context.Context, opts ...grpc.CallOption) (DHCTL_CommanderAttachClient, error) {
	stream, err := c.cc.NewStream(ctx, &DHCTL_ServiceDesc.Streams[5], DHCTL_CommanderAttach_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &dHCTLCommanderAttachClient{stream}
	return x, nil
}

type DHCTL_CommanderAttachClient interface {
	Send(*CommanderAttachRequest) error
	Recv() (*CommanderAttachResponse, error)
	grpc.ClientStream
}

type dHCTLCommanderAttachClient struct {
	grpc.ClientStream
}

func (x *dHCTLCommanderAttachClient) Send(m *CommanderAttachRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *dHCTLCommanderAttachClient) Recv() (*CommanderAttachResponse, error) {
	m := new(CommanderAttachResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *dHCTLClient) CommanderDetach(ctx context.Context, opts ...grpc.CallOption) (DHCTL_CommanderDetachClient, error) {
	stream, err := c.cc.NewStream(ctx, &DHCTL_ServiceDesc.Streams[6], DHCTL_CommanderDetach_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &dHCTLCommanderDetachClient{stream}
	return x, nil
}

type DHCTL_CommanderDetachClient interface {
	Send(*CommanderDetachRequest) error
	Recv() (*CommanderDetachResponse, error)
	grpc.ClientStream
}

type dHCTLCommanderDetachClient struct {
	grpc.ClientStream
}

func (x *dHCTLCommanderDetachClient) Send(m *CommanderDetachRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *dHCTLCommanderDetachClient) Recv() (*CommanderDetachResponse, error) {
	m := new(CommanderDetachResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// DHCTLServer is the server API for DHCTL service.
// All implementations must embed UnimplementedDHCTLServer
// for forward compatibility
type DHCTLServer interface {
	Check(DHCTL_CheckServer) error
	Bootstrap(DHCTL_BootstrapServer) error
	Destroy(DHCTL_DestroyServer) error
	Abort(DHCTL_AbortServer) error
	Converge(DHCTL_ConvergeServer) error
	CommanderAttach(DHCTL_CommanderAttachServer) error
	CommanderDetach(DHCTL_CommanderDetachServer) error
	mustEmbedUnimplementedDHCTLServer()
}

// UnimplementedDHCTLServer must be embedded to have forward compatible implementations.
type UnimplementedDHCTLServer struct {
}

func (UnimplementedDHCTLServer) Check(DHCTL_CheckServer) error {
	return status.Errorf(codes.Unimplemented, "method Check not implemented")
}
func (UnimplementedDHCTLServer) Bootstrap(DHCTL_BootstrapServer) error {
	return status.Errorf(codes.Unimplemented, "method Bootstrap not implemented")
}
func (UnimplementedDHCTLServer) Destroy(DHCTL_DestroyServer) error {
	return status.Errorf(codes.Unimplemented, "method Destroy not implemented")
}
func (UnimplementedDHCTLServer) Abort(DHCTL_AbortServer) error {
	return status.Errorf(codes.Unimplemented, "method Abort not implemented")
}
func (UnimplementedDHCTLServer) Converge(DHCTL_ConvergeServer) error {
	return status.Errorf(codes.Unimplemented, "method Converge not implemented")
}
func (UnimplementedDHCTLServer) CommanderAttach(DHCTL_CommanderAttachServer) error {
	return status.Errorf(codes.Unimplemented, "method CommanderAttach not implemented")
}
func (UnimplementedDHCTLServer) CommanderDetach(DHCTL_CommanderDetachServer) error {
	return status.Errorf(codes.Unimplemented, "method CommanderDetach not implemented")
}
func (UnimplementedDHCTLServer) mustEmbedUnimplementedDHCTLServer() {}

// UnsafeDHCTLServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to DHCTLServer will
// result in compilation errors.
type UnsafeDHCTLServer interface {
	mustEmbedUnimplementedDHCTLServer()
}

func RegisterDHCTLServer(s grpc.ServiceRegistrar, srv DHCTLServer) {
	s.RegisterService(&DHCTL_ServiceDesc, srv)
}

func _DHCTL_Check_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(DHCTLServer).Check(&dHCTLCheckServer{stream})
}

type DHCTL_CheckServer interface {
	Send(*CheckResponse) error
	Recv() (*CheckRequest, error)
	grpc.ServerStream
}

type dHCTLCheckServer struct {
	grpc.ServerStream
}

func (x *dHCTLCheckServer) Send(m *CheckResponse) error {
	return x.ServerStream.SendMsg(m)
}

func (x *dHCTLCheckServer) Recv() (*CheckRequest, error) {
	m := new(CheckRequest)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _DHCTL_Bootstrap_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(DHCTLServer).Bootstrap(&dHCTLBootstrapServer{stream})
}

type DHCTL_BootstrapServer interface {
	Send(*BootstrapResponse) error
	Recv() (*BootstrapRequest, error)
	grpc.ServerStream
}

type dHCTLBootstrapServer struct {
	grpc.ServerStream
}

func (x *dHCTLBootstrapServer) Send(m *BootstrapResponse) error {
	return x.ServerStream.SendMsg(m)
}

func (x *dHCTLBootstrapServer) Recv() (*BootstrapRequest, error) {
	m := new(BootstrapRequest)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _DHCTL_Destroy_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(DHCTLServer).Destroy(&dHCTLDestroyServer{stream})
}

type DHCTL_DestroyServer interface {
	Send(*DestroyResponse) error
	Recv() (*DestroyRequest, error)
	grpc.ServerStream
}

type dHCTLDestroyServer struct {
	grpc.ServerStream
}

func (x *dHCTLDestroyServer) Send(m *DestroyResponse) error {
	return x.ServerStream.SendMsg(m)
}

func (x *dHCTLDestroyServer) Recv() (*DestroyRequest, error) {
	m := new(DestroyRequest)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _DHCTL_Abort_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(DHCTLServer).Abort(&dHCTLAbortServer{stream})
}

type DHCTL_AbortServer interface {
	Send(*AbortResponse) error
	Recv() (*AbortRequest, error)
	grpc.ServerStream
}

type dHCTLAbortServer struct {
	grpc.ServerStream
}

func (x *dHCTLAbortServer) Send(m *AbortResponse) error {
	return x.ServerStream.SendMsg(m)
}

func (x *dHCTLAbortServer) Recv() (*AbortRequest, error) {
	m := new(AbortRequest)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _DHCTL_Converge_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(DHCTLServer).Converge(&dHCTLConvergeServer{stream})
}

type DHCTL_ConvergeServer interface {
	Send(*ConvergeResponse) error
	Recv() (*ConvergeRequest, error)
	grpc.ServerStream
}

type dHCTLConvergeServer struct {
	grpc.ServerStream
}

func (x *dHCTLConvergeServer) Send(m *ConvergeResponse) error {
	return x.ServerStream.SendMsg(m)
}

func (x *dHCTLConvergeServer) Recv() (*ConvergeRequest, error) {
	m := new(ConvergeRequest)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _DHCTL_CommanderAttach_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(DHCTLServer).CommanderAttach(&dHCTLCommanderAttachServer{stream})
}

type DHCTL_CommanderAttachServer interface {
	Send(*CommanderAttachResponse) error
	Recv() (*CommanderAttachRequest, error)
	grpc.ServerStream
}

type dHCTLCommanderAttachServer struct {
	grpc.ServerStream
}

func (x *dHCTLCommanderAttachServer) Send(m *CommanderAttachResponse) error {
	return x.ServerStream.SendMsg(m)
}

func (x *dHCTLCommanderAttachServer) Recv() (*CommanderAttachRequest, error) {
	m := new(CommanderAttachRequest)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _DHCTL_CommanderDetach_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(DHCTLServer).CommanderDetach(&dHCTLCommanderDetachServer{stream})
}

type DHCTL_CommanderDetachServer interface {
	Send(*CommanderDetachResponse) error
	Recv() (*CommanderDetachRequest, error)
	grpc.ServerStream
}

type dHCTLCommanderDetachServer struct {
	grpc.ServerStream
}

func (x *dHCTLCommanderDetachServer) Send(m *CommanderDetachResponse) error {
	return x.ServerStream.SendMsg(m)
}

func (x *dHCTLCommanderDetachServer) Recv() (*CommanderDetachRequest, error) {
	m := new(CommanderDetachRequest)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// DHCTL_ServiceDesc is the grpc.ServiceDesc for DHCTL service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var DHCTL_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "dhctl.DHCTL",
	HandlerType: (*DHCTLServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Check",
			Handler:       _DHCTL_Check_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
		{
			StreamName:    "Bootstrap",
			Handler:       _DHCTL_Bootstrap_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
		{
			StreamName:    "Destroy",
			Handler:       _DHCTL_Destroy_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
		{
			StreamName:    "Abort",
			Handler:       _DHCTL_Abort_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
		{
			StreamName:    "Converge",
			Handler:       _DHCTL_Converge_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
		{
			StreamName:    "CommanderAttach",
			Handler:       _DHCTL_CommanderAttach_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
		{
			StreamName:    "CommanderDetach",
			Handler:       _DHCTL_CommanderDetach_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "services.proto",
}

const (
	Validation_ValidateResources_FullMethodName                     = "/dhctl.Validation/ValidateResources"
	Validation_ValidateInitConfig_FullMethodName                    = "/dhctl.Validation/ValidateInitConfig"
	Validation_ValidateClusterConfig_FullMethodName                 = "/dhctl.Validation/ValidateClusterConfig"
	Validation_ValidateStaticClusterConfig_FullMethodName           = "/dhctl.Validation/ValidateStaticClusterConfig"
	Validation_ValidateProviderSpecificClusterConfig_FullMethodName = "/dhctl.Validation/ValidateProviderSpecificClusterConfig"
	Validation_ValidateChanges_FullMethodName                       = "/dhctl.Validation/ValidateChanges"
	Validation_ParseConnectionConfig_FullMethodName                 = "/dhctl.Validation/ParseConnectionConfig"
)

// ValidationClient is the client API for Validation service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ValidationClient interface {
	ValidateResources(ctx context.Context, in *ValidateResourcesRequest, opts ...grpc.CallOption) (*ValidateResourcesResponse, error)
	ValidateInitConfig(ctx context.Context, in *ValidateInitConfigRequest, opts ...grpc.CallOption) (*ValidateInitConfigResponse, error)
	ValidateClusterConfig(ctx context.Context, in *ValidateClusterConfigRequest, opts ...grpc.CallOption) (*ValidateClusterConfigResponse, error)
	ValidateStaticClusterConfig(ctx context.Context, in *ValidateStaticClusterConfigRequest, opts ...grpc.CallOption) (*ValidateStaticClusterConfigResponse, error)
	ValidateProviderSpecificClusterConfig(ctx context.Context, in *ValidateProviderSpecificClusterConfigRequest, opts ...grpc.CallOption) (*ValidateProviderSpecificClusterConfigResponse, error)
	ValidateChanges(ctx context.Context, in *ValidateChangesRequest, opts ...grpc.CallOption) (*ValidateChangesResponse, error)
	ParseConnectionConfig(ctx context.Context, in *ParseConnectionConfigRequest, opts ...grpc.CallOption) (*ParseConnectionConfigResponse, error)
}

type validationClient struct {
	cc grpc.ClientConnInterface
}

func NewValidationClient(cc grpc.ClientConnInterface) ValidationClient {
	return &validationClient{cc}
}

func (c *validationClient) ValidateResources(ctx context.Context, in *ValidateResourcesRequest, opts ...grpc.CallOption) (*ValidateResourcesResponse, error) {
	out := new(ValidateResourcesResponse)
	err := c.cc.Invoke(ctx, Validation_ValidateResources_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *validationClient) ValidateInitConfig(ctx context.Context, in *ValidateInitConfigRequest, opts ...grpc.CallOption) (*ValidateInitConfigResponse, error) {
	out := new(ValidateInitConfigResponse)
	err := c.cc.Invoke(ctx, Validation_ValidateInitConfig_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *validationClient) ValidateClusterConfig(ctx context.Context, in *ValidateClusterConfigRequest, opts ...grpc.CallOption) (*ValidateClusterConfigResponse, error) {
	out := new(ValidateClusterConfigResponse)
	err := c.cc.Invoke(ctx, Validation_ValidateClusterConfig_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *validationClient) ValidateStaticClusterConfig(ctx context.Context, in *ValidateStaticClusterConfigRequest, opts ...grpc.CallOption) (*ValidateStaticClusterConfigResponse, error) {
	out := new(ValidateStaticClusterConfigResponse)
	err := c.cc.Invoke(ctx, Validation_ValidateStaticClusterConfig_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *validationClient) ValidateProviderSpecificClusterConfig(ctx context.Context, in *ValidateProviderSpecificClusterConfigRequest, opts ...grpc.CallOption) (*ValidateProviderSpecificClusterConfigResponse, error) {
	out := new(ValidateProviderSpecificClusterConfigResponse)
	err := c.cc.Invoke(ctx, Validation_ValidateProviderSpecificClusterConfig_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *validationClient) ValidateChanges(ctx context.Context, in *ValidateChangesRequest, opts ...grpc.CallOption) (*ValidateChangesResponse, error) {
	out := new(ValidateChangesResponse)
	err := c.cc.Invoke(ctx, Validation_ValidateChanges_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *validationClient) ParseConnectionConfig(ctx context.Context, in *ParseConnectionConfigRequest, opts ...grpc.CallOption) (*ParseConnectionConfigResponse, error) {
	out := new(ParseConnectionConfigResponse)
	err := c.cc.Invoke(ctx, Validation_ParseConnectionConfig_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ValidationServer is the server API for Validation service.
// All implementations must embed UnimplementedValidationServer
// for forward compatibility
type ValidationServer interface {
	ValidateResources(context.Context, *ValidateResourcesRequest) (*ValidateResourcesResponse, error)
	ValidateInitConfig(context.Context, *ValidateInitConfigRequest) (*ValidateInitConfigResponse, error)
	ValidateClusterConfig(context.Context, *ValidateClusterConfigRequest) (*ValidateClusterConfigResponse, error)
	ValidateStaticClusterConfig(context.Context, *ValidateStaticClusterConfigRequest) (*ValidateStaticClusterConfigResponse, error)
	ValidateProviderSpecificClusterConfig(context.Context, *ValidateProviderSpecificClusterConfigRequest) (*ValidateProviderSpecificClusterConfigResponse, error)
	ValidateChanges(context.Context, *ValidateChangesRequest) (*ValidateChangesResponse, error)
	ParseConnectionConfig(context.Context, *ParseConnectionConfigRequest) (*ParseConnectionConfigResponse, error)
	mustEmbedUnimplementedValidationServer()
}

// UnimplementedValidationServer must be embedded to have forward compatible implementations.
type UnimplementedValidationServer struct {
}

func (UnimplementedValidationServer) ValidateResources(context.Context, *ValidateResourcesRequest) (*ValidateResourcesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ValidateResources not implemented")
}
func (UnimplementedValidationServer) ValidateInitConfig(context.Context, *ValidateInitConfigRequest) (*ValidateInitConfigResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ValidateInitConfig not implemented")
}
func (UnimplementedValidationServer) ValidateClusterConfig(context.Context, *ValidateClusterConfigRequest) (*ValidateClusterConfigResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ValidateClusterConfig not implemented")
}
func (UnimplementedValidationServer) ValidateStaticClusterConfig(context.Context, *ValidateStaticClusterConfigRequest) (*ValidateStaticClusterConfigResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ValidateStaticClusterConfig not implemented")
}
func (UnimplementedValidationServer) ValidateProviderSpecificClusterConfig(context.Context, *ValidateProviderSpecificClusterConfigRequest) (*ValidateProviderSpecificClusterConfigResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ValidateProviderSpecificClusterConfig not implemented")
}
func (UnimplementedValidationServer) ValidateChanges(context.Context, *ValidateChangesRequest) (*ValidateChangesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ValidateChanges not implemented")
}
func (UnimplementedValidationServer) ParseConnectionConfig(context.Context, *ParseConnectionConfigRequest) (*ParseConnectionConfigResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ParseConnectionConfig not implemented")
}
func (UnimplementedValidationServer) mustEmbedUnimplementedValidationServer() {}

// UnsafeValidationServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ValidationServer will
// result in compilation errors.
type UnsafeValidationServer interface {
	mustEmbedUnimplementedValidationServer()
}

func RegisterValidationServer(s grpc.ServiceRegistrar, srv ValidationServer) {
	s.RegisterService(&Validation_ServiceDesc, srv)
}

func _Validation_ValidateResources_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ValidateResourcesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ValidationServer).ValidateResources(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Validation_ValidateResources_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ValidationServer).ValidateResources(ctx, req.(*ValidateResourcesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Validation_ValidateInitConfig_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ValidateInitConfigRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ValidationServer).ValidateInitConfig(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Validation_ValidateInitConfig_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ValidationServer).ValidateInitConfig(ctx, req.(*ValidateInitConfigRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Validation_ValidateClusterConfig_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ValidateClusterConfigRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ValidationServer).ValidateClusterConfig(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Validation_ValidateClusterConfig_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ValidationServer).ValidateClusterConfig(ctx, req.(*ValidateClusterConfigRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Validation_ValidateStaticClusterConfig_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ValidateStaticClusterConfigRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ValidationServer).ValidateStaticClusterConfig(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Validation_ValidateStaticClusterConfig_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ValidationServer).ValidateStaticClusterConfig(ctx, req.(*ValidateStaticClusterConfigRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Validation_ValidateProviderSpecificClusterConfig_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ValidateProviderSpecificClusterConfigRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ValidationServer).ValidateProviderSpecificClusterConfig(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Validation_ValidateProviderSpecificClusterConfig_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ValidationServer).ValidateProviderSpecificClusterConfig(ctx, req.(*ValidateProviderSpecificClusterConfigRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Validation_ValidateChanges_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ValidateChangesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ValidationServer).ValidateChanges(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Validation_ValidateChanges_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ValidationServer).ValidateChanges(ctx, req.(*ValidateChangesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Validation_ParseConnectionConfig_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ParseConnectionConfigRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ValidationServer).ParseConnectionConfig(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Validation_ParseConnectionConfig_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ValidationServer).ParseConnectionConfig(ctx, req.(*ParseConnectionConfigRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Validation_ServiceDesc is the grpc.ServiceDesc for Validation service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Validation_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "dhctl.Validation",
	HandlerType: (*ValidationServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ValidateResources",
			Handler:    _Validation_ValidateResources_Handler,
		},
		{
			MethodName: "ValidateInitConfig",
			Handler:    _Validation_ValidateInitConfig_Handler,
		},
		{
			MethodName: "ValidateClusterConfig",
			Handler:    _Validation_ValidateClusterConfig_Handler,
		},
		{
			MethodName: "ValidateStaticClusterConfig",
			Handler:    _Validation_ValidateStaticClusterConfig_Handler,
		},
		{
			MethodName: "ValidateProviderSpecificClusterConfig",
			Handler:    _Validation_ValidateProviderSpecificClusterConfig_Handler,
		},
		{
			MethodName: "ValidateChanges",
			Handler:    _Validation_ValidateChanges_Handler,
		},
		{
			MethodName: "ParseConnectionConfig",
			Handler:    _Validation_ParseConnectionConfig_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "services.proto",
}

const (
	Status_GetStatus_FullMethodName = "/dhctl.Status/GetStatus"
)

// StatusClient is the client API for Status service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type StatusClient interface {
	GetStatus(ctx context.Context, in *GetStatusRequest, opts ...grpc.CallOption) (*GetStatusResponse, error)
}

type statusClient struct {
	cc grpc.ClientConnInterface
}

func NewStatusClient(cc grpc.ClientConnInterface) StatusClient {
	return &statusClient{cc}
}

func (c *statusClient) GetStatus(ctx context.Context, in *GetStatusRequest, opts ...grpc.CallOption) (*GetStatusResponse, error) {
	out := new(GetStatusResponse)
	err := c.cc.Invoke(ctx, Status_GetStatus_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// StatusServer is the server API for Status service.
// All implementations must embed UnimplementedStatusServer
// for forward compatibility
type StatusServer interface {
	GetStatus(context.Context, *GetStatusRequest) (*GetStatusResponse, error)
	mustEmbedUnimplementedStatusServer()
}

// UnimplementedStatusServer must be embedded to have forward compatible implementations.
type UnimplementedStatusServer struct {
}

func (UnimplementedStatusServer) GetStatus(context.Context, *GetStatusRequest) (*GetStatusResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetStatus not implemented")
}
func (UnimplementedStatusServer) mustEmbedUnimplementedStatusServer() {}

// UnsafeStatusServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to StatusServer will
// result in compilation errors.
type UnsafeStatusServer interface {
	mustEmbedUnimplementedStatusServer()
}

func RegisterStatusServer(s grpc.ServiceRegistrar, srv StatusServer) {
	s.RegisterService(&Status_ServiceDesc, srv)
}

func _Status_GetStatus_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetStatusRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StatusServer).GetStatus(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Status_GetStatus_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StatusServer).GetStatus(ctx, req.(*GetStatusRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Status_ServiceDesc is the grpc.ServiceDesc for Status service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Status_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "dhctl.Status",
	HandlerType: (*StatusServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetStatus",
			Handler:    _Status_GetStatus_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "services.proto",
}
