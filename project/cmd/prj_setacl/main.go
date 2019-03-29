package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"

	ufp "github.com/Donders-Institute/tg-toolset-golang/pkg/filepath"
	ustr "github.com/Donders-Institute/tg-toolset-golang/pkg/strings"
	"github.com/Donders-Institute/tg-toolset-golang/project/internal/acl"
	log "github.com/sirupsen/logrus"
)

// global variables from command-line arguments
var optsBase *string
var optsPath *string
var optsManager *string
var optsContributor *string
var optsWriter *string
var optsViewer *string
var optsTraverse *bool
var optsNthreads *int
var optsForce *bool
var optsVerbose *bool
var optsSilence *bool
var optsFollowLink *bool

// global variables derived in the program
var ppathSym string // the absolute path from the input project number or path, it can be a symlink.
var ppath string    // the referent resolved from ppathSym

// global variable for exit code
var exitcode int

var signalHandled = []os.Signal{
	syscall.SIGABRT,
	syscall.SIGHUP,
	syscall.SIGTERM,
	syscall.SIGINT,
}

func init() {
	optsManager = flag.String("m", "", "specify a comma-separated-list of users for the manager role")
	optsContributor = flag.String("c", "", "specify a comma-separated-list of users for the contributor role")
	optsWriter = flag.String("w", "", "specify a comma-separated-list of users for the writer role")
	optsViewer = flag.String("u", "", "specify a comma-separated-list of users for the viewer role")
	optsTraverse = flag.Bool("t", true, "enable/disable role users to travel through parent directories")
	optsBase = flag.String("d", "/project", "set the root path of project storage")
	optsPath = flag.String("p", "", "set path of a sub-directory in the project folder")
	optsNthreads = flag.Int("n", 4, "set number of concurrent processing threads")
	optsForce = flag.Bool("f", false, "force role setting regardlessly")
	optsVerbose = flag.Bool("v", false, "print `verbosed` messages")
	optsSilence = flag.Bool("s", false, "set to `silence` mode")
	optsFollowLink = flag.Bool("l", false, "`follow` symlinks to set roles on referents")

	flag.Usage = usage

	flag.Parse()

	// set logging
	log.SetOutput(os.Stderr)
	// set logging level
	llevel := log.InfoLevel
	if *optsVerbose {
		llevel = log.DebugLevel
	}
	log.SetLevel(llevel)

	// initialize exitcode to 0
	exitcode = 0
}

func usage() {
	fmt.Printf("\nSetting users' access permission on a given project or a path.\n")
	fmt.Printf("\nUSAGE: %s [OPTIONS] projectId|path\n", os.Args[0])
	fmt.Printf("\nOPTIONS:\n")
	flag.PrintDefaults()
	fmt.Printf("\nEXAMPLES:\n")
	fmt.Printf("\n%s\n", ustr.StringWrap("Adding or setting users 'honlee' and 'edwger' to the 'contributor' role on project 3010000.01", 80))
	fmt.Printf("\n  %s -c honlee,edwger 3010000.01\n", os.Args[0])
	fmt.Printf("\n%s\n", ustr.StringWrap("Adding or setting user 'honlee' to the 'manager' role, and 'edwger' to the 'viewer' role on project 3010000.01", 80))
	fmt.Printf("\n  %s -m honlee -u edwger 3010000.01\n", os.Args[0])
	fmt.Printf("\n%s\n", ustr.StringWrap("Adding or setting users 'honlee' and 'edwger' to the 'contributor' role on a specific path, and allowing the two users to traverse through the parent directories", 80))
	fmt.Printf("\n  %s -c honlee,edwger /project/3010000.01/data_dir\n", os.Args[0])
	fmt.Printf("\n")
}

