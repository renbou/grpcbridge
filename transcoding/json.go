package transcoding

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/renbou/grpcbridge/bridgedesc"
	"golang.org/x/exp/constraints"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/structpb"
)

var nullJson = []byte("null")

// JSONMarshaler implements marshaling and unmarshaling of arbitrary protobuf message fields with custom type resolving,
// meant to be used for transcoding request and response messages from different targets.
// Encoding is implemented as specified in the Proto3 [JSON Mapping].
//
// [JSON Mapping]: https://protobuf.dev/programming-guides/proto3/#json
type JSONMarshaler struct {
	protojson.MarshalOptions
	protojson.UnmarshalOptions
}

// JSONEncoder is a streaming alternative to [JSONMarshaler.Marshal].
// It can be created using [JSONMarshaler.NewEncoder], and all marshaled messages will be written to the assigned writer.
type JSONEncoder struct {
	marshaler *JSONMarshaler
	types     bridgedesc.TypeResolver
	writer    io.Writer
}

// JSONDecoder is a streaming alternative to [JSONMarshaler.Unmarshal].
// It can be created using [JSONMarshaler.NewDecoder], and all messages will be unmarshaled from the assigned reader.
type JSONDecoder struct {
	protojson.UnmarshalOptions
	dec *json.Decoder
}

// Marshal marshals the given protobuf message or field to JSON.
// If the field descriptor is nil, the whole message is marshaled, which equates to using [protojson.Marshal].
func (m *JSONMarshaler) Marshal(types bridgedesc.TypeResolver, msg protoreflect.Message, fd protoreflect.FieldDescriptor) ([]byte, error) {
	// copy the options to avoid overwriting them
	opts := m.MarshalOptions

	// Allow overriding the resolver.
	if opts.Resolver == nil {
		opts.Resolver = types
	}

	value := protoreflect.ValueOfMessage(msg)
	if fd != nil {
		value = msg.Get(fd)
	}

	// The simple case is marshaling a valid proto.Message
	if msg, ok := value.Interface().(protoreflect.Message); ok {
		return opts.Marshal(msg.Interface())
	}

	if !value.IsValid() {
		return slices.Clone(nullJson), nil
	}

	// And a whole other case is marshaling arbitrary values from the message.
	// Inspiration taken from
	// https://github.com/protocolbuffers/protobuf-go/blob/ec47fd138f9221b19a2afd6570b3c39ede9df3dc/encoding/protojson/encode.go#L288
	// And
	// https://github.com/grpc-ecosystem/grpc-gateway/blob/b8369842eebbfa62bf7c985c59e4aba640519dc4/runtime/marshal_jsonpb.go#L77
	if fd.IsList() {
		return m.marshalList(&opts, value.List(), fd)
	} else if fd.IsMap() {
		return m.marshalMap(&opts, value.Map(), fd)
	}

	return m.marshalSingular(&opts, value, fd)
}

// Unmarshal unmarshals the given JSON into a protobuf message or field.
// If the field descriptor is nil, the JSON is unmarshaled into the message itself, which equates to using [protojson.Unmarshal].
// Unmarshal expects the message to be mutable, so nested messages should be retrieved by calling Mutable().
func (m *JSONMarshaler) Unmarshal(types bridgedesc.TypeResolver, b []byte, msg protoreflect.Message, fd protoreflect.FieldDescriptor) error {
	return m.NewDecoder(types, bytes.NewReader(b)).Decode(msg, fd)
}

// NewEncoder initializes a new [JSONEncoder] which will use the specified type information for marshaling messages to the given writer.
func (m *JSONMarshaler) NewEncoder(types bridgedesc.TypeResolver, w io.Writer) *JSONEncoder {
	return &JSONEncoder{marshaler: m, types: types, writer: w}
}

// Encode encodes a single message or field value to the encoder's writer.
func (e *JSONEncoder) Encode(msg protoreflect.Message, fd protoreflect.FieldDescriptor) error {
	b, err := e.marshaler.Marshal(e.types, msg, fd)
	if err != nil {
		return err
	}

	_, err = e.writer.Write(b)
	return err
}

// NewDecoder initializes a new [JSONDecoder] which will use the specified type information for unmarshaling messages from the given writer.
func (m *JSONMarshaler) NewDecoder(types bridgedesc.TypeResolver, r io.Reader) *JSONDecoder {
	dec := &JSONDecoder{
		// opts are copied
		UnmarshalOptions: m.UnmarshalOptions,
		dec:              json.NewDecoder(r),
	}

	if dec.UnmarshalOptions.Resolver == nil {
		dec.UnmarshalOptions.Resolver = types
	}

	return dec
}

