package remote

import "testing"

func TestArchFromUname(t *testing.T) {
	cases := []struct {
		in         string
		goos, arch string
		wantErr    bool
	}{
		{"Linux x86_64\n", "linux", "amd64", false},
		{"Linux aarch64", "linux", "arm64", false},
		{"Darwin arm64", "darwin", "arm64", false},
		{"Linux armv7l", "linux", "arm", false},
		{"Linux riscv64", "", "", true},
		{"garbage", "", "", true},
	}

	for _, c := range cases {
		goos, arch, err := archFromUname(c.in)

		if c.wantErr {
			if err == nil {
				t.Errorf("archFromUname(%q): expected error", c.in)
			}
			continue
		}

		if err != nil {
			t.Errorf("archFromUname(%q): %v", c.in, err)
			continue
		}
		if goos != c.goos || arch != c.arch {
			t.Errorf("archFromUname(%q) = %s/%s, want %s/%s", c.in, goos, arch, c.goos, c.arch)
		}
	}
}

func TestShellQuote(t *testing.T) {
	if got := shellQuote("a b"); got != "'a b'" {
		t.Errorf("shellQuote(a b) = %s", got)
	}
	if got := shellQuote("it's"); got != `'it'\''s'` {
		t.Errorf("shellQuote(it's) = %s", got)
	}
}
