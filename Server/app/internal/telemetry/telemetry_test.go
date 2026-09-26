package telemetry

import (
	"context"
	"testing"
)

// Init must succeed with whatever OTel SDK go.mod pins — a resource schema
// mismatch here takes down every service at boot.
func TestInitSucceeds(t *testing.T) {
	shutdown, err := Init(context.Background(), "test-service", "")
	if err != nil {
		t.Fatalf("telemetry init: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}