// Decoder decodes a single message or field value from the decoder's reader.
func (d *JSONDecoder) Decode(msg protoreflect.Message, fd protoreflect.FieldDescriptor) error {
	// Try unmarshaling the whole message at once.
	if fd == nil {
		return d.unmarshalMessage(msg)
	}

	if !msg.Get(fd).IsValid() {
		return nil
	}

	if fd.IsList() {
		return d.unmarshalList(msg.Mutable(fd).List(), fd)
	} else if fd.IsMap() {
		return d.unmarshalMap(msg.Mutable(fd).Map(), fd)
	}

	return d.unmarshalSingular(msg, fd)
}

func (m *JSONMarshaler) marshalSingular(opts *protojson.MarshalOptions, value protoreflect.Value, fd protoreflect.FieldDescriptor) ([]byte, error) {
	marshalVal := value.Interface()

	// fd is guaranteed to be non-nil here, since otherwise value would be a message
	switch fd.Kind() {
	case protoreflect.EnumKind:
		if fd.Enum().FullName() == structpb.NullValue_NULL_VALUE.Descriptor().FullName() {
			return slices.Clone(nullJson), nil
		}

		desc := fd.Enum().Values().ByNumber(value.Enum())
		if opts.UseEnumNumbers || desc == nil {
			marshalVal = int64(value.Enum())
		} else {
			marshalVal = string(desc.Name())
		}
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return opts.Marshal(value.Message().Interface())
	default:
	}

	return m.marshalJSON(marshalVal, opts)
}

func (m *JSONMarshaler) marshalList(opts *protojson.MarshalOptions, protolist protoreflect.List, fd protoreflect.FieldDescriptor) ([]byte, error) {
	if !protolist.IsValid() && !opts.EmitUnpopulated {
		return slices.Clone(nullJson), nil
	}

	marshaled := make([]json.RawMessage, protolist.Len())

	for i := 0; i < protolist.Len(); i++ {
		msg, err := m.marshalSingular(opts, protolist.Get(i), fd)
		if err != nil {
			return nil, err
		}

		marshaled[i] = json.RawMessage(msg)
	}

	return m.marshalJSON(marshaled, opts)
}

func (m *JSONMarshaler) marshalMap(opts *protojson.MarshalOptions, protomap protoreflect.Map, fd protoreflect.FieldDescriptor) ([]byte, error) {
	if !protomap.IsValid() && !opts.EmitUnpopulated {
		return slices.Clone(nullJson), nil
	}

	marshaled := make(map[string]json.RawMessage)

	var marshalErr error

	protomap.Range(func(mk protoreflect.MapKey, v protoreflect.Value) bool {
		msg, err := m.marshalSingular(opts, v, fd.MapValue())
		if err != nil {
			marshalErr = err
			return false
		}

		marshaled[mk.String()] = json.RawMessage(msg)
		return true
	})

	if marshalErr != nil {
		return nil, marshalErr
	}

	return m.marshalJSON(marshaled, opts)
}

func (m *JSONMarshaler) marshalJSON(value any, opts *protojson.MarshalOptions) ([]byte, error) {
	return json.MarshalIndent(value, "", opts.Indent)
}

func (d *JSONDecoder) unmarshalMessage(msg protoreflect.Message) error {
	var b json.RawMessage
	if err := d.dec.Decode(&b); err != nil {
		return err
	}

	return d.UnmarshalOptions.Unmarshal(b, msg.Interface())
}

func (d *JSONDecoder) unmarshalList(protolist protoreflect.List, fd protoreflect.FieldDescriptor) error {
	var marshaled []json.RawMessage

	if err := d.dec.Decode(&marshaled); err != nil {
		return err
	}

	originalDec := d.dec
	defer func() {
		d.dec = originalDec
	}()

	for _, raw := range marshaled {
		d.dec = json.NewDecoder(bytes.NewReader(raw))

		switch fd.Kind() {
		case protoreflect.MessageKind, protoreflect.GroupKind:
			if err := d.unmarshalMessage(protolist.AppendMutable().Message()); err != nil {
				return err
			}
		default:
			value, err := d.unmarshalScalar(fd)
			if err != nil {
				return err
			}

			protolist.Append(value)
		}
	}

	return nil
}

