package transcoding

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/renbou/grpcbridge/internal/bridgetest/testpb"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
)

var (
	scalarFields    = new(testpb.Scalars).ProtoReflect().Descriptor().Fields()
	nonScalarFields = new(testpb.NonScalars).ProtoReflect().Descriptor().Fields()
	wellKnownFields = new(testpb.WellKnown).ProtoReflect().Descriptor().Fields()
)

func protoCmp() cmp.Option {
	return cmp.Options{
		protocmp.Transform(),
		cmpopts.EquateNaNs(),
	}
}

// Test_JSONMarshaler_Marshal tests that JSONMarshaler is able to properly marshal arbitrary message fields as JSON.
func Test_JSONMarshaler_Marshal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		opts     *protojson.MarshalOptions
		message  proto.Message
		field    protoreflect.FieldDescriptor
		expected any
	}{
		{
			name: "scalars",
			message: &testpb.Scalars{
				BoolValue:     true,
				Int32Value:    1234,
				Int64Value:    -1234,
				Uint32Value:   42,
				Uint64Value:   2000,
				Sint32Value:   -1010,
				Sint64Value:   -3023,
				Fixed32Value:  1028,
				Fixed64Value:  1200,
				Sfixed32Value: -1,
				Sfixed64Value: -2,
				FloatValue:    3.14,
				DoubleValue:   3.14159,
				StringValue:   "stringy string",
				BytesValue:    []byte("bytey bytes"),
			},
			expected: map[string]any{
				"boolValue":     true,
				"int32Value":    1234.0,
				"int64Value":    "-1234",
				"uint32Value":   42.0,
				"uint64Value":   "2000",
				"sint32Value":   -1010.0,
				"sint64Value":   "-3023",
				"fixed32Value":  1028.0,
				"fixed64Value":  "1200",
				"sfixed32Value": -1.0,
				"sfixed64Value": "-2",
				"floatValue":    3.14,
				"doubleValue":   3.14159,
				"stringValue":   "stringy string",
				"bytesValue":    "Ynl0ZXkgYnl0ZXM=",
			},
		},
		{
			name: "string",
			message: &testpb.Scalars{
				StringValue: "stringy",
			},
			field:    scalarFields.ByName("string_value"),
			expected: "stringy",
		},
		{
			name: "bool",
			message: &testpb.Scalars{
				BoolValue: true,
			},
			field:    scalarFields.ByName("bool_value"),
			expected: true,
		},
		{
			name: "int32",
			message: &testpb.Scalars{
				Int32Value: 123,
			},
			field:    scalarFields.ByName("int32_value"),
			expected: 123.0,
		},
		{
			name: "int64",
			message: &testpb.Scalars{
				Int64Value: -123,
			},
			field:    scalarFields.ByName("int64_value"),
			expected: -123.0,
		},
		{
			name: "bytes",
			message: &testpb.Scalars{
				BytesValue: []byte("bytey bytes"),
			},
			field:    scalarFields.ByName("bytes_value"),
			expected: "Ynl0ZXkgYnl0ZXM=",
		},
		{
			name:     "enum",
			message:  &testpb.NonScalars{RootDigits: testpb.Digits_ONE},
			field:    nonScalarFields.ByName("root_digits"),
			expected: "ONE",
		},
		{
			name:     "enum as number",
			opts:     &protojson.MarshalOptions{UseEnumNumbers: true},
			message:  &testpb.NonScalars{RootDigits: testpb.Digits_ONE},
			field:    nonScalarFields.ByName("root_digits"),
			expected: 1.0,
		},
		{
			name:     "null",
			message:  &testpb.WellKnown{NullValue: structpb.NullValue_NULL_VALUE},
			field:    wellKnownFields.ByName("null_value"),
			expected: nil,
		},
		{
			name: "list of strings",
			message: &testpb.NonScalars{
				StrList: []string{"a", "b", "c"},
			},
			field:    nonScalarFields.ByName("str_list"),
			expected: []any{"a", "b", "c"},
		},
		{
			name:     "null list",
			message:  &testpb.NonScalars{},
			field:    nonScalarFields.ByName("str_list"),
			expected: nil,
		},
		{
			name: "complex list",
			message: &testpb.NonScalars{
				MsgList: []*testpb.NonScalars_Child{
					{Nested: &testpb.NonScalars_Child_Digits{Digits: testpb.Digits_ONE}},
					{Nested: &testpb.NonScalars_Child_GrandChild_{GrandChild: &testpb.NonScalars_Child_GrandChild{
						BytesValue: []byte("bytey bytes"),
					}}},
					{Nested: &testpb.NonScalars_Child_Digits{Digits: testpb.Digits_TWO}},
				},
			},
			field: nonScalarFields.ByName("msg_list"),
			expected: []any{
				map[string]any{"digits": "ONE"},
				map[string]any{"grandChild": map[string]any{"bytesValue": "Ynl0ZXkgYnl0ZXM="}},
				map[string]any{"digits": "TWO"},
			},
		},
		{
			name: "str2str map",
			message: &testpb.NonScalars{
				Str2StrMap: map[string]string{
					"a": "b",
					"c": "d",
				},
			},
			field:    nonScalarFields.ByName("str2str_map"),
			expected: map[string]any{"a": "b", "c": "d"},
		},
		{
			name: "int2empty map",
			message: &testpb.NonScalars{
				Int2EmptyMap: map[int32]*emptypb.Empty{
					0: new(emptypb.Empty),
				},
			},
			field:    nonScalarFields.ByName("int2empty_map"),
			expected: map[string]any{"0": map[string]any{}},
		},
		{
			name:     "null map",
			message:  &testpb.NonScalars{},
			field:    nonScalarFields.ByName("str2str_map"),
			expected: nil,
		},
		{
			name:     "duration",
			message:  &testpb.NonScalars{Duration: durationpb.New(time.Second * 123)},
			field:    nonScalarFields.ByName("duration"),
			expected: "123s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			marshaler := JSONMarshaler{}

			if tt.opts != nil {
				marshaler.MarshalOptions = *tt.opts
			}

			// Act
			actualData, marshalErr := marshaler.Marshal(testpb.TestServiceTypesResolver, tt.message.ProtoReflect(), tt.field)

			var actual any
			json.Unmarshal(actualData, &actual)

			// Assert
			if marshalErr != nil {
				t.Fatalf("Marshal() returned non-nil error = %q", marshalErr)
			}

			if diff := cmp.Diff(tt.expected, actual); diff != "" {
				t.Errorf("Marshal() results differing from expected(-want+got):\n%s", diff)
			}
		})
	}
}

