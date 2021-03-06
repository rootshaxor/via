package via

import (
	"encoding/json"
	"fmt"
	"github.com/mrosset/util/console"
	"github.com/mrosset/util/file"
	"github.com/mrosset/util/human"
	mjson "github.com/mrosset/util/json"
	"path/filepath"
	"sort"
	"time"
)

// Plans proved type for Plan directory type
type Plans struct {
	Path
}

// ConfigFile returns config.json Path
func (p Plans) ConfigFile() Path {
	return p.Expand().Join("config.json")
}

// PlanSlice provides a slice of plans
type PlanSlice []*Plan

// GetPlans returns a Plan slice of all Plan in config.Plans
func GetPlans(config *Config) (PlanSlice, error) {
	pf, err := PlanFiles(config)
	if err != nil {
		return nil, err
	}
	plans := PlanSlice{}
	for _, f := range pf {
		p, _ := ReadPath(f)
		plans = append(plans, p)
	}
	return plans, nil
}

// SortSize returns a copy of this PlanSlice sorted by field Size.
func (ps PlanSlice) SortSize() PlanSlice {
	nps := append(PlanSlice{}, ps...)
	sort.Sort(Size(nps))
	return nps
}

// Print each plans name and size to console
//
// TODO: use template
func (ps PlanSlice) Print() {
	for _, p := range ps {
		console.Println(p.NameVersion(), human.ByteSize(p.Size))
	}
	console.Flush()
}

// Slice returns a slice of plan names
func (ps PlanSlice) Slice() []string {
	s := []string{}
	for _, p := range ps {
		s = append(s, p.Name)
	}
	return s
}

// Contains return true if plan already exists in this PlanSlice slice
func (ps PlanSlice) Contains(plan *Plan) bool {
	for _, p := range ps {
		if p.Name == plan.Name {
			return true
		}
	}
	return false
}

// Expand returns a Plan that has been parsed by go's template
// engine. This provides a form of self referencing json. Where field
// names can be reference from other filed names
func (p *Plan) Expand() *Plan {
	o := new(Plan)
	err := mjson.Parse(o, p)
	if err != nil {
		panic(err)
	}
	return o
}

// revive:disable
// Plan is the plan type used to define plan meta data and build
// instructions
type Plan struct {
	Name          string
	Version       string
	Url           string
	Group         string
	StageDir      string
	Inherit       string
	Cid           string
	BuildInStage  bool
	IsRebuilt     bool
	BuildTime     time.Duration
	Date          time.Time
	Size          int64
	SubPackages   []string
	AutoDepends   []string
	ManualDepends []string
	BuildDepends  []string
	Flags         Flags
	Patch         []string
	Build         []string
	Package       []string
	PostInstall   []string
	Remove        []string
	Files         []string
}

//revive:enable

// Depends returns the PlanSlice Autodepends and ManualDepends as one
// string slice
func (p *Plan) Depends() []string {
	return append(p.AutoDepends, p.ManualDepends...)
}

// NameVersion returns a plans name and version separated by a hyphen
func (p *Plan) NameVersion() string {
	return fmt.Sprintf("%s-%s", p.Name, p.Version)
}

// PlanJSON provides json Marshal interface for Plan
type PlanJSON Plan

func (j *PlanJSON) sortFields() {
	sort.Strings(j.SubPackages)
	sort.Strings(j.Flags)
	sort.Strings(j.Remove)
	sort.Strings(j.AutoDepends)
	sort.Strings(j.ManualDepends)
	sort.Strings(j.BuildDepends)
}

// MarshalJSON provides Marshal interface
func (j PlanJSON) MarshalJSON() ([]byte, error) {
	j.sortFields()
	return json.Marshal(Plan(j))
}

// PlanFiles returns a string slice with the full path of all of all
// plans
func PlanFiles(config *Config) ([]string, error) {
	return filepath.Glob(
		config.Plans.Join("*", "*.json").String(),
	)
}

// FindPlanPath returns the fullpath for a plan by it's given name
func FindPlanPath(config *Config, name string) (Path, error) {
	glob := config.Plans.Join("*", name+".json").String()
	e, err := filepath.Glob(glob)
	if err != nil {
		return "", err
	}
	if len(e) != 1 {
		return "", fmt.Errorf("%s: expected 1 plan found %d", name, len(e))
	}
	return Path(e[0]), nil
}

// NewPlan returns a new Plan that has been initialized
func NewPlan(config *Config, name string) (plan *Plan, err error) {
	path, err := FindPlanPath(config, name)
	if err != nil {
		return nil, err
	}
	plan, err = ReadPath(path.String())
	if err != nil {
		return nil, err
	}
	return plan, nil
}

// ReadPath reads a plan by path and return a Plan
func ReadPath(path string) (plan *Plan, err error) {
	plan = new(Plan)
	err = mjson.Read(plan, path)
	if err != nil {
		return nil, err
	}
	return plan, nil
}

// PackageFile returns the plans tarball name
func PackageFile(config *Config, plan *Plan) string {
	if plan.Cid == "" {
		return fmt.Sprintf("%s-%s-%s.tar.gz", plan.NameVersion(), config.OS, config.Arch)
	}
	return fmt.Sprintf("%s.tar.gz", plan.Cid)
}

// SourceFile return the base name of the plans upstream source
// file/directory
func (p *Plan) SourceFile() string {
	return join(filepath.Base(p.Expand().Url))
}

// PackagePath returns the full path of the plans package file
func PackagePath(config *Config, plan *Plan) string {
	return join(config.Repo.String(), PackageFile(config, plan))
}

// PackageFileExists return true if a plan's package file exists
func PackageFileExists(config *Config, plan *Plan) bool {
	return file.Exists(PackagePath(config, plan))
}

func (p Plan) stageDir() string {
	if p.StageDir != "" {
		return p.StageDir
	}
	return p.NameVersion()
}

// FmtPlans walks all plans and formats it sorting fields
//
// FIXME: this should be renamed to Format and a new Lint function
// created. Lint function should have no side effects just look for
// known style isses. For example we can check that each upstream URL
// is using https and not http
func FmtPlans(config *Config) (err error) {
	files, err := PlanFiles(config)
	if err != nil {
		return err
	}
	for _, f := range files {
		plan, err := ReadPath(f)
		if err != nil {
			err = fmt.Errorf("%s %s", f, err)
			elog.Println(err)
			return err
		}
		// If Group is empty, we can set it
		if plan.Group == "" {
			plan.Group = baseDir(f)
		}
		if verbose {
			console.Println("fmt", plan.Name, plan.Version, plan.IsRebuilt)
		}
		if err := WritePlan(config, plan); err != nil {
			return err
		}
	}
	console.Flush()
	return nil
}
