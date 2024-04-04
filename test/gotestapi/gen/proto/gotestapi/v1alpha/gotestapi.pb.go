// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.33.0
// 	protoc        (unknown)
// source: proto/gotestapi/v1alpha/gotestapi.proto

package gotestapiv1alpha

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// Veggie declared here to be imported by v1 package.
type Veggie int32

const (
	Veggie_UNKNOWN  Veggie = 0
	Veggie_CARROT   Veggie = 1
	Veggie_TOMATO   Veggie = 2
	Veggie_CUCUMBER Veggie = 3
	Veggie_POTATO   Veggie = 4
)

// Enum value maps for Veggie.
var (
	Veggie_name = map[int32]string{
		0: "UNKNOWN",
		1: "CARROT",
		2: "TOMATO",
		3: "CUCUMBER",
		4: "POTATO",
	}
	Veggie_value = map[string]int32{
		"UNKNOWN":  0,
		"CARROT":   1,
		"TOMATO":   2,
		"CUCUMBER": 3,
		"POTATO":   4,
	}
)

func (x Veggie) Enum() *Veggie {
	p := new(Veggie)
	*p = x
	return p
}

func (x Veggie) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Veggie) Descriptor() protoreflect.EnumDescriptor {
	return file_proto_gotestapi_v1alpha_gotestapi_proto_enumTypes[0].Descriptor()
}

func (Veggie) Type() protoreflect.EnumType {
	return &file_proto_gotestapi_v1alpha_gotestapi_proto_enumTypes[0]
}

func (x Veggie) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Veggie.Descriptor instead.
func (Veggie) EnumDescriptor() ([]byte, []int) {
	return file_proto_gotestapi_v1alpha_gotestapi_proto_rawDescGZIP(), []int{0}
}

type CreatePurchaseRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Veggie   Veggie `protobuf:"varint,1,opt,name=veggie,proto3,enum=gotestapi.v1alpha.Veggie" json:"veggie,omitempty"`
	Quantity int32  `protobuf:"varint,2,opt,name=quantity,proto3" json:"quantity,omitempty"`
}

func (x *CreatePurchaseRequest) Reset() {
	*x = CreatePurchaseRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_gotestapi_v1alpha_gotestapi_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CreatePurchaseRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreatePurchaseRequest) ProtoMessage() {}

func (x *CreatePurchaseRequest) ProtoReflect() protoreflect.Message {
	mi := &file_proto_gotestapi_v1alpha_gotestapi_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreatePurchaseRequest.ProtoReflect.Descriptor instead.
func (*CreatePurchaseRequest) Descriptor() ([]byte, []int) {
	return file_proto_gotestapi_v1alpha_gotestapi_proto_rawDescGZIP(), []int{0}
}

func (x *CreatePurchaseRequest) GetVeggie() Veggie {
	if x != nil {
		return x.Veggie
	}
	return Veggie_UNKNOWN
}

func (x *CreatePurchaseRequest) GetQuantity() int32 {
	if x != nil {
		return x.Quantity
	}
	return 0
}

type CreatePurchaseResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	PurchaseId string `protobuf:"bytes,1,opt,name=purchase_id,json=purchaseId,proto3" json:"purchase_id,omitempty"`
}

func (x *CreatePurchaseResponse) Reset() {
	*x = CreatePurchaseResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_gotestapi_v1alpha_gotestapi_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CreatePurchaseResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreatePurchaseResponse) ProtoMessage() {}

