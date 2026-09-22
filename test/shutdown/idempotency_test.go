package shutdown_test

import (
	"NeoNect/test/harness"
	"context"
	"testing"
)

func TestShutdown_Idempotency(t *testing.T) {
	h := harness.Setup(t)

	// Test calling WebSocketManager.Shutdown() directly multiple times
	h.App.Handlers.WSManager.Shutdown()
	h.App.Handlers.WSManager.Shutdown()
	h.App.Handlers.WSManager.Shutdown()

	// Test calling App.Close() multiple times
	ctx := context.Background()
	h.App.Close(ctx)
	h.App.Close(ctx)

	// Test mixing them
	h.App.Handlers.WSManager.Shutdown()

	// If it doesn't panic, it passes!
}
