package reflection

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

type namedProtoBundle struct {
	name  string
	proto []byte
}

func hashNamedProtoBundles(bundles []namedProtoBundle) string {
	slices.SortFunc(bundles, func(a, b namedProtoBundle) int {
		return strings.Compare(a.name, b.name)
	})

	h := sha256.New()
	for _, bundle := range bundles {
		h.Write(bundle.proto)
	}

	return hex.EncodeToString(h.Sum(nil))
}

func hashServiceNames(names []protoreflect.FullName) string {
	slices.Sort(names)

	h := sha256.New()
	for _, name := range names {
		h.Write([]byte(name))
	}

	return hex.EncodeToString(h.Sum(nil))
}

// updatePresentDescriptorSet is used by resolver.retrieveDependencies to update the set of available file descriptors.
func updatePresentDescriptorSet(descriptors *descriptorpb.FileDescriptorSet, present map[string]struct{}) {
	for _, fd := range descriptors.File {
		present[fd.GetName()] = struct{}{}
	}
}

// growMissingDescriptorSet is used by resolver.retrieveDependencies to update the set of missing file descriptors.
func growMissingDescriptorSet(descriptors *descriptorpb.FileDescriptorSet, present map[string]struct{}, missing map[string]struct{}) {
	for _, fd := range descriptors.File {
		for _, dep := range fd.Dependency {
			if _, ok := present[dep]; !ok {
				missing[dep] = struct{}{}
			}
		}
	}
}

// shrinkMissingDescriptorSet is used by resolver.retrieveDependencies to remove found file descriptors from the missing set.
func shrinkMissingDescriptorSet(descriptors *descriptorpb.FileDescriptorSet, missing map[string]struct{}) {
	for _, fd := range descriptors.File {
		delete(missing, fd.GetName())
	}
}
