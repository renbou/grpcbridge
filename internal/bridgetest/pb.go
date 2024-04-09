package bridgetest

import (
	"sync"

	"google.golang.org/protobuf/reflect/protoregistry"
)

var emptyOnce sync.Once

// EmptyGlobalProtoRegistry empties the global proto registries,
// which is needed for testing to simulate lack of PB information in grpcbridge's global registries,
// as it will be during real use where proto registries are constructed on the fly for each target.
func EmptyGlobalProtoRegistry() {
	emptyOnce.Do(func() {
		protoregistry.GlobalFiles = new(protoregistry.Files)
		protoregistry.GlobalTypes = new(protoregistry.Types)
	})
}
