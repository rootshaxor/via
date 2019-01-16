package via

import (
	"testing"
)

var testConfig = &Config{
	Root:    "/home/mrosset/src/via/pkg/testdata/root",
	Repo:    "testdata/repo",
	DB:      "/home/mrosset/src/via/pkg/testdata/root/db",
	Binary:  "http://localhost:8080/ipfs/",
	Threads: 8,
	IpfsApi: "localhost:5001",
}

func TestConfigExpand(t *testing.T) {
	var (
		c = &Config{
			Branch: "x86_64-via-linux-gnu",
			Prefix: "/usr/local/via",
			Binary: "https://bitbucket.org/strings/publish/raw/{{.Branch}}/repo",
			Env: map[string]string{
				"C_INCLUDE_PATH": "{{.Prefix}}/include",
			},
		}
		expect = "https://bitbucket.org/strings/publish/raw/x86_64-via-linux-gnu/repo"
		got    = ""
	)

	got = c.Expand().Binary
	if got != expect {
		t.Errorf("expected %s got %s", expect, got)
	}

	expect = "/usr/local/via/include"
	got = c.Expand().Env["C_INCLUDE_PATH"]
	if got != expect {
		t.Errorf("expected %s got %s", expect, got)
	}
}

func TestConfig(t *testing.T) {
	if config == nil {
		t.Errorf("config is nil")
	}
}
