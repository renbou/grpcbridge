package grpcadapter_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/renbou/grpcbridge/grpcadapter"
	"google.golang.org/grpc/metadata"
)

var testProxyMDFilterOpts = grpcadapter.ProxyMDFilterOpts{
	// different cases to check that the filter supports non-lowercased options.
	AllowRequestMD:   []string{"Authorization", "grpc-metadata-foo", "binary-bin", "Grpc-Metadata-Binbin-Bin"},
	PrefixRequestMD:  "Grpc-Metadata-",
	AllowResponseMD:  []string{"Cookie", "grpc-metadata-header", "outgOIng-bIn"},
	PrefixResponseMD: "Grpc-outgoing-",
	AllowTrailerMD:   []string{"trace-id", "grpc-outgoing-trailer"},
	PrefixTrailerMD:  "grpc-outgoing-trailer-",
}

var testMD = metadata.MD{
	"authorization":                     []string{"Bearer token"},
	"grpc-metadata-foo":                 []string{"bar", "value1", "value2"},
	"binary-bin":                        []string{"dGVz", "dGVzdA==", "dGVzdA"},
	"grpc-metadata-binbin-bin":          []string{"dGU="},
	"grpc-metadata-not-allowed":         []string{"vals"},
	"nono":                              []string{"example"},
	"grpc-timeout":                      []string{"10s"},
	"extra-trailer":                     []string{"val"},
	"grpc-metadata-allowed-trailer-bin": []string{"AAED"},
	"cookie":                            []string{"session"},
	"grpc-metadata-header":              []string{"return1", "return2"},
	"outgoing-bin":                      []string{"AA==", "AA"},
	"trace-id":                          []string{"12345"},
	"grpc-outgoing-trailer":             []string{"10.01230"},
}

