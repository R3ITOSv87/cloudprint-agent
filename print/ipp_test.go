package print

import (
	"testing"
)

func TestParseIPPURI(t *testing.T) {
	tests := []struct {
		input   string
		host    string
		port    int
		wantErr bool
	}{
		{"ipp://192.168.1.50:631/ipp/print", "192.168.1.50", 631, false},
		{"ipp://printer.local/ipp/print", "printer.local", 631, false},
		{"ipp://192.168.1.100:9631/printers/HP", "192.168.1.100", 9631, false},
		{"ipps://secure-printer.local:443/ipp/print", "secure-printer.local", 443, false},
		{"192.168.1.50:631/ipp/print", "192.168.1.50", 631, false},
	}

	for _, tt := range tests {
		uri := tt.input
		// normalize as PrintFile does
		if len(uri) > 0 && uri[0] != 'i' {
			uri = "ipp://" + uri
		}
		host, port, httpURL, err := parseIPPURI(uri)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseIPPURI(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if err != nil {
			continue
		}
		if host != tt.host {
			t.Errorf("parseIPPURI(%q) host = %q, want %q", tt.input, host, tt.host)
		}
		if port != tt.port {
			t.Errorf("parseIPPURI(%q) port = %d, want %d", tt.input, port, tt.port)
		}
		if httpURL == "" {
			t.Errorf("parseIPPURI(%q) httpURL is empty", tt.input)
		}
	}
}

func TestProbeIPPUnreachable(t *testing.T) {
	// 192.0.2.1 is TEST-NET, guaranteed unreachable
	name, reachable := ProbeIPP("ipp://192.0.2.1:631/ipp/print")
	if reachable {
		t.Error("expected unreachable printer to return reachable=false")
	}
	if name != "" {
		t.Errorf("expected empty name for unreachable printer, got %q", name)
	}
}

func TestProbeIPPEmptyURI(t *testing.T) {
	name, reachable := ProbeIPP("")
	if reachable {
		t.Error("empty URI should return reachable=false")
	}
	_ = name
}
