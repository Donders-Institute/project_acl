package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/acl"

	log "github.com/sirupsen/logrus"
)

var optsPath *string
var nthreads *int
var verbose *bool

func init() {
	optsPath = flag.String("d", "/project", "root path of project storage")
	nthreads = flag.Int("n", 4, "number of concurrent processing threads")
	verbose = flag.Bool("v", false, "print debug messages")

	flag.Usage = usage
	flag.Parse()

	// set logging
	log.SetOutput(os.Stderr)
	// set logging level
	llevel := log.InfoLevel
	if *verbose {
		llevel = log.DebugLevel
	}
	log.SetLevel(llevel)
}

func usage() {
	fmt.Printf("\nGetting an overview on users' project roles.\n")
	fmt.Printf("\nUSAGE: %s [OPTIONS] userID\n", os.Args[0])
	fmt.Printf("\nOPTIONS:\n")
	flag.PrintDefaults()
	fmt.Printf("\n")
}

func main() {

	// command-line arguments
	args := flag.Args()

	if len(args) < 1 {
		flag.Usage()
		log.Fatal(fmt.Sprintf("unknown user id: %v", args))
	}

	uid := args[0]

	dirs := make(chan string, *nthreads*2)
	members := make(chan projectRole)

	wg := sync.WaitGroup{}
	for i := 0; i < *nthreads; i++ {
		wg.Add(1)
		go findUserMember(uid, dirs, members, &wg)
	}

	// go routine to list all directories in the /project folder
	go func(path string) {
		// close the dirs channel on exit
		defer close(dirs)

		start := time.Now()

		infoDirs, err := ioutil.ReadDir(path)
		if err != nil {
			log.Errorf("cannot get content of path: %s", path)
			return
		}

		elapsed := time.Since(start)

		log.Debugf("project listing took %s\n", elapsed)

		for _, infoDir := range infoDirs {
			dirs <- filepath.Join(path, infoDir.Name())
		}

	}(*optsPath)

	// go routine printing user's membership.
	go func() {
		for member := range members {
			fmt.Printf("%s: %s\n", member.projectID, member.role)
		}
	}()

	wg.Wait()

	// close up members channel
	close(members)
}

type projectRole struct {
	projectID string
	role      acl.Role
}

func findUserMember(uid string, dirs chan string, members chan projectRole, wg *sync.WaitGroup) {

	for dir := range dirs {

		log.Debugf("finding user member for %s in %s", uid, dir)

		// get all members of the dir
		runner := acl.Runner{
			RootPath:   dir,
			FollowLink: true,
			SkipFiles:  true,
			Nthreads:   1,
		}

		chanOut, err := runner.GetRoles(false)
		if err != nil {
			log.Errorf("cannot get role for path %s: %s", dir, err)
			return
		}

		// feed members channel if the user in question is in the list.
		for o := range chanOut {
			for r, users := range o.RoleMap {
				if r == acl.System {
					continue
				}
				for _, u := range users {
					if u == uid {
						members <- projectRole{
							projectID: filepath.Base(dir),
							role:      r,
						}
						break
					}
					continue
				}
			}
		}
	}

	wg.Done()
}
