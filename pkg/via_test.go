package via

import (
        "bytes"
        "os"
        "os/exec"
        "reflect"
        "testing"
)

const ExpectGotFmt = "%s: expect '%v' got '%v'"

func init() {
        Verbose(false)
}

type test struct {
        Name   string
        Expect interface{}
        Got    interface{}
}

type tests []test

func (ts tests) equals(t *testing.T) {
        for _, test := range ts {
                test.equals(t)
        }
}

func (vt test) equals(t *testing.T) bool {
        if !reflect.DeepEqual(vt.Expect, vt.Got) {
                t.Errorf(ExpectGotFmt, vt.Name, vt.Expect, vt.Got)
                return false
        }
        return true
}

func TestTestType(t *testing.T) {
        test{
                Expect: "foo",
                Got:    "foo",
        }.equals(t)
}

func TestReadelf(t *testing.T) {
        t.Parallel()
        var (
                out = "testdata/a.out"
        )
        defer os.Remove(out)
        bin, err := exec.LookPath("gcc")
        if err != nil {
                t.Fatal(err)
        }
        gcc := &exec.Cmd{
                Path:  bin,
                Args:  []string{"gcc", "-o", out, "-xc", "-"},
                Stdin: bytes.NewBufferString("int main() {}\n"),
        }
        if err := gcc.Start(); err != nil {
                t.Fatal(err)
        }
        if err = Readelf(out); err != nil {
                t.Error(err)
        }
}
