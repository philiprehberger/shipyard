package ssh

import "testing"

func TestParseTarget(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input    string
		wantUser string
		wantHost string
		wantPort int
		wantErr  bool
	}{
		{"ubuntu@1.2.3.4", "ubuntu", "1.2.3.4", 22, false},
		{"deploy@example.com:2222", "deploy", "example.com", 2222, false},
		{"root@[::1]:22", "root", "::1", 22, false},
		{"root@[fe80::1]", "root", "fe80::1", 22, false},

		{"", "", "", 0, true},
		{"nouser", "", "", 0, true},
		{"@host", "", "", 0, true},
		{"user@", "", "", 0, true},
		{"user@host:notaport", "", "", 0, true},
		{"user@host:99999", "", "", 0, true},
		{"user@host:0", "", "", 0, true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			got, err := ParseTarget(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v; wantErr = %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if got.User != tc.wantUser || got.Host != tc.wantHost || got.Port != tc.wantPort {
				t.Errorf("got %+v; want {User:%s Host:%s Port:%d}", got, tc.wantUser, tc.wantHost, tc.wantPort)
			}
		})
	}
}
