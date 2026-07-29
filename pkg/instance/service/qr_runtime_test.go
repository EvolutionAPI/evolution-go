package instance_service

import (
	"errors"
	"testing"
)

func TestQrNeedsNewRuntime(t *testing.T) {
	cases := []struct {
		name        string
		exists      bool
		loggedIn    bool
		wantStart   bool
		wantErrorIs error
	}{
		{name: "no client at all starts one", exists: false, wantStart: true},
		{name: "client waiting for QR reuses it", exists: true, wantStart: false},
		{name: "logged in client is never restarted", exists: true, loggedIn: true, wantErrorIs: ErrSessionAlreadyLoggedIn},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start, err := qrNeedsNewRuntime(tc.exists, tc.loggedIn)
			if !errors.Is(err, tc.wantErrorIs) {
				t.Fatalf("error = %v, want %v", err, tc.wantErrorIs)
			}
			if start != tc.wantStart {
				t.Fatalf("start = %v, want %v", start, tc.wantStart)
			}
		})
	}
}