// Test_JSONMarshaler_Unmarshal tests that JSONMarshaler is able to properly unmarshal arbitrary message fields from JSON.
func Test_JSONMarshaler_Unmarshal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    any
		message  proto.Message
		field    protoreflect.FieldDescriptor
		expected proto.Message
	}{
		{
			name: "scalars",
			input: map[string]any{
				"boolValue":     true,
				"int32Value":    1234.0,
				"int64Value":    "-1234",
				"uint32Value":   42.0,
				"uint64Value":   "2000",
				"sint32Value":   -1010.0,
				"sint64Value":   "-3023",
				"fixed32Value":  1028.0,
				"fixed64Value":  "1200",
				"sfixed32Value": -1.0,
				"sfixed64Value": "-2",
				"floatValue":    3.14,
				"doubleValue":   3.14159,
				"stringValue":   "stringy string",
				"bytesValue":    "Ynl0ZXkgYnl0ZXM=",
			},
			message: new(testpb.Scalars),
			field:   nil,
			expected: &testpb.Scalars{
				BoolValue:     true,
				Int32Value:    1234,
				Int64Value:    -1234,
				Uint32Value:   42,
				Uint64Value:   2000,
				Sint32Value:   -1010,
				Sint64Value:   -3023,
				Fixed32Value:  1028,
				Fixed64Value:  1200,
				Sfixed32Value: -1,
				Sfixed64Value: -2,
				FloatValue:    3.14,
				DoubleValue:   3.14159,
				StringValue:   "stringy string",
				BytesValue:    []byte("bytey bytes"),
			},
		},
		{
			name:     "bool",
			input:    true,
			message:  new(testpb.Scalars),
			field:    scalarFields.ByName("bool_value"),
			expected: &testpb.Scalars{BoolValue: true},
		},
		{
			name:     "int32",
			input:    123,
			message:  new(testpb.Scalars),
			field:    scalarFields.ByName("int32_value"),
			expected: &testpb.Scalars{Int32Value: 123},
		},
		{
			name:     "sint32",
			input:    -123,
			message:  new(testpb.Scalars),
			field:    scalarFields.ByName("sint32_value"),
			expected: &testpb.Scalars{Sint32Value: -123},
		},
		{
			name:     "sfixed32",
			input:    -123,
			message:  new(testpb.Scalars),
			field:    scalarFields.ByName("sfixed32_value"),
			expected: &testpb.Scalars{Sfixed32Value: -123},
		},
		{
			name:     "int64",
			input:    "123",
			message:  new(testpb.Scalars),
			field:    scalarFields.ByName("int64_value"),
			expected: &testpb.Scalars{Int64Value: 123},
		},
		{
			name:     "sint64",
			input:    "-123",
			message:  new(testpb.Scalars),
			field:    scalarFields.ByName("sint64_value"),
			expected: &testpb.Scalars{Sint64Value: -123},
		},
		{
			name:     "sfixed64",
			input:    "-123",
			message:  new(testpb.Scalars),
			field:    scalarFields.ByName("sfixed64_value"),
			expected: &testpb.Scalars{Sfixed64Value: -123},
		},
		{
			name:     "uint32",
			input:    424,
			message:  new(testpb.Scalars),
			field:    scalarFields.ByName("uint32_value"),
			expected: &testpb.Scalars{Uint32Value: 424},
		},
		{
			name:     "fixed32",
			input:    424,
			message:  new(testpb.Scalars),
			field:    scalarFields.ByName("fixed32_value"),
			expected: &testpb.Scalars{Fixed32Value: 424},
		},
		{
			name:     "uint64",
			input:    424,
			message:  new(testpb.Scalars),
			field:    scalarFields.ByName("uint64_value"),
			expected: &testpb.Scalars{Uint64Value: 424},
		},
		{
			name:     "fixed64",
			input:    "424",
			message:  new(testpb.Scalars),
			field:    scalarFields.ByName("fixed64_value"),
			expected: &testpb.Scalars{Fixed64Value: 424},
		},
		{
			name:     "float number",
			input:    1.23,
			message:  new(testpb.Scalars),
			field:    scalarFields.ByName("float_value"),
			expected: &testpb.Scalars{FloatValue: 1.23},
		},
		{
			name:     "float str",
			input:    "1.23",
			message:  new(testpb.Scalars),
			field:    scalarFields.ByName("float_value"),
			expected: &testpb.Scalars{FloatValue: 1.23},
		},
		{
			name:     "float NaN",
			input:    "NaN",
			message:  new(testpb.Scalars),
			field:    scalarFields.ByName("float_value"),
			expected: &testpb.Scalars{FloatValue: float32(math.NaN())},
		},
		{
			name:     "float +Infinity",
			input:    "+Infinity",
			message:  new(testpb.Scalars),
			field:    scalarFields.ByName("float_value"),
			expected: &testpb.Scalars{FloatValue: float32(math.Inf(1))},
		},
		{
			name:     "float -Infinity",
			input:    "-Infinity",
			message:  new(testpb.Scalars),
			field:    scalarFields.ByName("float_value"),
			expected: &testpb.Scalars{FloatValue: float32(math.Inf(-1))},
		},
		{
			name:     "double number",
			input:    1.23,
			message:  new(testpb.Scalars),
			field:    scalarFields.ByName("double_value"),
			expected: &testpb.Scalars{DoubleValue: 1.23},
		},
		{
			name:     "double str",
			input:    "1.23",
			message:  new(testpb.Scalars),
			field:    scalarFields.ByName("double_value"),
			expected: &testpb.Scalars{DoubleValue: 1.23},
		},
		{
			name:     "double NaN",
			input:    "NaN",
			message:  new(testpb.Scalars),
			field:    scalarFields.ByName("double_value"),
			expected: &testpb.Scalars{DoubleValue: float64(math.NaN())},
		},
		{
			name:     "double +Infinity",
			input:    "+Infinity",
			message:  new(testpb.Scalars),
			field:    scalarFields.ByName("double_value"),
			expected: &testpb.Scalars{DoubleValue: float64(math.Inf(1))},
		},
		{
			name:     "double -Infinity",
			input:    "-Infinity",
			message:  new(testpb.Scalars),
			field:    scalarFields.ByName("double_value"),
			expected: &testpb.Scalars{DoubleValue: float64(math.Inf(-1))},
		},
		{
			name:     "string",
			input:    "stringy",
			message:  new(testpb.Scalars),
			field:    scalarFields.ByName("string_value"),
			expected: &testpb.Scalars{StringValue: "stringy"},
		},
		{
			name:     "bytes",
			input:    "Ynl0ZXkgYnl0ZXM=",
			message:  new(testpb.Scalars),
			field:    scalarFields.ByName("bytes_value"),
			expected: &testpb.Scalars{BytesValue: []byte("bytey bytes")},
		},
		{
			name:     "enum",
			input:    "ONE",
			message:  new(testpb.NonScalars),
			field:    nonScalarFields.ByName("root_digits"),
			expected: &testpb.NonScalars{RootDigits: testpb.Digits_ONE},
		},
		{
			name:     "null enum",
			input:    nil,
			message:  new(testpb.WellKnown),
			field:    wellKnownFields.ByName("null_value"),
			expected: &testpb.WellKnown{NullValue: structpb.NullValue_NULL_VALUE},
		},
		{
			name:     "enum number",
			input:    1,
			message:  new(testpb.NonScalars),
			field:    nonScalarFields.ByName("root_digits"),
			expected: &testpb.NonScalars{RootDigits: testpb.Digits_ONE},
		},
		{
			name:     "duration",
			input:    "123s",
			message:  new(testpb.NonScalars),
			field:    nonScalarFields.ByName("duration"),
			expected: &testpb.NonScalars{Duration: durationpb.New(time.Second * 123)},
		},
		{
			name:     "list of strings",
			input:    []any{"a", "b", "c"},
			message:  new(testpb.NonScalars),
			field:    nonScalarFields.ByName("str_list"),
			expected: &testpb.NonScalars{StrList: []string{"a", "b", "c"}},
		},
		{
			name:    "list of messages",
			input:   []any{map[string]any{"digits": "ONE"}, map[string]any{"grandChild": map[string]any{"bytes_value": "Ynl0ZXkgYnl0ZXM="}}, map[string]any{"digits": "TWO"}},
			message: new(testpb.NonScalars),
			field:   nonScalarFields.ByName("msg_list"),
			expected: &testpb.NonScalars{MsgList: []*testpb.NonScalars_Child{
				{Nested: &testpb.NonScalars_Child_Digits{Digits: testpb.Digits_ONE}},
				{Nested: &testpb.NonScalars_Child_GrandChild_{GrandChild: &testpb.NonScalars_Child_GrandChild{
					BytesValue: []byte("bytey bytes"),
				}}},
				{Nested: &testpb.NonScalars_Child_Digits{Digits: testpb.Digits_TWO}},
			}},
		},
		{
			name: "map of strings",
			input: map[string]any{
				"a": "b",
				"c": "d",
			},
			message: new(testpb.NonScalars),
			field:   nonScalarFields.ByName("str2str_map"),
			expected: &testpb.NonScalars{Str2StrMap: map[string]string{
				"a": "b",
				"c": "d",
			}},
		},
		{
			name: "map of empty",
			input: map[string]any{
				"123": map[string]any{},
				"456": map[string]any{},
			},
			message: new(testpb.NonScalars),
			field:   nonScalarFields.ByName("int2empty_map"),
			expected: &testpb.NonScalars{Int2EmptyMap: map[int32]*emptypb.Empty{
				123: new(emptypb.Empty),
				456: new(emptypb.Empty),
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			marshaler := JSONMarshaler{}

			input, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("json.Marshal() return non-nil error = %q on input message", err)
			}

			// Act
			unmarshalErr := marshaler.Unmarshal(testpb.TestServiceTypesResolver, input, tt.message.ProtoReflect(), tt.field)

			// Assert
			if unmarshalErr != nil {
				t.Fatalf("Unmarshal() returned non-nil error = %q", unmarshalErr)
			}

			if diff := cmp.Diff(tt.expected, tt.message, protoCmp()); diff != "" {
				t.Errorf("Unmarshal() results differing from expected(-want+got):\n%s", diff)
			}
		})
	}
}

