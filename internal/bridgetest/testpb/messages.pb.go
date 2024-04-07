// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.32.0
// 	protoc        (unknown)
// source: messages.proto

package testpb

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Scalars struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	BoolValue     bool    `protobuf:"varint,1,opt,name=bool_value,json=boolValue,proto3" json:"bool_value,omitempty"`
	Int32Value    int32   `protobuf:"varint,2,opt,name=int32_value,json=int32Value,proto3" json:"int32_value,omitempty"`
	Int64Value    int64   `protobuf:"varint,3,opt,name=int64_value,json=int64Value,proto3" json:"int64_value,omitempty"`
	Uint32Value   uint32  `protobuf:"varint,4,opt,name=uint32_value,json=uint32Value,proto3" json:"uint32_value,omitempty"`
	Uint64Value   uint64  `protobuf:"varint,5,opt,name=uint64_value,json=uint64Value,proto3" json:"uint64_value,omitempty"`
	Sint32Value   int32   `protobuf:"zigzag32,6,opt,name=sint32_value,json=sint32Value,proto3" json:"sint32_value,omitempty"`
	Sint64Value   int64   `protobuf:"zigzag64,7,opt,name=sint64_value,json=sint64Value,proto3" json:"sint64_value,omitempty"`
	Fixed32Value  uint32  `protobuf:"fixed32,8,opt,name=fixed32_value,json=fixed32Value,proto3" json:"fixed32_value,omitempty"`
	Fixed64Value  uint64  `protobuf:"fixed64,9,opt,name=fixed64_value,json=fixed64Value,proto3" json:"fixed64_value,omitempty"`
	Sfixed32Value int32   `protobuf:"fixed32,10,opt,name=sfixed32_value,json=sfixed32Value,proto3" json:"sfixed32_value,omitempty"`
	Sfixed64Value int64   `protobuf:"fixed64,11,opt,name=sfixed64_value,json=sfixed64Value,proto3" json:"sfixed64_value,omitempty"`
	FloatValue    float32 `protobuf:"fixed32,12,opt,name=float_value,json=floatValue,proto3" json:"float_value,omitempty"`
	DoubleValue   float64 `protobuf:"fixed64,13,opt,name=double_value,json=doubleValue,proto3" json:"double_value,omitempty"`
	StringValue   string  `protobuf:"bytes,14,opt,name=string_value,json=stringValue,proto3" json:"string_value,omitempty"`
	BytesValue    []byte  `protobuf:"bytes,15,opt,name=bytes_value,json=bytesValue,proto3" json:"bytes_value,omitempty"`
}

func (x *Scalars) Reset() {
	*x = Scalars{}
	if protoimpl.UnsafeEnabled {
		mi := &file_messages_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Scalars) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Scalars) ProtoMessage() {}

