package via

import (
	"github.com/str1ngs/util/json"
	"path"
)

type Manifest struct {
	Plan  *Plan
	Files []string
	Dirs  []string
}

func ReadManifest(name string) (man *Manifest, err error) {
	man = new(Manifest)
	err = json.Read(man, path.Join(config.DB.Installed(), name, "manifest.json"))
	if err != nil {
		return
	}
	return
}