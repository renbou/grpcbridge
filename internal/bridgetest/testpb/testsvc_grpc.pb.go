// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             (unknown)
// source: testsvc.proto

package testpb

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
	TestService_UnaryUnbound_FullMethodName    = "/grpcbridge.internal.bridgetest.testpb.TestService/UnaryUnbound"
	TestService_UnaryBound_FullMethodName      = "/grpcbridge.internal.bridgetest.testpb.TestService/UnaryBound"
	TestService_UnaryCombined_FullMethodName   = "/grpcbridge.internal.bridgetest.testpb.TestService/UnaryCombined"
	TestService_BadResponsePath_FullMethodName = "/grpcbridge.internal.bridgetest.testpb.TestService/BadResponsePath"
	TestService_Echo_FullMethodName            = "/grpcbridge.internal.bridgetest.testpb.TestService/Echo"
	TestService_UnaryFlow_FullMethodName       = "/grpcbridge.internal.bridgetest.testpb.TestService/UnaryFlow"
	TestService_ClientFlow_FullMethodName      = "/grpcbridge.internal.bridgetest.testpb.TestService/ClientFlow"
	TestService_ServerFlow_FullMethodName      = "/grpcbridge.internal.bridgetest.testpb.TestService/ServerFlow"
	TestService_BiDiFlow_FullMethodName        = "/grpcbridge.internal.bridgetest.testpb.TestService/BiDiFlow"
)

// TestServiceClient is the client API for TestService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type TestServiceClient interface {
	UnaryUnbound(ctx context.Context, in *Scalars, opts ...grpc.CallOption) (*Scalars, error)
	UnaryBound(ctx context.Context, in *Scalars, opts ...grpc.CallOption) (*Combined, error)
	UnaryCombined(ctx context.Context, in *Combined, opts ...grpc.CallOption) (*Combined, error)
	BadResponsePath(ctx context.Context, in *Scalars, opts ...grpc.CallOption) (*Combined, error)
	Echo(ctx context.Context, in *Combined, opts ...grpc.CallOption) (*Combined, error)
	// Flow handlers are used to test various streaming flow scenarios.
	UnaryFlow(ctx context.Context, in *FlowMessage, opts ...grpc.CallOption) (*FlowMessage, error)
	ClientFlow(ctx context.Context, opts ...grpc.CallOption) (TestService_ClientFlowClient, error)
	ServerFlow(ctx context.Context, in *FlowMessage, opts ...grpc.CallOption) (TestService_ServerFlowClient, error)
	BiDiFlow(ctx context.Context, opts ...grpc.CallOption) (TestService_BiDiFlowClient, error)
}

type testServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewTestServiceClient(cc grpc.ClientConnInterface) TestServiceClient {
	return &testServiceClient{cc}
}

