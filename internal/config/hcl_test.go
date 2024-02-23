package config

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/renbou/grpcbridge"
)

func testpath(filename string) string {
	return filepath.Join("testdata", filename)
}

func Test_ReadHCL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		filename   string
		wantConfig *grpcbridge.Config
	}{
		{
			filename: "config-single.hcl",
			wantConfig: &grpcbridge.Config{
				Services: map[string]grpcbridge.ServiceConfig{
					"test": {
						Target: "127.0.0.1:50051",
					},
				},
			},
		},
		{
			filename: "config-multiple.hcl",
			wantConfig: &grpcbridge.Config{
				Services: map[string]grpcbridge.ServiceConfig{
					"testsvc1": {
						Target: "127.0.0.1:50052",
					},
					"testsvc2": {
						Target: "scheme://testsvc2:grpc",
					},
					"testsvc3": {
						Target: "https://127.0.0.1:50054",
					},
				},
			},
		},
		{
			filename: "config-json.json",
			wantConfig: &grpcbridge.Config{
				Services: map[string]grpcbridge.ServiceConfig{
					"testsvc": {
						Target: "localhost:50051",
					},
					"anothersvc": {
						Target: "localhost:50052",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			t.Parallel()

			gotConfig, err := ReadHCL(testpath(tt.filename))
			if err != nil {
				t.Fatalf("ReadHCL(%q) returned error = %v, want nil", tt.filename, err)
			}

			if diff := cmp.Diff(tt.wantConfig, gotConfig); diff != "" {
				t.Errorf("ReadHCL(%q) returned config differing from expected (-want+got)\n%s", tt.filename, diff)
			}
		})
	}
}

func Test_ReadHCL_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		filename string
	}{
		{
			filename: "config-nonexistent.hcl",
		},
		{
			filename: "config-invalid.hcl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			t.Parallel()

			if _, err := ReadHCL(testpath(tt.filename)); err == nil {
				t.Errorf("ReadHCL(%q) returned nil error, want non-nil error for invalid configs", tt.filename)
			}
		})
	}
}