// Test_ProxyMDFilter_FilterRequestMD tests that ProxyMDFilter properly forwards allowed incoming headers
// with prefixes, and handles special cases like grpc-metadata-, -bin, and grpc-timeout.
func Test_ProxyMDFilter_FilterRequestMD(t *testing.T) {
	t.Parallel()

	// no prefix for most of the test cases
	opts := testProxyMDFilterOpts
	opts.PrefixRequestMD = ""

	tests := []struct {
		name   string
		modify func(opts *grpcadapter.ProxyMDFilterOpts)
		in     metadata.MD
		out    metadata.MD
	}{
		{
			name: "empty",
			in:   metadata.MD{},
			out:  metadata.MD{},
		},
		{
			name: "just timeout",
			in:   metadata.MD{"grpc-timeout": []string{"100", "20"}},
			out:  metadata.MD{"grpc-timeout": []string{"100", "20"}},
		},
		{
			name: "no prefix",
			in:   testMD,
			out: metadata.MD{
				"authorization": []string{"Bearer token"},
				"foo":           []string{"bar", "value1", "value2"},
				"binary-bin":    []string{"tes", "test", "test"},
				"binbin-bin":    []string{"te"},
				"grpc-timeout":  []string{"10s"},
			},
		},
		{
			name: "prefix",
			modify: func(opts *grpcadapter.ProxyMDFilterOpts) {
				opts.PrefixRequestMD = testProxyMDFilterOpts.PrefixRequestMD
			},
			in: testMD,
			out: metadata.MD{
				"grpc-metadata-authorization": []string{"Bearer token"},
				"foo":                         []string{"bar", "value1", "value2"},
				"grpc-metadata-binary-bin":    []string{"tes", "test", "test"},
				"binbin-bin":                  []string{"te"},
				"grpc-timeout":                []string{"10s"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			topts := opts
			if tt.modify != nil {
				tt.modify(&topts)
			}

			pf := grpcadapter.NewProxyMDFilter(topts)

			// Act
			out := pf.FilterRequestMD(tt.in)

			// Assert
			if diff := cmp.Diff(tt.out, out); diff != "" {
				t.Errorf("ProxyMDFilter.FilterRequestMD() returned metadata differing from expected (-want +got):\n%s", diff)
			}
		})
	}
}

// Test_ProxyMDFilter_FilterResponseMD tests the outgoing metadata forwarding of ProxyMDFilter.
// Outgoing metadata should not be transformed in any way - so no grpc-metadata-, -bin, grpc-timeout.
func Test_ProxyMDFilter_FilterResponseMD(t *testing.T) {
	t.Parallel()

	// no prefix for most of the test cases
	opts := testProxyMDFilterOpts
	opts.PrefixResponseMD = ""

	tests := []struct {
		name   string
		modify func(opts *grpcadapter.ProxyMDFilterOpts)
		in     metadata.MD
		out    metadata.MD
	}{
		{
			name: "empty",
			in:   metadata.MD{},
			out:  metadata.MD{},
		},
		{
			name: "just timeout",
			in:   metadata.MD{"grpc-timeout": []string{"100", "20"}},
			out:  metadata.MD{},
		},
		{
			name: "no prefix",
			in:   testMD,
			out: metadata.MD{
				"cookie":               []string{"session"},
				"grpc-metadata-header": []string{"return1", "return2"},
				"outgoing-bin":         []string{"AA==", "AA"},
			},
		},
		{
			name: "prefix",
			modify: func(opts *grpcadapter.ProxyMDFilterOpts) {
				opts.PrefixResponseMD = testProxyMDFilterOpts.PrefixResponseMD
			},
			in: testMD,
			out: metadata.MD{
				"grpc-outgoing-cookie":               []string{"session"},
				"grpc-outgoing-grpc-metadata-header": []string{"return1", "return2"},
				"grpc-outgoing-outgoing-bin":         []string{"AA==", "AA"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			topts := opts
			if tt.modify != nil {
				tt.modify(&topts)
			}

			pf := grpcadapter.NewProxyMDFilter(topts)

			// Act
			out := pf.FilterResponseMD(tt.in)

			// Assert
			if diff := cmp.Diff(tt.out, out); diff != "" {
				t.Errorf("ProxyMDFilter.FilterResponseMD() returned metadata differing from expected (-want +got):\n%s", diff)
			}
		})
	}
}

// Test_ProxyMDFilter_FilterTrailerMD tests that FilterTrailerMD works exactly like FilterResponseMD but uses other allowlists and prefixes.
func Test_ProxyMDFilter_FilterTrailerMD(t *testing.T) {
	t.Parallel()

	// no prefix for most of the test cases
	opts := testProxyMDFilterOpts
	opts.PrefixTrailerMD = ""

	tests := []struct {
		name   string
		modify func(opts *grpcadapter.ProxyMDFilterOpts)
		in     metadata.MD
		out    metadata.MD
	}{
		{
			name: "empty",
			in:   metadata.MD{},
			out:  metadata.MD{},
		},
		{
			name: "just timeout",
			in:   metadata.MD{"grpc-timeout": []string{"100", "20"}},
			out:  metadata.MD{},
		},
		{
			name: "no prefix",
			in:   testMD,
			out: metadata.MD{
				"trace-id":              []string{"12345"},
				"grpc-outgoing-trailer": []string{"10.01230"},
			},
		},
		{
			name: "prefix",
			modify: func(opts *grpcadapter.ProxyMDFilterOpts) {
				opts.PrefixTrailerMD = testProxyMDFilterOpts.PrefixTrailerMD
			},
			in: testMD,
			out: metadata.MD{
				"grpc-outgoing-trailer-trace-id":              []string{"12345"},
				"grpc-outgoing-trailer-grpc-outgoing-trailer": []string{"10.01230"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			topts := opts
			if tt.modify != nil {
				tt.modify(&topts)
			}

			pf := grpcadapter.NewProxyMDFilter(topts)

			// Act
			out := pf.FilterTrailerMD(tt.in)

			// Assert
			if diff := cmp.Diff(tt.out, out); diff != "" {
				t.Errorf("ProxyMDFilter.FilterTrailerMD() returned metadata differing from expected (-want +got):\n%s", diff)
			}
		})
	}
}