func (c *testServiceClient) UnaryUnbound(ctx context.Context, in *Scalars, opts ...grpc.CallOption) (*Scalars, error) {
	out := new(Scalars)
	err := c.cc.Invoke(ctx, TestService_UnaryUnbound_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *testServiceClient) UnaryBound(ctx context.Context, in *Scalars, opts ...grpc.CallOption) (*Combined, error) {
	out := new(Combined)
	err := c.cc.Invoke(ctx, TestService_UnaryBound_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *testServiceClient) UnaryCombined(ctx context.Context, in *Combined, opts ...grpc.CallOption) (*Combined, error) {
	out := new(Combined)
	err := c.cc.Invoke(ctx, TestService_UnaryCombined_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *testServiceClient) BadResponsePath(ctx context.Context, in *Scalars, opts ...grpc.CallOption) (*Combined, error) {
	out := new(Combined)
	err := c.cc.Invoke(ctx, TestService_BadResponsePath_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *testServiceClient) Echo(ctx context.Context, in *Combined, opts ...grpc.CallOption) (*Combined, error) {
	out := new(Combined)
	err := c.cc.Invoke(ctx, TestService_Echo_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *testServiceClient) UnaryFlow(ctx context.Context, in *FlowMessage, opts ...grpc.CallOption) (*FlowMessage, error) {
	out := new(FlowMessage)
	err := c.cc.Invoke(ctx, TestService_UnaryFlow_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *testServiceClient) ClientFlow(ctx context.Context, opts ...grpc.CallOption) (TestService_ClientFlowClient, error) {
	stream, err := c.cc.NewStream(ctx, &TestService_ServiceDesc.Streams[0], TestService_ClientFlow_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &testServiceClientFlowClient{stream}
	return x, nil
}

type TestService_ClientFlowClient interface {
	Send(*FlowMessage) error
	CloseAndRecv() (*FlowMessage, error)
	grpc.ClientStream
}

type testServiceClientFlowClient struct {
	grpc.ClientStream
}

func (x *testServiceClientFlowClient) Send(m *FlowMessage) error {
	return x.ClientStream.SendMsg(m)
}

func (x *testServiceClientFlowClient) CloseAndRecv() (*FlowMessage, error) {
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	m := new(FlowMessage)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *testServiceClient) ServerFlow(ctx context.Context, in *FlowMessage, opts ...grpc.CallOption) (TestService_ServerFlowClient, error) {
	stream, err := c.cc.NewStream(ctx, &TestService_ServiceDesc.Streams[1], TestService_ServerFlow_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &testServiceServerFlowClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type TestService_ServerFlowClient interface {
	Recv() (*FlowMessage, error)
	grpc.ClientStream
}

type testServiceServerFlowClient struct {
	grpc.ClientStream
}

func (x *testServiceServerFlowClient) Recv() (*FlowMessage, error) {
	m := new(FlowMessage)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *testServiceClient) BiDiFlow(ctx context.Context, opts ...grpc.CallOption) (TestService_BiDiFlowClient, error) {
	stream, err := c.cc.NewStream(ctx, &TestService_ServiceDesc.Streams[2], TestService_BiDiFlow_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &testServiceBiDiFlowClient{stream}
	return x, nil
}

type TestService_BiDiFlowClient interface {
	Send(*FlowMessage) error
	Recv() (*FlowMessage, error)
	grpc.ClientStream
}

type testServiceBiDiFlowClient struct {
	grpc.ClientStream
}

func (x *testServiceBiDiFlowClient) Send(m *FlowMessage) error {
	return x.ClientStream.SendMsg(m)
}

func (x *testServiceBiDiFlowClient) Recv() (*FlowMessage, error) {
	m := new(FlowMessage)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// TestServiceServer is the server API for TestService service.
// All implementations must embed UnimplementedTestServiceServer
// for forward compatibility
type TestServiceServer interface {
	UnaryUnbound(context.Context, *Scalars) (*Scalars, error)
	UnaryBound(context.Context, *Scalars) (*Combined, error)
	UnaryCombined(context.Context, *Combined) (*Combined, error)
	BadResponsePath(context.Context, *Scalars) (*Combined, error)
	Echo(context.Context, *Combined) (*Combined, error)
	// Flow handlers are used to test various streaming flow scenarios.
	UnaryFlow(context.Context, *FlowMessage) (*FlowMessage, error)
	ClientFlow(TestService_ClientFlowServer) error
	ServerFlow(*FlowMessage, TestService_ServerFlowServer) error
	BiDiFlow(TestService_BiDiFlowServer) error
	mustEmbedUnimplementedTestServiceServer()
}

// UnimplementedTestServiceServer must be embedded to have forward compatible implementations.
type UnimplementedTestServiceServer struct {
}

func (UnimplementedTestServiceServer) UnaryUnbound(context.Context, *Scalars) (*Scalars, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UnaryUnbound not implemented")
}
func (UnimplementedTestServiceServer) UnaryBound(context.Context, *Scalars) (*Combined, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UnaryBound not implemented")
}
func (UnimplementedTestServiceServer) UnaryCombined(context.Context, *Combined) (*Combined, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UnaryCombined not implemented")
}
func (UnimplementedTestServiceServer) BadResponsePath(context.Context, *Scalars) (*Combined, error) {
	return nil, status.Errorf(codes.Unimplemented, "method BadResponsePath not implemented")
}
func (UnimplementedTestServiceServer) Echo(context.Context, *Combined) (*Combined, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Echo not implemented")
}
func (UnimplementedTestServiceServer) UnaryFlow(context.Context, *FlowMessage) (*FlowMessage, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UnaryFlow not implemented")
}
func (UnimplementedTestServiceServer) ClientFlow(TestService_ClientFlowServer) error {
	return status.Errorf(codes.Unimplemented, "method ClientFlow not implemented")
}
func (UnimplementedTestServiceServer) ServerFlow(*FlowMessage, TestService_ServerFlowServer) error {
	return status.Errorf(codes.Unimplemented, "method ServerFlow not implemented")
}
func (UnimplementedTestServiceServer) BiDiFlow(TestService_BiDiFlowServer) error {
	return status.Errorf(codes.Unimplemented, "method BiDiFlow not implemented")
}
func (UnimplementedTestServiceServer) mustEmbedUnimplementedTestServiceServer() {}

// UnsafeTestServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to TestServiceServer will
// result in compilation errors.
type UnsafeTestServiceServer interface {
	mustEmbedUnimplementedTestServiceServer()
}

func RegisterTestServiceServer(s grpc.ServiceRegistrar, srv TestServiceServer) {
	s.RegisterService(&TestService_ServiceDesc, srv)
}

func _TestService_UnaryUnbound_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Scalars)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TestServiceServer).UnaryUnbound(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: TestService_UnaryUnbound_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TestServiceServer).UnaryUnbound(ctx, req.(*Scalars))
	}
	return interceptor(ctx, in, info, handler)
}

func _TestService_UnaryBound_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Scalars)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TestServiceServer).UnaryBound(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: TestService_UnaryBound_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TestServiceServer).UnaryBound(ctx, req.(*Scalars))
	}
	return interceptor(ctx, in, info, handler)
}

func _TestService_UnaryCombined_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Combined)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TestServiceServer).UnaryCombined(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: TestService_UnaryCombined_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TestServiceServer).UnaryCombined(ctx, req.(*Combined))
	}
	return interceptor(ctx, in, info, handler)
}

