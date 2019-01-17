package via

import (
	"compress/gzip"
	"fmt"
	"github.com/mrosset/gurl"
	"github.com/mrosset/util/console"
	"github.com/mrosset/util/file"
	"github.com/mrosset/util/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
)

var (
	client  = new(http.Client)
	verbose = false
	elog    = log.New(os.Stderr, "", log.Lshortfile)
	lfmt    = "%-20.20s %v\n"
	debug   = false
	expand  = os.ExpandEnv
	update  = false
	deps    = false
)

func Root(s string) {
	config.Root = s
}

func Verbose(b bool) {
	verbose = b
}

func Deps(b bool) {
	deps = b
}

func Update(b bool) {
	update = b
}

func Debug(b bool) {
	debug = b
}

func DownloadSrc(plan *Plan) (err error) {
	if file.Exists(plan.SourcePath()) && !update {
		return nil
	}
	fmt.Printf(lfmt, "download", plan.NameVersion())
	eurl := plan.Expand().Url
	u, err := url.Parse(eurl)
	if err != nil {
		return err
	}
	switch u.Scheme {
	case "ftp":
		wget(cache.Sources(), eurl)
	case "http", "https":
		return gurl.Download(cache.Sources(), eurl)
	default:
		return fmt.Errorf("%s: URL scheme is not supported", u.Scheme)
	}
	return nil
}

// Stages the downloaded source in via's cache directory
// the stage only happens once unless BuilInStage is used
func Stage(plan *Plan) (err error) {
	if plan.Url == "" || file.Exists(plan.GetStageDir()) {
		// nothing to stage
		return nil
	}
	fmt.Printf(lfmt, "stage", plan.NameVersion())
	u, err := url.Parse(plan.Expand().Url)
	if err != nil {
		elog.Println(err)
		return err
	}
	if u.Scheme == "git" {
		fmt.Println(cache.Stages())
		fmt.Println(plan.SourcePath())
		cmd := exec.Command("git", "clone", plan.SourcePath(), plan.SourceFile())
		cmd.Dir = cache.Stages()
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
		goto ret
	}
	switch filepath.Ext(plan.SourceFile()) {
	case ".zip":
		unzip(cache.Stages(), plan.SourcePath())
	default:
		GNUUntar(cache.Stages(), plan.SourcePath())
	}
ret:
	fmt.Printf(lfmt, "patch", plan.NameVersion())
	if err := doCommands(join(cache.Stages(), plan.stageDir()), plan.Patch); err != nil {
		return err
	}
	return
}

// Calls each shell command in the plans Build field.
func Build(config *Config, plan *Plan) (err error) {
	var (
		build = plan.Build
	)
	if file.Exists(plan.PackagePath(config)) {
		fmt.Printf("FIXME: (short flags)  package %s exists building anyways.\n", plan.PackagePath(config))
	}
	for _, p := range plan.BuildDepends {
		if IsInstalled(config, p) {
			continue
		}
		dp, err := NewPlan(p)
		if err != nil {
			return err
		}
		if err := NewInstaller(config, dp).Install(); err != nil {
			return err
		}
	}
	flags := append(config.Flags, plan.Flags...)
	os.MkdirAll(plan.BuildDir(), 0755)
	// Parent plan Build is run first this plans is added at the end.
	if plan.Inherit != "" {
		parent, _ := NewPlan(plan.Inherit)
		build = append(parent.Build, plan.Build...)
		flags = append(flags, parent.Flags...)
	}
	os.Setenv("SRCDIR", plan.GetStageDir())
	os.Setenv("Flags", expand(flags.String()))
	err = doCommands(plan.BuildDir(), build)
	if err != nil {
		return fmt.Errorf("%s in %s", err.Error(), plan.BuildDir())
	}
	return nil
}

func doCommands(dir string, cmds []string) (err error) {
	for i, j := range cmds {
		if debug {
			elog.Println(i, j)
		}
		cmd := exec.Command("bash", "-c", j)
		cmd.Dir = dir
		cmd.Stdin = os.Stdin
		if verbose {
			cmd.Stdout = os.Stdout
		}
		cmd.Stderr = os.Stderr
		if debug {
			fmt.Println(cmd.Args)
		}
		err = cmd.Run()
		if err != nil {
			elog.Printf("%s: %s\n", j, err)
			return err
		}
	}
	return nil
}