// Test_JSONMarshaler_Unmarshal_Errors tests the error handling capabilities of Unmarshal.
func Test_JSONMarshaler_Unmarshal_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		message proto.Message
		field   protoreflect.FieldDescriptor
		wantErr string
	}{
		{
			name:    "scalar in message",
			input:   `{"boolValue":123}`,
			message: new(testpb.Scalars),
			field:   nil,
			wantErr: "invalid value for bool type",
		},
		{
			name:    "invalid message",
			input:   `asd`,
			message: new(testpb.Scalars),
			field:   nil,
			wantErr: "invalid character 'a' looking for beginning of value",
		},
		{
			name:    "scalar",
			input:   `123`,
			message: new(testpb.Scalars),
			field:   scalarFields.ByName("bool_value"),
			wantErr: "cannot unmarshal number into Go value of type bool",
		},
		{
			name:    "float",
			input:   `"str"`,
			message: new(testpb.Scalars),
			field:   scalarFields.ByName("float_value"),
			wantErr: "invalid value for float type",
		},
		{
			name:    "missing scalar",
			input:   ``,
			message: new(testpb.Scalars),
			field:   scalarFields.ByName("float_value"),
			wantErr: "EOF",
		},
		{
			name:    "invalid scalar list",
			input:   `[{}, true]`,
			message: new(testpb.NonScalars),
			field:   nonScalarFields.ByName("str_list"),
			wantErr: "cannot unmarshal object into Go value of type string",
		},
		{
			name:    "invalid message list",
			input:   `[1]`,
			message: new(testpb.NonScalars),
			field:   nonScalarFields.ByName("msg_list"),
			wantErr: "syntax error",
		},
		{
			name:    "invalid list",
			input:   `1`,
			message: new(testpb.NonScalars),
			field:   nonScalarFields.ByName("str_list"),
			wantErr: "cannot unmarshal number into Go value of type []json.RawMessage",
		},
		{
			name:    "invalid enum",
			input:   `asdfjd`,
			message: new(testpb.NonScalars),
			field:   nonScalarFields.ByName("root_digits"),
			wantErr: "invalid character 'a' looking for beginning of value",
		},
		{
			name:    "invalid enum value",
			input:   `"FIFTY_SEVEN"`,
			message: new(testpb.NonScalars),
			field:   nonScalarFields.ByName("root_digits"),
			wantErr: "invalid value for enum type: FIFTY_SEVEN",
		},
		{
			name:    "invalid map",
			input:   `1`,
			message: new(testpb.NonScalars),
			field:   nonScalarFields.ByName("str2str_map"),
			wantErr: "cannot unmarshal number into Go value of type map[string]json.RawMessage",
		},
		{
			name:    "invalid map key",
			input:   `{"nan":{}}`,
			message: new(testpb.NonScalars),
			field:   nonScalarFields.ByName("int2empty_map"),
			wantErr: "invalid map key value",
		},
		{
			name:    "invalid map message value",
			input:   `{"1":true}`,
			message: new(testpb.NonScalars),
			field:   nonScalarFields.ByName("int2empty_map"),
			wantErr: "syntax error",
		},
		{
			name:    "invalid map scalar value",
			input:   `{"1":{}}`,
			message: new(testpb.NonScalars),
			field:   nonScalarFields.ByName("str2str_map"),
			wantErr: "cannot unmarshal object into Go value of type string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			marshaler := JSONMarshaler{}

			// Act
			unmarshalErr := marshaler.Unmarshal(testpb.TestServiceTypesResolver, []byte(tt.input), tt.message.ProtoReflect(), tt.field)

			// Assert
			if unmarshalErr == nil {
				t.Errorf("Unmarshal() returned nil error despite invalid input")
			} else if !strings.Contains(unmarshalErr.Error(), tt.wantErr) {
				t.Errorf("Unmarshal() returned error = %q, want it to contain %q", unmarshalErr.Error(), tt.wantErr)
			}
		})
	}
}

