package meshsync

import "testing"

// TestBrokerHost verifies that the host part of a broker URL is extracted
// correctly regardless of scheme, credentials (userinfo) or port.
func TestBrokerHost(t *testing.T) {
	testCases := []struct {
		name      string
		input     string
		expected  string
		expectErr bool
	}{
		{
			name:     "host and port without scheme",
			input:    "host:4222",
			expected: "host",
		},
		{
			name:     "scheme prefixed",
			input:    "nats://host:4222",
			expected: "host",
		},
		{
			name:     "token as userinfo",
			input:    "nats://TOKEN@host:4222",
			expected: "host",
		},
		{
			name:     "user and password with tls scheme",
			input:    "tls://user:pass@host:4222",
			expected: "host",
		},
		{
			name:     "ipv6 literal",
			input:    "nats://[::1]:4222",
			expected: "::1",
		},
		{
			name:     "bare host",
			input:    "host",
			expected: "host",
		},
		{
			name:      "empty string",
			input:     "",
			expectErr: true,
		},
		{
			name:      "scheme without host",
			input:     "nats://",
			expectErr: true,
		},
		{
			name:      "invalid character in host",
			input:     "nats://ho st:4222",
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			host, err := brokerHost(tc.input)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error for input %q, got host %q", tc.input, host)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tc.input, err)
			}
			if host != tc.expected {
				t.Errorf("expected host %q for input %q, got %q", tc.expected, tc.input, host)
			}
		})
	}
}