func Package(bdir string, plan *Plan) (err error) {
	var (
		pack = plan.Package
	)
	pdir := join(cache.Packages(), plan.NameVersion())
	if bdir == "" {
		bdir = join(cache.Builds(), plan.NameVersion())
	}
	if plan.BuildInStage {
		bdir = join(cache.Stages(), plan.stageDir())
	}
	if file.Exists(pdir) {
		err := os.RemoveAll(pdir)
		if err != nil {
			return err
		}
	}
	err = os.Mkdir(pdir, 0755)
	if err != nil {
		elog.Println(err)
		return err
	}
	os.Setenv("PKGDIR", pdir)
	if plan.Inherit != "" {
		parent, _ := NewPlan(plan.Inherit)
		pack = append(parent.Package, plan.Package...)
	}
	err = doCommands(bdir, pack)
	if err != nil {
		return err
	}
	for _, j := range plan.SubPackages {
		sub, err := NewPlan(j)
		if err != nil {
			return err
		}
		if err = Package(bdir, sub); err != nil {
			return err
		}
	}
	err = CreatePackage(plan)
	if err != nil {
		return (err)
	}
	plan.Cid, err = IpfsAdd(config, Path(plan.PackagePath(config)))
	if err != nil {
		return err
	}
	plan.IsRebuilt = true
	return plan.Save()
	/*
		err = CreatePackage(plan)
		if err != nil {
			return err
		}
		return Sign(plan)
	*/
}

func CreatePackage(plan *Plan) (err error) {
	pfile := plan.PackagePath(config)
	os.MkdirAll(filepath.Dir(pfile), 0755)
	fd, err := os.Create(pfile)
	if err != nil {
		elog.Println(err)
		return err
	}
	defer fd.Close()
	gz := gzip.NewWriter(fd)
	defer gz.Close()
	return Tarball(gz, plan)
}

// Updates each plans Oid to the Oid of the tarball in publish git repo
// this function should never be used in production. It's used for making sure
// the plans Oid match the git repo's Oid
func SyncHashs(config *Config) {
	plans, _ := GetPlans()
	for _, p := range plans {
		if file.Exists(p.PackagePath(config)) {
			p.Cid, _ = HashOnly(config, Path(p.PackagePath(config)))
			p.Save()
			log.Println(p.Cid, p.Name)
		}
	}
}

func OInstall(config *Config, name string) (err error) {
	plan, err := NewPlan(name)
	if err != nil {
		elog.Println(name, err)
		return
	}
	if IsInstalled(config, name) {
		fmt.Printf("FIXME: (short flags) package %s installed upgrading anyways.\n", plan.NameVersion())
		err := Remove(config, name)
		if err != nil {
			return err
		}
	}
	db := filepath.Join(config.DB.Installed(), plan.Name)
	if file.Exists(db) {
		return fmt.Errorf("%s is already installed", name)
	}
	cid, err := HashOnly(config, Path(plan.PackagePath(config)))
	if err != nil {
		elog.Println(err)
		return (err)
	}
	if cid != plan.Cid {
		return fmt.Errorf("%s Plans CID does not match tarballs got %s", plan.NameVersion(), cid)
	}
	man, err := ReadPackManifest(plan.PackagePath(config))
	if err != nil {
		elog.Println(err)
		return err
	}
	errs := conflicts(config, man)
	if len(errs) > 0 {
		//return errs[0]
		for _, e := range errs {
			elog.Println(e)
		}
	}
	fd, err := os.Open(plan.PackagePath(config))
	if err != nil {
		elog.Println(err)
		return
	}
	defer fd.Close()
	gz, err := gzip.NewReader(fd)
	if err != nil {
		return
	}
	defer gz.Close()
	os.MkdirAll(config.Root, 0755)
	err = Untar(config.Root, gz)
	if err != nil {
		elog.Println(err)
		return err
	}
	err = os.MkdirAll(db, 0755)
	if err != nil {
		elog.Println(err)
		return err
	}
	man.Cid = plan.Cid
	return json.Write(man, join(db, "manifest.json"))
}

func PostInstall(plan *Plan) (err error) {
	return doCommands("/", append(plan.PostInstall, config.PostInstall...))
}

func Remove(config *Config, name string) (err error) {
	if !IsInstalled(config, name) {
		err = fmt.Errorf("%s is not installed.", name)
		elog.Println(err)
		return err
	}

	man, err := ReadManifest(name)
	if err != nil {
		return err
	}
	for _, f := range man.Files {
		fpath := join(config.Root, f)
		err = os.Remove(fpath)
		if err != nil {
			elog.Println(f, err)
		}
	}

	return os.RemoveAll(join(config.DB.Installed(), name))
}