func (x *Scalars) ProtoReflect() protoreflect.Message {
	mi := &file_messages_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Scalars.ProtoReflect.Descriptor instead.
func (*Scalars) Descriptor() ([]byte, []int) {
	return file_messages_proto_rawDescGZIP(), []int{0}
}

func (x *Scalars) GetBoolValue() bool {
	if x != nil {
		return x.BoolValue
	}
	return false
}

func (x *Scalars) GetInt32Value() int32 {
	if x != nil {
		return x.Int32Value
	}
	return 0
}

func (x *Scalars) GetInt64Value() int64 {
	if x != nil {
		return x.Int64Value
	}
	return 0
}

func (x *Scalars) GetUint32Value() uint32 {
	if x != nil {
		return x.Uint32Value
	}
	return 0
}

func (x *Scalars) GetUint64Value() uint64 {
	if x != nil {
		return x.Uint64Value
	}
	return 0
}

func (x *Scalars) GetSint32Value() int32 {
	if x != nil {
		return x.Sint32Value
	}
	return 0
}

func (x *Scalars) GetSint64Value() int64 {
	if x != nil {
		return x.Sint64Value
	}
	return 0
}

func (x *Scalars) GetFixed32Value() uint32 {
	if x != nil {
		return x.Fixed32Value
	}
	return 0
}

func (x *Scalars) GetFixed64Value() uint64 {
	if x != nil {
		return x.Fixed64Value
	}
	return 0
}

func (x *Scalars) GetSfixed32Value() int32 {
	if x != nil {
		return x.Sfixed32Value
	}
	return 0
}

func (x *Scalars) GetSfixed64Value() int64 {
	if x != nil {
		return x.Sfixed64Value
	}
	return 0
}

func (x *Scalars) GetFloatValue() float32 {
	if x != nil {
		return x.FloatValue
	}
	return 0
}

func (x *Scalars) GetDoubleValue() float64 {
	if x != nil {
		return x.DoubleValue
	}
	return 0
}

func (x *Scalars) GetStringValue() string {
	if x != nil {
		return x.StringValue
	}
	return ""
}

func (x *Scalars) GetBytesValue() []byte {
	if x != nil {
		return x.BytesValue
	}
	return nil
}

type NonScalars struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Str2StrMap   map[string]string        `protobuf:"bytes,1,rep,name=str2str_map,json=str2strMap,proto3" json:"str2str_map,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	Str2Int32Map map[string]int32         `protobuf:"bytes,2,rep,name=str2int32_map,json=str2int32Map,proto3" json:"str2int32_map,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"varint,2,opt,name=value,proto3"`
	Int2EmptyMap map[int32]*emptypb.Empty `protobuf:"bytes,3,rep,name=int2empty_map,json=int2emptyMap,proto3" json:"int2empty_map,omitempty" protobuf_key:"varint,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
}

func (x *NonScalars) Reset() {
	*x = NonScalars{}
	if protoimpl.UnsafeEnabled {
		mi := &file_messages_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *NonScalars) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NonScalars) ProtoMessage() {}

func (x *NonScalars) ProtoReflect() protoreflect.Message {
	mi := &file_messages_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NonScalars.ProtoReflect.Descriptor instead.
func (*NonScalars) Descriptor() ([]byte, []int) {
	return file_messages_proto_rawDescGZIP(), []int{1}
}

func (x *NonScalars) GetStr2StrMap() map[string]string {
	if x != nil {
		return x.Str2StrMap
	}
	return nil
}

func (x *NonScalars) GetStr2Int32Map() map[string]int32 {
	if x != nil {
		return x.Str2Int32Map
	}
	return nil
}

func (x *NonScalars) GetInt2EmptyMap() map[int32]*emptypb.Empty {
	if x != nil {
		return x.Int2EmptyMap
	}
	return nil
}

var File_messages_proto protoreflect.FileDescriptor

var file_messages_proto_rawDesc = []byte{
	0x0a, 0x0e, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x12, 0x25, 0x67, 0x72, 0x70, 0x63, 0x62, 0x72, 0x69, 0x64, 0x67, 0x65, 0x2e, 0x69, 0x6e, 0x74,
	0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2e, 0x62, 0x72, 0x69, 0x64, 0x67, 0x65, 0x74, 0x65, 0x73, 0x74,
	0x2e, 0x74, 0x65, 0x73, 0x74, 0x70, 0x62, 0x1a, 0x1b, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x65, 0x6d, 0x70, 0x74, 0x79, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x22, 0x96, 0x04, 0x0a, 0x07, 0x53, 0x63, 0x61, 0x6c, 0x61, 0x72, 0x73,
	0x12, 0x1d, 0x0a, 0x0a, 0x62, 0x6f, 0x6f, 0x6c, 0x5f, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x08, 0x52, 0x09, 0x62, 0x6f, 0x6f, 0x6c, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x12,
	0x1f, 0x0a, 0x0b, 0x69, 0x6e, 0x74, 0x33, 0x32, 0x5f, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x05, 0x52, 0x0a, 0x69, 0x6e, 0x74, 0x33, 0x32, 0x56, 0x61, 0x6c, 0x75, 0x65,
	0x12, 0x1f, 0x0a, 0x0b, 0x69, 0x6e, 0x74, 0x36, 0x34, 0x5f, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x03, 0x52, 0x0a, 0x69, 0x6e, 0x74, 0x36, 0x34, 0x56, 0x61, 0x6c, 0x75,
	0x65, 0x12, 0x21, 0x0a, 0x0c, 0x75, 0x69, 0x6e, 0x74, 0x33, 0x32, 0x5f, 0x76, 0x61, 0x6c, 0x75,
	0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x0b, 0x75, 0x69, 0x6e, 0x74, 0x33, 0x32, 0x56,
	0x61, 0x6c, 0x75, 0x65, 0x12, 0x21, 0x0a, 0x0c, 0x75, 0x69, 0x6e, 0x74, 0x36, 0x34, 0x5f, 0x76,
	0x61, 0x6c, 0x75, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28, 0x04, 0x52, 0x0b, 0x75, 0x69, 0x6e, 0x74,
	0x36, 0x34, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x21, 0x0a, 0x0c, 0x73, 0x69, 0x6e, 0x74, 0x33,
	0x32, 0x5f, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x06, 0x20, 0x01, 0x28, 0x11, 0x52, 0x0b, 0x73,
	0x69, 0x6e, 0x74, 0x33, 0x32, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x21, 0x0a, 0x0c, 0x73, 0x69,
	0x6e, 0x74, 0x36, 0x34, 0x5f, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x07, 0x20, 0x01, 0x28, 0x12,
	0x52, 0x0b, 0x73, 0x69, 0x6e, 0x74, 0x36, 0x34, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x23, 0x0a,
	0x0d, 0x66, 0x69, 0x78, 0x65, 0x64, 0x33, 0x32, 0x5f, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x08,
	0x20, 0x01, 0x28, 0x07, 0x52, 0x0c, 0x66, 0x69, 0x78, 0x65, 0x64, 0x33, 0x32, 0x56, 0x61, 0x6c,
	0x75, 0x65, 0x12, 0x23, 0x0a, 0x0d, 0x66, 0x69, 0x78, 0x65, 0x64, 0x36, 0x34, 0x5f, 0x76, 0x61,
	0x6c, 0x75, 0x65, 0x18, 0x09, 0x20, 0x01, 0x28, 0x06, 0x52, 0x0c, 0x66, 0x69, 0x78, 0x65, 0x64,
	0x36, 0x34, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x25, 0x0a, 0x0e, 0x73, 0x66, 0x69, 0x78, 0x65,
	0x64, 0x33, 0x32, 0x5f, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x0f, 0x52,
	0x0d, 0x73, 0x66, 0x69, 0x78, 0x65, 0x64, 0x33, 0x32, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x25,
	0x0a, 0x0e, 0x73, 0x66, 0x69, 0x78, 0x65, 0x64, 0x36, 0x34, 0x5f, 0x76, 0x61, 0x6c, 0x75, 0x65,
	0x18, 0x0b, 0x20, 0x01, 0x28, 0x10, 0x52, 0x0d, 0x73, 0x66, 0x69, 0x78, 0x65, 0x64, 0x36, 0x34,
	0x56, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x1f, 0x0a, 0x0b, 0x66, 0x6c, 0x6f, 0x61, 0x74, 0x5f, 0x76,
	0x61, 0x6c, 0x75, 0x65, 0x18, 0x0c, 0x20, 0x01, 0x28, 0x02, 0x52, 0x0a, 0x66, 0x6c, 0x6f, 0x61,
	0x74, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x21, 0x0a, 0x0c, 0x64, 0x6f, 0x75, 0x62, 0x6c, 0x65,
	0x5f, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x0d, 0x20, 0x01, 0x28, 0x01, 0x52, 0x0b, 0x64, 0x6f,
	0x75, 0x62, 0x6c, 0x65, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x21, 0x0a, 0x0c, 0x73, 0x74, 0x72,
	0x69, 0x6e, 0x67, 0x5f, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x0e, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x0b, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x1f, 0x0a, 0x0b,
	0x62, 0x79, 0x74, 0x65, 0x73, 0x5f, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x0f, 0x20, 0x01, 0x28,
	0x0c, 0x52, 0x0a, 0x62, 0x79, 0x74, 0x65, 0x73, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x22, 0x9d, 0x04,
	0x0a, 0x0a, 0x4e, 0x6f, 0x6e, 0x53, 0x63, 0x61, 0x6c, 0x61, 0x72, 0x73, 0x12, 0x62, 0x0a, 0x0b,
	0x73, 0x74, 0x72, 0x32, 0x73, 0x74, 0x72, 0x5f, 0x6d, 0x61, 0x70, 0x18, 0x01, 0x20, 0x03, 0x28,
	0x0b, 0x32, 0x41, 0x2e, 0x67, 0x72, 0x70, 0x63, 0x62, 0x72, 0x69, 0x64, 0x67, 0x65, 0x2e, 0x69,
	0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2e, 0x62, 0x72, 0x69, 0x64, 0x67, 0x65, 0x74, 0x65,
	0x73, 0x74, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x70, 0x62, 0x2e, 0x4e, 0x6f, 0x6e, 0x53, 0x63, 0x61,
	0x6c, 0x61, 0x72, 0x73, 0x2e, 0x53, 0x74, 0x72, 0x32, 0x73, 0x74, 0x72, 0x4d, 0x61, 0x70, 0x45,
	0x6e, 0x74, 0x72, 0x79, 0x52, 0x0a, 0x73, 0x74, 0x72, 0x32, 0x73, 0x74, 0x72, 0x4d, 0x61, 0x70,
	0x12, 0x68, 0x0a, 0x0d, 0x73, 0x74, 0x72, 0x32, 0x69, 0x6e, 0x74, 0x33, 0x32, 0x5f, 0x6d, 0x61,
	0x70, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x43, 0x2e, 0x67, 0x72, 0x70, 0x63, 0x62, 0x72,
	0x69, 0x64, 0x67, 0x65, 0x2e, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2e, 0x62, 0x72,
	0x69, 0x64, 0x67, 0x65, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x70, 0x62, 0x2e,
	0x4e, 0x6f, 0x6e, 0x53, 0x63, 0x61, 0x6c, 0x61, 0x72, 0x73, 0x2e, 0x53, 0x74, 0x72, 0x32, 0x69,
	0x6e, 0x74, 0x33, 0x32, 0x4d, 0x61, 0x70, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x0c, 0x73, 0x74,
	0x72, 0x32, 0x69, 0x6e, 0x74, 0x33, 0x32, 0x4d, 0x61, 0x70, 0x12, 0x68, 0x0a, 0x0d, 0x69, 0x6e,
	0x74, 0x32, 0x65, 0x6d, 0x70, 0x74, 0x79, 0x5f, 0x6d, 0x61, 0x70, 0x18, 0x03, 0x20, 0x03, 0x28,
	0x0b, 0x32, 0x43, 0x2e, 0x67, 0x72, 0x70, 0x63, 0x62, 0x72, 0x69, 0x64, 0x67, 0x65, 0x2e, 0x69,
	0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2e, 0x62, 0x72, 0x69, 0x64, 0x67, 0x65, 0x74, 0x65,
	0x73, 0x74, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x70, 0x62, 0x2e, 0x4e, 0x6f, 0x6e, 0x53, 0x63, 0x61,
	0x6c, 0x61, 0x72, 0x73, 0x2e, 0x49, 0x6e, 0x74, 0x32, 0x65, 0x6d, 0x70, 0x74, 0x79, 0x4d, 0x61,
	0x70, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x0c, 0x69, 0x6e, 0x74, 0x32, 0x65, 0x6d, 0x70, 0x74,
	0x79, 0x4d, 0x61, 0x70, 0x1a, 0x3d, 0x0a, 0x0f, 0x53, 0x74, 0x72, 0x32, 0x73, 0x74, 0x72, 0x4d,
	0x61, 0x70, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c,
	0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a,
	0x02, 0x38, 0x01, 0x1a, 0x3f, 0x0a, 0x11, 0x53, 0x74, 0x72, 0x32, 0x69, 0x6e, 0x74, 0x33, 0x32,
	0x4d, 0x61, 0x70, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61,
	0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x05, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65,
	0x3a, 0x02, 0x38, 0x01, 0x1a, 0x57, 0x0a, 0x11, 0x49, 0x6e, 0x74, 0x32, 0x65, 0x6d, 0x70, 0x74,
	0x79, 0x4d, 0x61, 0x70, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x2c, 0x0a, 0x05, 0x76,
	0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x16, 0x2e, 0x67, 0x6f, 0x6f,
	0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70,
	0x74, 0x79, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x42, 0x39, 0x5a,
	0x37, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x72, 0x65, 0x6e, 0x62,
	0x6f, 0x75, 0x2f, 0x67, 0x72, 0x70, 0x63, 0x62, 0x72, 0x69, 0x64, 0x67, 0x65, 0x2f, 0x69, 0x6e,
	0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x62, 0x72, 0x69, 0x64, 0x67, 0x65, 0x74, 0x65, 0x73,
	0x74, 0x2f, 0x74, 0x65, 0x73, 0x74, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_messages_proto_rawDescOnce sync.Once
	file_messages_proto_rawDescData = file_messages_proto_rawDesc
)

func file_messages_proto_rawDescGZIP() []byte {
	file_messages_proto_rawDescOnce.Do(func() {
		file_messages_proto_rawDescData = protoimpl.X.CompressGZIP(file_messages_proto_rawDescData)
	})
	return file_messages_proto_rawDescData
}

var file_messages_proto_msgTypes = make([]protoimpl.MessageInfo, 5)
var file_messages_proto_goTypes = []interface{}{
	(*Scalars)(nil),       // 0: grpcbridge.internal.bridgetest.testpb.Scalars
	(*NonScalars)(nil),    // 1: grpcbridge.internal.bridgetest.testpb.NonScalars
	nil,                   // 2: grpcbridge.internal.bridgetest.testpb.NonScalars.Str2strMapEntry
	nil,                   // 3: grpcbridge.internal.bridgetest.testpb.NonScalars.Str2int32MapEntry
	nil,                   // 4: grpcbridge.internal.bridgetest.testpb.NonScalars.Int2emptyMapEntry
	(*emptypb.Empty)(nil), // 5: google.protobuf.Empty
}
var file_messages_proto_depIdxs = []int32{
	2, // 0: grpcbridge.internal.bridgetest.testpb.NonScalars.str2str_map:type_name -> grpcbridge.internal.bridgetest.testpb.NonScalars.Str2strMapEntry
	3, // 1: grpcbridge.internal.bridgetest.testpb.NonScalars.str2int32_map:type_name -> grpcbridge.internal.bridgetest.testpb.NonScalars.Str2int32MapEntry
	4, // 2: grpcbridge.internal.bridgetest.testpb.NonScalars.int2empty_map:type_name -> grpcbridge.internal.bridgetest.testpb.NonScalars.Int2emptyMapEntry
	5, // 3: grpcbridge.internal.bridgetest.testpb.NonScalars.Int2emptyMapEntry.value:type_name -> google.protobuf.Empty
	4, // [4:4] is the sub-list for method output_type
	4, // [4:4] is the sub-list for method input_type
	4, // [4:4] is the sub-list for extension type_name
	4, // [4:4] is the sub-list for extension extendee
	0, // [0:4] is the sub-list for field type_name
}

func init() { file_messages_proto_init() }
func file_messages_proto_init() {
	if File_messages_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_messages_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Scalars); i {
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
		file_messages_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*NonScalars); i {
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
			RawDescriptor: file_messages_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   5,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_messages_proto_goTypes,
		DependencyIndexes: file_messages_proto_depIdxs,
		MessageInfos:      file_messages_proto_msgTypes,
	}.Build()
	File_messages_proto = out.File
	file_messages_proto_rawDesc = nil
	file_messages_proto_goTypes = nil
	file_messages_proto_depIdxs = nil
}