func (x *CreatePurchaseResponse) ProtoReflect() protoreflect.Message {
	mi := &file_proto_gotestapi_v1alpha_gotestapi_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreatePurchaseResponse.ProtoReflect.Descriptor instead.
func (*CreatePurchaseResponse) Descriptor() ([]byte, []int) {
	return file_proto_gotestapi_v1alpha_gotestapi_proto_rawDescGZIP(), []int{1}
}

func (x *CreatePurchaseResponse) GetPurchaseId() string {
	if x != nil {
		return x.PurchaseId
	}
	return ""
}

var File_proto_gotestapi_v1alpha_gotestapi_proto protoreflect.FileDescriptor

var file_proto_gotestapi_v1alpha_gotestapi_proto_rawDesc = []byte{
	0x0a, 0x27, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x67, 0x6f, 0x74, 0x65, 0x73, 0x74, 0x61, 0x70,
	0x69, 0x2f, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x2f, 0x67, 0x6f, 0x74, 0x65, 0x73, 0x74,
	0x61, 0x70, 0x69, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x11, 0x67, 0x6f, 0x74, 0x65, 0x73,
	0x74, 0x61, 0x70, 0x69, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x22, 0x66, 0x0a, 0x15,
	0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x50, 0x75, 0x72, 0x63, 0x68, 0x61, 0x73, 0x65, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x31, 0x0a, 0x06, 0x76, 0x65, 0x67, 0x67, 0x69, 0x65, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x19, 0x2e, 0x67, 0x6f, 0x74, 0x65, 0x73, 0x74, 0x61, 0x70,
	0x69, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x2e, 0x56, 0x65, 0x67, 0x67, 0x69, 0x65,
	0x52, 0x06, 0x76, 0x65, 0x67, 0x67, 0x69, 0x65, 0x12, 0x1a, 0x0a, 0x08, 0x71, 0x75, 0x61, 0x6e,
	0x74, 0x69, 0x74, 0x79, 0x18, 0x02, 0x20, 0x01, 0x28, 0x05, 0x52, 0x08, 0x71, 0x75, 0x61, 0x6e,
	0x74, 0x69, 0x74, 0x79, 0x22, 0x39, 0x0a, 0x16, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x50, 0x75,
	0x72, 0x63, 0x68, 0x61, 0x73, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x1f,
	0x0a, 0x0b, 0x70, 0x75, 0x72, 0x63, 0x68, 0x61, 0x73, 0x65, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x0a, 0x70, 0x75, 0x72, 0x63, 0x68, 0x61, 0x73, 0x65, 0x49, 0x64, 0x2a,
	0x47, 0x0a, 0x06, 0x56, 0x65, 0x67, 0x67, 0x69, 0x65, 0x12, 0x0b, 0x0a, 0x07, 0x55, 0x4e, 0x4b,
	0x4e, 0x4f, 0x57, 0x4e, 0x10, 0x00, 0x12, 0x0a, 0x0a, 0x06, 0x43, 0x41, 0x52, 0x52, 0x4f, 0x54,
	0x10, 0x01, 0x12, 0x0a, 0x0a, 0x06, 0x54, 0x4f, 0x4d, 0x41, 0x54, 0x4f, 0x10, 0x02, 0x12, 0x0c,
	0x0a, 0x08, 0x43, 0x55, 0x43, 0x55, 0x4d, 0x42, 0x45, 0x52, 0x10, 0x03, 0x12, 0x0a, 0x0a, 0x06,
	0x50, 0x4f, 0x54, 0x41, 0x54, 0x4f, 0x10, 0x04, 0x32, 0x73, 0x0a, 0x0a, 0x56, 0x65, 0x67, 0x67,
	0x69, 0x65, 0x53, 0x68, 0x6f, 0x70, 0x12, 0x65, 0x0a, 0x0e, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65,
	0x50, 0x75, 0x72, 0x63, 0x68, 0x61, 0x73, 0x65, 0x12, 0x28, 0x2e, 0x67, 0x6f, 0x74, 0x65, 0x73,
	0x74, 0x61, 0x70, 0x69, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x2e, 0x43, 0x72, 0x65,
	0x61, 0x74, 0x65, 0x50, 0x75, 0x72, 0x63, 0x68, 0x61, 0x73, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x1a, 0x29, 0x2e, 0x67, 0x6f, 0x74, 0x65, 0x73, 0x74, 0x61, 0x70, 0x69, 0x2e, 0x76,
	0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x2e, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x50, 0x75, 0x72,
	0x63, 0x68, 0x61, 0x73, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x42, 0xc4, 0x01,
	0x0a, 0x15, 0x63, 0x6f, 0x6d, 0x2e, 0x67, 0x6f, 0x74, 0x65, 0x73, 0x74, 0x61, 0x70, 0x69, 0x2e,
	0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x42, 0x0e, 0x47, 0x6f, 0x74, 0x65, 0x73, 0x74, 0x61,
	0x70, 0x69, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x36, 0x67, 0x6f, 0x74, 0x65, 0x73,
	0x74, 0x61, 0x70, 0x69, 0x2f, 0x67, 0x65, 0x6e, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x67,
	0x6f, 0x74, 0x65, 0x73, 0x74, 0x61, 0x70, 0x69, 0x2f, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61,
	0x3b, 0x67, 0x6f, 0x74, 0x65, 0x73, 0x74, 0x61, 0x70, 0x69, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68,
	0x61, 0xa2, 0x02, 0x03, 0x47, 0x58, 0x58, 0xaa, 0x02, 0x11, 0x47, 0x6f, 0x74, 0x65, 0x73, 0x74,
	0x61, 0x70, 0x69, 0x2e, 0x56, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0xca, 0x02, 0x11, 0x47, 0x6f,
	0x74, 0x65, 0x73, 0x74, 0x61, 0x70, 0x69, 0x5c, 0x56, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0xe2,
	0x02, 0x1d, 0x47, 0x6f, 0x74, 0x65, 0x73, 0x74, 0x61, 0x70, 0x69, 0x5c, 0x56, 0x31, 0x61, 0x6c,
	0x70, 0x68, 0x61, 0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0xea,
	0x02, 0x12, 0x47, 0x6f, 0x74, 0x65, 0x73, 0x74, 0x61, 0x70, 0x69, 0x3a, 0x3a, 0x56, 0x31, 0x61,
	0x6c, 0x70, 0x68, 0x61, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_proto_gotestapi_v1alpha_gotestapi_proto_rawDescOnce sync.Once
	file_proto_gotestapi_v1alpha_gotestapi_proto_rawDescData = file_proto_gotestapi_v1alpha_gotestapi_proto_rawDesc
)

func file_proto_gotestapi_v1alpha_gotestapi_proto_rawDescGZIP() []byte {
	file_proto_gotestapi_v1alpha_gotestapi_proto_rawDescOnce.Do(func() {
		file_proto_gotestapi_v1alpha_gotestapi_proto_rawDescData = protoimpl.X.CompressGZIP(file_proto_gotestapi_v1alpha_gotestapi_proto_rawDescData)
	})
	return file_proto_gotestapi_v1alpha_gotestapi_proto_rawDescData
}

var file_proto_gotestapi_v1alpha_gotestapi_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_proto_gotestapi_v1alpha_gotestapi_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_proto_gotestapi_v1alpha_gotestapi_proto_goTypes = []interface{}{
	(Veggie)(0),                    // 0: gotestapi.v1alpha.Veggie
	(*CreatePurchaseRequest)(nil),  // 1: gotestapi.v1alpha.CreatePurchaseRequest
	(*CreatePurchaseResponse)(nil), // 2: gotestapi.v1alpha.CreatePurchaseResponse
}
var file_proto_gotestapi_v1alpha_gotestapi_proto_depIdxs = []int32{
	0, // 0: gotestapi.v1alpha.CreatePurchaseRequest.veggie:type_name -> gotestapi.v1alpha.Veggie
	1, // 1: gotestapi.v1alpha.VeggieShop.CreatePurchase:input_type -> gotestapi.v1alpha.CreatePurchaseRequest
	2, // 2: gotestapi.v1alpha.VeggieShop.CreatePurchase:output_type -> gotestapi.v1alpha.CreatePurchaseResponse
	2, // [2:3] is the sub-list for method output_type
	1, // [1:2] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_proto_gotestapi_v1alpha_gotestapi_proto_init() }
func file_proto_gotestapi_v1alpha_gotestapi_proto_init() {
	if File_proto_gotestapi_v1alpha_gotestapi_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_proto_gotestapi_v1alpha_gotestapi_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CreatePurchaseRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_gotestapi_v1alpha_gotestapi_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CreatePurchaseResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_proto_gotestapi_v1alpha_gotestapi_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_proto_gotestapi_v1alpha_gotestapi_proto_goTypes,
		DependencyIndexes: file_proto_gotestapi_v1alpha_gotestapi_proto_depIdxs,
		EnumInfos:         file_proto_gotestapi_v1alpha_gotestapi_proto_enumTypes,
		MessageInfos:      file_proto_gotestapi_v1alpha_gotestapi_proto_msgTypes,
	}.Build()
	File_proto_gotestapi_v1alpha_gotestapi_proto = out.File
	file_proto_gotestapi_v1alpha_gotestapi_proto_rawDesc = nil
	file_proto_gotestapi_v1alpha_gotestapi_proto_goTypes = nil
	file_proto_gotestapi_v1alpha_gotestapi_proto_depIdxs = nil
}