func BuildDeps(config *Config, plan *Plan) (err error) {
	deps := append(plan.AutoDepends, plan.ManualDepends...)
	for _, d := range deps {
		if IsInstalled(config, d) {
			continue
		}
		p, _ := NewPlan(d)
		if file.Exists(p.PackagePath(config)) {
			if err := NewInstaller(config, p).Install(); err != nil {
				return err
			}
			continue
		}
		fmt.Println("building", d, "for", plan.NameVersion())
		err := BuildDeps(config, p)
		if err != nil {
			elog.Println(err)
			return err
		}
	}
	err = BuildSteps(config, plan)
	if err != nil {
		return err
	}
	return NewInstaller(config, plan).Install()
}

// Run all of the functions required to build a package
func BuildSteps(config *Config, plan *Plan) (err error) {
	if file.Exists(plan.PackageFile()) {
		return fmt.Errorf("package %s exists", plan.PackageFile())
	}
	if err := DownloadSrc(plan); err != nil {
		elog.Println(err)
		return err
	}
	if err := Stage(plan); err != nil {
		elog.Println(err)
		return err
	}
	fmt.Printf(lfmt, "build", plan.NameVersion())
	if err := Build(config, plan); err != nil {
		elog.Println(err)
		return err
	}
	fmt.Printf(lfmt, "package", plan.NameVersion())
	if err := Package("", plan); err != nil {
		elog.Println(err)
		return err
	}
	return nil
}

var (
	rexName   = regexp.MustCompile("[A-Za-z]+")
	rexTruple = regexp.MustCompile("[0-9]+.[0-9]+.[0-9]+")
	rexDouble = regexp.MustCompile("[0-9]+.[0-9]+")
)

// Creates a new plan from a given Url
func Create(url, group string) (err error) {
	var (
		xfile   = filepath.Base(url)
		name    = rexName.FindString(xfile)
		triple  = rexTruple.FindString(xfile)
		double  = rexDouble.FindString(xfile)
		version string
	)
	switch {
	case triple != "":
		version = triple
	case double != "":
		version = double
	default:
		return fmt.Errorf("regex fail for %s", xfile)
	}
	plan := &Plan{Name: name, Version: version, Url: url, Group: group}
	plan.Inherit = "gnu"
	if file.Exists(plan.Path()) {
		return fmt.Errorf("%s already exists", plan.Path())
	}
	return plan.Save()
}

func IsInstalled(config *Config, name string) bool {
	return file.Exists(join(config.DB.Installed(), name))
}

func refactor(plan *Plan) {
	if len(plan.SubPackages) > 0 {
		for _, j := range plan.SubPackages {
			s, _ := NewPlan(j)
			if s.Version == plan.Version {
				continue
			}
			s.Version = plan.Version
			s.Save()
			fmt.Println(s.Name)

		}
	}
}

func Lint() (err error) {
	e, err := PlanFiles()
	if err != nil {
		return err
	}
	for _, j := range e {
		plan, err := ReadPath(j)
		if err != nil {
			err = fmt.Errorf("%s %s", j, err)
			elog.Println(err)
			return err
		}
		// If Group is empty, we can set it
		if plan.Group == "" {
			plan.Group = baseDir(j)
		}
		if verbose {
			console.Println("lint", plan.Name, plan.Version, plan.IsRebuilt)
		}
		sort.Strings(plan.SubPackages)
		sort.Strings(plan.Flags)
		sort.Strings(plan.Remove)
		sort.Strings(plan.AutoDepends)
		sort.Strings(plan.ManualDepends)
		sort.Strings(plan.BuildDepends)
		refactor(plan)
		err = plan.Save()
		if err != nil {
			elog.Println(err)
			return err
		}
	}
	console.Flush()
	return nil
}

func fatal(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
func Clean(name string) error {
	plan, err := NewPlan(name)
	if err != nil {
		return err
	}
	fmt.Printf(lfmt, "clean", plan.NameVersion())
	dir := join(cache.Builds(), plan.NameVersion())
	if err = os.RemoveAll(dir); err != nil {
		return err
	}

	if plan.BuildInStage {
		dir = join(cache.Stages(), plan.stageDir())
		return os.RemoveAll(dir)
	}
	return nil
}

func PlanFiles() ([]string, error) {
	return filepath.Glob(join(config.Plans, "*", "*.json"))
}

func conflicts(config *Config, man *Plan) (errs []error) {
	for _, f := range man.Files {
		fpath := join(config.Root, f)
		if file.Exists(fpath) {
			errs = append(errs, fmt.Errorf("%s already exists.", f))
		}
	}
	return errs
}

func GetConfig() *Config {
	return config
}