func main() {

	// this defer function ensures that the os.Exit is called with a proper exitcode which is set
	// before this defer operation is registered.
	defer func() {
		os.Exit(exitcode)
	}()

	// command-line options
	args := flag.Args()

	if len(args) < 1 {
		flag.Usage()
		log.Fatal(fmt.Sprintf("unknown project number: %v", args))
	}

	// map for role specification inputs (commad options)
	roleSpec := make(map[acl.Role]string)
	roleSpec[acl.Manager] = *optsManager
	roleSpec[acl.Writer] = *optsWriter
	roleSpec[acl.Contributor] = *optsContributor
	roleSpec[acl.Viewer] = *optsViewer

	// construct operable map and check duplicated specification
	roles, usersT, err := parseRoles(roleSpec)
	if err != nil {
		log.Fatal(fmt.Sprintf("%s", err))
	}

	// the input argument starts with 7 digits (considered as project number)
	ppathSym = args[0]
	if matched, _ := regexp.MatchString("^[0-9]{7,}", ppathSym); matched {
		ppathSym = filepath.Join(*optsBase, ppathSym, *optsPath)
	} else {
		ppathSym, _ = filepath.Abs(ppathSym)
	}
	// resolve any symlinks on ppathSym to actual path this program should work on.
	ppath, _ = filepath.EvalSymlinks(ppathSym)

	fpinfo, err := ufp.GetFilePathMode(ppath)
	if err != nil {
		log.Fatal(fmt.Sprintf("path not found or unaccessible: %s", ppath))
	}

	// check whether there is a need to set ACL based on the ACL set on ppath.
	roler := acl.GetRoler(*fpinfo)
	if roler == nil {
		log.Fatal(fmt.Sprintf("roler not found for path: %s", fpinfo.Path))
	}
	log.Debug(fmt.Sprintf("%+v", fpinfo))
	rolesNow, err := roler.GetRoles(*fpinfo)
	if err != nil {
		log.Fatal(fmt.Sprintf("%s: %s", err, fpinfo.Path))
	}
	// if there is a new role to set, n will be larger than 0
	n := 0
	for r, users := range roles {
		if _, ok := rolesNow[r]; !ok {
			n++
			break
		}
		ulist := "," + strings.Join(rolesNow[r], ",") + ","
		for _, u := range users {
			if strings.Index(ulist, ","+u+",") < 0 {
				n++
				break
			}
		}
	}
	if n == 0 && !*optsForce {
		log.Warnln("All roles in place, I have nothing to do.")
		os.Exit(0)
	}

	// acquiring operation lock file
	if fpinfo.Mode.IsDir() {
		// acquire lock for the current process
		flock := filepath.Join(ppath, ".prj_setacl.lock")
		if err := ufp.AcquireLock(flock); err != nil {
			log.Fatal(fmt.Sprintf("%s", err))
		}
		defer os.Remove(flock)
	}

	chanS := make(chan os.Signal, 1)
	signal.Notify(chanS, signalHandled...)

	// RoleMap for traverse role
	rolesT := make(map[acl.Role][]string)
	rolesT[acl.Traverse] = usersT

	// set specified user roles
	chanF := ufp.GoFastWalk(ppath, *optsFollowLink, *optsNthreads*4)
	chanOut := goSetRoles(roles, chanF, *optsNthreads)

	// set traverse roles
	chanFt := goPrintOut(chanOut, *optsTraverse, rolesT, *optsNthreads*4)
	chanOutt := goSetRoles(rolesT, chanFt, *optsNthreads)

	// block main until the output is all printed, or a system signal is received
	select {
	case s := <-chanS:
		log.Warnf("Stopped due to received signal: %s\n", s)
		exitcode = int(s.(syscall.Signal))
		runtime.Goexit()
	case <-goPrintOut(chanOutt, false, nil, 0):
		exitcode = 0
		runtime.Goexit()
	}
}

// parseRoles checks the role specification from the caller on the following two things:
//
// 1. The users specified in the roleSpec cannot contain the current user.
//
// 2. The same user id cannot appear twice.
func parseRoles(roleSpec map[acl.Role]string) (map[acl.Role][]string, []string, error) {
	roles := make(map[acl.Role][]string)
	users := make(map[string]bool)

	var usersT []string

	me, _ := user.Current()

	for r, spec := range roleSpec {
		if spec == "" {
			continue
		}
		roles[r] = strings.Split(spec, ",")
		usersT = append(usersT, roles[r]...)
		for _, u := range roles[r] {

			// cannot change the role for the user himself
			if u == me.Username {
				return nil, nil, errors.New("managing yourself is not permitted: " + u)
			}

			// cannot specify the same user name more than once
			if users[u] {
				return nil, nil, errors.New("user specified more than once: " + u)
			}
			users[u] = true
		}
	}
	return roles, usersT, nil
}

