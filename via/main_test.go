package main

import (
	"os"
	"testing"
)

var clean = os.Args

func TestBuild(t *testing.T) {
	args := []string{"build", "ccache"}
	os.Args = append(os.Args, args...)
	main()
}