// Test_JSONEncoder_Encode tests that stream-encoding multiple messages works using a JSONEncoder.
func Test_JSONEncoder_Encode(t *testing.T) {
	t.Parallel()

	// Arrange
	buf := bytes.NewBuffer(nil)
	marshaler := JSONMarshaler{}
	encoder := marshaler.NewEncoder(testpb.TestServiceTypesResolver, buf)

	const want = `true` + "\n" + `"Str"`

	// Act
	_ = encoder.Encode((&testpb.Scalars{BoolValue: true}).ProtoReflect(), scalarFields.ByName("bool_value"))
	buf.WriteByte('\n')
	_ = encoder.Encode((&testpb.Scalars{StringValue: "Str"}).ProtoReflect(), scalarFields.ByName("string_value"))

	// Assert
	if diff := cmp.Diff(want, buf.String()); diff != "" {
		t.Errorf("Encode() results differing from expected(-want+got):\n%s", diff)
	}
}

// Test_JSONDecoder_Decode tests that stream-decoding multiple messages works using a JSONDecoder.
func Test_JSONDecoder_Decode(t *testing.T) {
	t.Parallel()

	// Arrange
	reader := strings.NewReader("true\n123")
	marshaler := JSONMarshaler{}
	decoder := marshaler.NewDecoder(testpb.TestServiceTypesResolver, reader)

	msgA := new(testpb.Scalars)
	msgB := new(testpb.Scalars)

	wantA := &testpb.Scalars{BoolValue: true}
	wantB := &testpb.Scalars{Int64Value: 123}

	// Act
	decoder.Decode(msgA.ProtoReflect(), scalarFields.ByName("bool_value"))
	decoder.Decode(msgB.ProtoReflect(), scalarFields.ByName("int64_value"))

	// Assert
	if diff := cmp.Diff(wantA, msgA, protoCmp()); diff != "" {
		t.Errorf("Decode() results for first message differing from expected(-want+got):\n%s", diff)
	}

	if diff := cmp.Diff(wantB, msgB, protoCmp()); diff != "" {
		t.Errorf("Decode() results for second message differing from expected(-want+got):\n%s", diff)
	}
}
