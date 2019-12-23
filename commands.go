package main

import (
	"flag"
	"fmt"
	"runtime"
	"strconv"

	"github.com/rafaelespinoza/standardfile/config"
	"github.com/rafaelespinoza/standardfile/db"
)

// Commands associates a CLI input argument to a Command.
var Commands = map[string]*Command{
	"api":     &_APICommand,
	"db":      &_DBCommand,
	"version": &_VersionCommand,
}

// A Command is the behavior to specify from this CLI.
type Command struct {
	// api says whether or not the command is for running the API server. It
	// tells main to handle it specially.
	api bool
	// description should summarize the command in < 1 line.
	description string
	// run is a wrapper function that selects the necessary command line inputs,
	// executes the command and returns any errors.
	run func(a *Args) error
	// setup should prepare Args for interpretation by using the pointer to Args
	// with the returned flag set.
	setup func(a *Args) *flag.FlagSet
}

var (
	_APICommand = Command{
		api:         true,
		description: "run, manage api server",
		run: func(a *Args) error {
			// special handling from main, just pass through.
			return nil
		},
		setup: func(a *Args) *flag.FlagSet {
			const name = "api"
			flags := flag.NewFlagSet(name, flag.ExitOnError)
			flags.BoolVar(&a.daemon, "d", false, "run server in background")
			flags.BoolVar(&a.stop, "stop", false, "shutdown server")
			flags.Usage = func() {
				fmt.Printf(`Usage: %s %s [-d] [-stop]

	Run and manager the api server. By default it runs in the foreground. Pass
	the -d flag to run it as a background daemon. Pass -stop to shut it down.
				`, _Bin, name)
				printFlagDefaults(flags)
			}
			return flags
		},
	}

	_DBCommand = Command{
		description: "perform database tasks",
		run: func(a *Args) error {
			db.Migrate(config.Conf)
			return nil
		},
		setup: func(a *Args) *flag.FlagSet {
			const name = "db"
			flags := flag.NewFlagSet(name, flag.ExitOnError)
			flags.BoolVar(&a.migrate, "migrate", false, "perform DB migrations")
			flags.Usage = func() {
				fmt.Printf(`Usage: %s %s [-migrate]

	Do database tasks. Pass -migrate to perform a migration.
				`, _Bin, name)
				printFlagDefaults(flags)
			}
			return flags
		},
	}

	_VersionCommand = Command{
		description: "show version information and other metadata",
		run: func(a *Args) error {
			conf := config.Conf

			socket := "no"
			if len(conf.Socket) > 0 {
				socket = conf.Socket
			}
			fmt.Println(`        Version:           ` + _Version + `
        Built:             ` + _BuildTime + `
        Go Version:        ` + runtime.Version() + `
        OS/Arch:           ` + runtime.GOOS + "/" + runtime.GOARCH + `
        Loaded Config:     ` + _LoadedConfig + `
        No Registrations:  ` + strconv.FormatBool(conf.NoReg) + `
        CORS Enabled:      ` + strconv.FormatBool(conf.UseCORS) + `
        Run in Foreground: ` + strconv.FormatBool(a.daemon) + `
        Webserver Port:    ` + strconv.Itoa(conf.Port) + `
        Socket:            ` + socket + `
        DB Path:           ` + conf.DB + `
        Debug:             ` + strconv.FormatBool(conf.Debug))

			return nil
		},
		setup: func(a *Args) *flag.FlagSet {
			const name = "version"
			flags := flag.NewFlagSet(name, flag.ExitOnError)
			flags.Usage = func() {
				fmt.Printf(`Usage: %s %s

	Print out configuration metadata and version information.
				`, _Bin, name)
				printFlagDefaults(flags)
			}
			return flags
		},
	}
)

func printFlagDefaults(f *flag.FlagSet) {
	fmt.Printf("\nFlags:\n\n")
	f.PrintDefaults()
}
