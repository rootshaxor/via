package via

import (
	"github.com/mrosset/util/file"
	"os"
	"testing"
)

func TestBatchAdd(t *testing.T) {
	var (
		d      = NewBatch(testConfig)
		expect = "hello-2.9"
	)
	d.Add(testPlan)
	got := d.Plans[testPlan.Name].NameVersion()
	if expect != got {
		t.Errorf("expect '%s' got '%s'", expect, got)
	}
}

func TestBatchWalk(t *testing.T) {
	var (
		p, _   = NewPlan(config, "ccache")
		got    = NewBatch(testConfig)
		expect = 1
	)
	got.Walk(p)
	if len(got.Plans) != expect {
		t.Errorf("expect %d depends got %d", expect, len(got.Plans))
	}
}

func testBatchInstall(t *testing.T) {
	var (
		p, _   = NewPlan(config, "ccache")
		got    = NewBatch(testConfig)
		expect = join(testConfig.Repo, "repo", p.PackageFile())
	)
	defer os.RemoveAll(testConfig.Repo)
	defer os.RemoveAll(testConfig.Root)
	defer os.RemoveAll(testConfig.DB.Installed(testConfig))
	got.Walk(p)
	errors := got.Install()
	if len(errors) != 0 {
		t.Error(errors)
	}
	if !file.Exists(expect) {
		t.Errorf("expect: %s got: %v", expect, false)
	}
}