func _TestService_BadResponsePath_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Scalars)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TestServiceServer).BadResponsePath(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: TestService_BadResponsePath_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TestServiceServer).BadResponsePath(ctx, req.(*Scalars))
	}
	return interceptor(ctx, in, info, handler)
}

func _TestService_Echo_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Combined)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TestServiceServer).Echo(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: TestService_Echo_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TestServiceServer).Echo(ctx, req.(*Combined))
	}
	return interceptor(ctx, in, info, handler)
}

func _TestService_UnaryFlow_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(FlowMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TestServiceServer).UnaryFlow(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: TestService_UnaryFlow_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TestServiceServer).UnaryFlow(ctx, req.(*FlowMessage))
	}
	return interceptor(ctx, in, info, handler)
}

func _TestService_ClientFlow_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(TestServiceServer).ClientFlow(&testServiceClientFlowServer{stream})
}

type TestService_ClientFlowServer interface {
	SendAndClose(*FlowMessage) error
	Recv() (*FlowMessage, error)
	grpc.ServerStream
}

type testServiceClientFlowServer struct {
	grpc.ServerStream
}

func (x *testServiceClientFlowServer) SendAndClose(m *FlowMessage) error {
	return x.ServerStream.SendMsg(m)
}

func (x *testServiceClientFlowServer) Recv() (*FlowMessage, error) {
	m := new(FlowMessage)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _TestService_ServerFlow_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(FlowMessage)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(TestServiceServer).ServerFlow(m, &testServiceServerFlowServer{stream})
}

type TestService_ServerFlowServer interface {
	Send(*FlowMessage) error
	grpc.ServerStream
}

type testServiceServerFlowServer struct {
	grpc.ServerStream
}

func (x *testServiceServerFlowServer) Send(m *FlowMessage) error {
	return x.ServerStream.SendMsg(m)
}

func _TestService_BiDiFlow_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(TestServiceServer).BiDiFlow(&testServiceBiDiFlowServer{stream})
}

type TestService_BiDiFlowServer interface {
	Send(*FlowMessage) error
	Recv() (*FlowMessage, error)
	grpc.ServerStream
}

type testServiceBiDiFlowServer struct {
	grpc.ServerStream
}

func (x *testServiceBiDiFlowServer) Send(m *FlowMessage) error {
	return x.ServerStream.SendMsg(m)
}

func (x *testServiceBiDiFlowServer) Recv() (*FlowMessage, error) {
	m := new(FlowMessage)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// TestService_ServiceDesc is the grpc.ServiceDesc for TestService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var TestService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "grpcbridge.internal.bridgetest.testpb.TestService",
	HandlerType: (*TestServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "UnaryUnbound",
			Handler:    _TestService_UnaryUnbound_Handler,
		},
		{
			MethodName: "UnaryBound",
			Handler:    _TestService_UnaryBound_Handler,
		},
		{
			MethodName: "UnaryCombined",
			Handler:    _TestService_UnaryCombined_Handler,
		},
		{
			MethodName: "BadResponsePath",
			Handler:    _TestService_BadResponsePath_Handler,
		},
		{
			MethodName: "Echo",
			Handler:    _TestService_Echo_Handler,
		},
		{
			MethodName: "UnaryFlow",
			Handler:    _TestService_UnaryFlow_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "ClientFlow",
			Handler:       _TestService_ClientFlow_Handler,
			ClientStreams: true,
		},
		{
			StreamName:    "ServerFlow",
			Handler:       _TestService_ServerFlow_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "BiDiFlow",
			Handler:       _TestService_BiDiFlow_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "testsvc.proto",
}
