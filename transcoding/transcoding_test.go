package transcoding

import (
	"os"
	"testing"

	"github.com/renbou/grpcbridge/internal/bridgetest"
)

func TestMain(m *testing.M) {
	// Needed to ensure that parsing relies only on the target's registry.
	bridgetest.EmptyGlobalProtoRegistry()

	os.Exit(m.Run())
}