// goSetRoles performs actions for setting ACL (defined by roles) on paths provided
// through the chanF channel, in a asynchronous manner. It returns a channel containing
// ACL information of paths on which the ACL setting is correctly applied.
//
// The returned channel can be passed onto the goPrintOut function for displaying the
// results asynchronously.
func goSetRoles(roles acl.RoleMap, chanF chan ufp.FilePathMode, nthreads int) chan acl.RolePathMap {

	// output channel
	chanOut := make(chan acl.RolePathMap)

	// core function of updating ACL on the given file path
	updateACL := func(f ufp.FilePathMode) {
		// TODO: make the roler depends on path
		roler := acl.GetRoler(f)
		log.Debug(fmt.Sprintf("path: %s %s", f.Path, reflect.TypeOf(roler)))

		if roler == nil {
			log.Warn(fmt.Sprintf("roler not found: %s", f.Path))
			return
		}

		if rolesNew, err := roler.SetRoles(f, roles, false, false); err == nil {
			chanOut <- acl.RolePathMap{Path: f.Path, RoleMap: rolesNew}
		} else {
			log.Error(fmt.Sprintf("%s: %s", err, f.Path))
		}
	}

	// launch parallel go routines for setting ACL
	go func() {
		var wg sync.WaitGroup
		wg.Add(nthreads)
		for i := 0; i < nthreads; i++ {
			go func() {
				for f := range chanF {
					log.Debug("process file: " + f.Path)
					updateACL(f)
				}
				wg.Done()
			}()
		}
		wg.Wait()
		close(chanOut)
	}()

	return chanOut
}

// goPrintOut prints out information of paths on which the new ACL has been applied.
//
// Optionally, it also resolves the paths on which the traverse role has to be set.
// The paths resolved for traverse role can be passed onto the goSetRoles function for
// setting the traverse role.
func goPrintOut(chanOut chan acl.RolePathMap, resolvePathForTraverse bool, rolesT map[acl.Role][]string, bufferChanTraverse int) chan ufp.FilePathMode {

	chanFt := make(chan ufp.FilePathMode, bufferChanTraverse)
	go func() {
		counter := 0
		spinner := ustr.NewSpinner()
		for o := range chanOut {
			counter++
			if *optsSilence {
				// print visited directory/path counter
				switch m := counter % 100; m {
				case 1:
					fmt.Printf("\r %s path visited: %d", spinner.Next(), counter)
				default:
					fmt.Printf("\r %s path visited: %d", spinner.Current(), counter)
				}
			} else {
				// the role has been set to the path
				log.Info(fmt.Sprintf("%s", o.Path))
			}

			for r, users := range o.RoleMap {
				log.Debug(fmt.Sprintf("%12s: %s", r, strings.Join(users, ",")))
			}
			// examine the path to see if it is deviated from the ppath from
			// the project storage perspective.  If so, it should be considered for the
			// traverse role settings.
			if resolvePathForTraverse && !acl.IsSameProjectPath(o.Path, ppath) {
				acl.GetPathsForSetTraverse(o.Path, rolesT, &chanFt)
			}
		}
		// enter a newline when using the silence mode
		if *optsSilence && counter != 0 {
			fmt.Printf("\n")
		}
		// examine ppath (and ppathSym if it's not the same as ppath) to resolve possible
		// parents for setting the traverse role.
		if resolvePathForTraverse {
			acl.GetPathsForSetTraverse(ppath, rolesT, &chanFt)
			if ppath != ppathSym {
				acl.GetPathsForSetTraverse(ppathSym, rolesT, &chanFt)
			}
		}
		defer close(chanFt)
	}()

	return chanFt
}