func (d *JSONDecoder) unmarshalMap(protomap protoreflect.Map, fd protoreflect.FieldDescriptor) error {
	var marshaled map[string]json.RawMessage

	if err := d.dec.Decode(&marshaled); err != nil {
		return err
	}

	originalDec := d.dec
	defer func() {
		d.dec = originalDec
	}()

	for key, raw := range marshaled {
		d.dec = json.NewDecoder(strings.NewReader(strconv.Quote(key)))

		keyValue, err := d.unmarshalScalar(fd.MapKey())
		if err != nil {
			return fmt.Errorf("invalid map key value: %w", err)
		}

		d.dec = json.NewDecoder(bytes.NewReader(raw))

		var mappedValue protoreflect.Value

		switch fd.MapValue().Kind() {
		case protoreflect.MessageKind, protoreflect.GroupKind:
			mappedValue = protomap.NewValue()
			if err := d.unmarshalMessage(mappedValue.Message()); err != nil {
				return err
			}
		default:
			value, err := d.unmarshalScalar(fd.MapValue())
			if err != nil {
				return err
			}

			mappedValue = value
		}

		protomap.Set(protoreflect.MapKey(keyValue), mappedValue)
	}

	return nil
}

func (d *JSONDecoder) unmarshalSingular(msg protoreflect.Message, fd protoreflect.FieldDescriptor) error {
	// fd is guaranteed to be non-nil here, since otherwise value would be a message
	switch fd.Kind() {
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return d.unmarshalMessage(msg.Mutable(fd).Message())
	}

	value, err := d.unmarshalScalar(fd)
	if err != nil {
		return err
	}

	msg.Set(fd, value)
	return nil
}

func (d *JSONDecoder) unmarshalScalar(fd protoreflect.FieldDescriptor) (protoreflect.Value, error) {
	// All the cases specified in https://pkg.go.dev/google.golang.org/protobuf@v1.33.0/reflect/protoreflect#Value
	switch fd.Kind() {
	case protoreflect.BoolKind:
		return jsonValueDecode(d.dec, protoreflect.ValueOfBool)
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return jsonValueDecode(d.dec, jsonConvertNumber(protoreflect.ValueOfInt32))
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return jsonValueDecode(d.dec, jsonConvertNumber(protoreflect.ValueOfInt64))
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return jsonValueDecode(d.dec, jsonConvertNumber(protoreflect.ValueOfUint32))
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return jsonValueDecode(d.dec, jsonConvertNumber(protoreflect.ValueOfUint64))
	case protoreflect.FloatKind:
		return jsonFloatDecode(fd, d.dec, protoreflect.ValueOfFloat32)
	case protoreflect.DoubleKind:
		return jsonFloatDecode(fd, d.dec, protoreflect.ValueOfFloat64)
	case protoreflect.StringKind:
		return jsonValueDecode(d.dec, protoreflect.ValueOfString)
	case protoreflect.BytesKind:
		return jsonValueDecode(d.dec, protoreflect.ValueOfBytes)
	case protoreflect.EnumKind:
		var repr any
		if err := d.dec.Decode(&repr); err != nil {
			return protoreflect.Value{}, err
		}

		if fd.Enum().FullName() == structpb.NullValue_NULL_VALUE.Descriptor().FullName() && repr == nil {
			return protoreflect.ValueOfEnum(0), nil
		}

		switch v := repr.(type) {
		case string:
			if enumVal := fd.Enum().Values().ByName(protoreflect.Name(v)); enumVal != nil {
				return protoreflect.ValueOfEnum(enumVal.Number()), nil
			} else if d.UnmarshalOptions.DiscardUnknown {
				return protoreflect.Value{}, nil
			}
		case float64:
			return protoreflect.ValueOfEnum(protoreflect.EnumNumber(v)), nil
		}

		return protoreflect.Value{}, fmt.Errorf("invalid value for %v type: %v", fd.Kind(), repr)
	default:
		panic(fmt.Sprintf("grpcbridge: transcoding: invalid scalar kind %v", fd.Kind()))
	}
}

func jsonValueDecode[T any](dec *json.Decoder, convert func(T) protoreflect.Value) (protoreflect.Value, error) {
	var val T
	if err := dec.Decode(&val); err != nil {
		return protoreflect.Value{}, err
	}

	return convert(val), nil
}

func jsonConvertNumber[T constraints.Integer](convert func(T) protoreflect.Value) func(json.Number) protoreflect.Value {
	return func(n json.Number) protoreflect.Value {
		i, _ := n.Int64()
		return convert(T(i))
	}
}

func jsonFloatDecode[T constraints.Float](fd protoreflect.FieldDescriptor, dec *json.Decoder, convert func(T) protoreflect.Value) (protoreflect.Value, error) {
	tok, err := dec.Token()
	if err != nil {
		return protoreflect.Value{}, err
	}

	switch tok := tok.(type) {
	case float64:
		return convert(T(tok)), nil
	case string:
		// this supports NaN, -Infinity, +Infinity
		f, err := strconv.ParseFloat(tok, 64)
		if err == nil {
			return convert(T(f)), nil
		}
	}

	return protoreflect.Value{}, fmt.Errorf("invalid value for %v type: %v", fd.Kind(), tok)
}
