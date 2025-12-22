package driverd

import (
	"context"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
)

func TestIdentityServer_Probe(t *testing.T) {
	tests := []struct {
		name     string
		ready    bool
		expected bool
	}{
		{
			name:     "Ready",
			ready:    true,
			expected: true,
		},
		{
			name:     "Not Ready",
			ready:    false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is, _ := NewIdentityServer("test-driver", "v1.0.0", func() bool { return tt.ready })
			resp, err := is.Probe(context.Background(), &csi.ProbeRequest{})
			if err != nil {
				t.Fatalf("Probe failed: %v", err)
			}
			if resp.Ready == nil {
				t.Fatal("ProbeResponse.Ready is nil")
			}
			if resp.Ready.Value != tt.expected {
				t.Errorf("Expected Ready.Value to be %v, got %v", tt.expected, resp.Ready.Value)
			}
		})
	}
}
