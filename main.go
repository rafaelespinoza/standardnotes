package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strings"
	"syscall"

	"github.com/rafaelespinoza/standardnotes/internal/api"
	"github.com/rafaelespinoza/standardnotes/internal/config"
	"github.com/sevlyar/go-daemon"
)

// Args is a collection of named variables that are set from CLI flags or the
// config file. Values set from CLI flags override values in the config file.
type Args struct {
	config string
	stop   bool

	daemon  bool
	db      string
	debug   bool
	host    string
	migrate bool
	noReg   bool
	port    int
	socket  string
	useCors bool
}

func init() {
	flag.Usage = func() {
		cmds := make([]string, 0)
		for cmd := range Commands {
			cmds = append(
				cmds,
				fmt.Sprintf("%-20s\t%-40s", cmd, Commands[cmd].description),
			)
		}
		sort.Strings(cmds)
		fmt.Fprintf(flag.CommandLine.Output(), `Usage:
	%s [options] command

Description:

	standardnotes backend API, golang implementation.

	https://github.com/rafaelespinoza/standardnotes

Commands:

	%v
`, _Bin, strings.Join(cmds, "\n\t"))

		printFlagDefaults(flag.CommandLine)
	}

	flag.StringVar(&_Args.config, "config", "./internal/config/standardnotes.json", "config file location")
	// The following flags can also be set with a config file, but are
	// overridden with a CLI flag.
	flag.StringVar(&_Args.db, "db", "sf.db", "path to database (sqlite3) file")
	flag.BoolVar(&_Args.debug, "debug", false, "run server in debug mode")
	flag.StringVar(&_Args.host, "host", "localhost", "server hostname")
	flag.BoolVar(&_Args.noReg, "noreg", false, "disable user registration")
	flag.IntVar(&_Args.port, "port", 8888, "server port")
	flag.StringVar(&_Args.socket, "socket", "", "server socket")
	flag.BoolVar(&_Args.useCors, "cors", false, "use CORS in server")
}

var (
	_LoadedConfig = "using flags"
	// _Args is the shared set named values.
	_Args = Args{}
	// _Bin is the name of the binary.
	_Bin = os.Args[0]
	// _Version will be set by linker.
	_Version = "dev"
	// _BuildTime will be set by linker.
	_BuildTime = "N/A"
)

func initCommand(positionalArgs []string, a *Args) (cmd *Command, err error) {
	if len(positionalArgs) == 0 || positionalArgs[0] == "help" {
		err = flag.ErrHelp
		return
	} else if c, ok := Commands[strings.ToLower(positionalArgs[0])]; !ok {
		err = fmt.Errorf("unknown command %q", positionalArgs[0])
		return
	} else {
		cmd = c
	}

	if data, ierr := ioutil.ReadFile(a.config); ierr != nil {
		err = ierr
		return
	} else if ierr = json.Unmarshal(data, &config.Conf); ierr != nil {
		err = ierr
		return
	}
	if a.useCors {
		config.Conf.UseCORS = true
	}
	if a.debug {
		config.Conf.Debug = true
	}
	if a.db != "" {
		config.Conf.DB = a.db
	} else if config.Conf.DB == "" {
		config.Conf.DB = "sf.db"
	}
	if a.noReg {
		config.Conf.NoReg = true
	}
	if a.port != 0 {
		config.Conf.Port = a.port
	}
	if a.socket != "" {
		config.Conf.Socket = a.socket
	}

	subflags := cmd.setup(a)
	if err = subflags.Parse(positionalArgs[1:]); err != nil {
		return
	}
	return
}

func main() {
	flag.Parse()
	var cmd *Command
	var err error
	if cmd, err = initCommand(flag.Args(), &_Args); cmd == nil {
		// either asked for help or asked for unknown command.
		flag.Usage()
		fmt.Println(err)
		os.Exit(1)
	} else if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if !cmd.api {
		if err := cmd.run(&_Args); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		return
	}

	daemon.AddCommand(
		daemon.BoolFlag(&_Args.stop),
		syscall.SIGTERM,
		func(os.Signal) error {
			api.Shutdown()
			return daemon.ErrStop
		},
	)

	if !_Args.stop && !_Args.daemon {
		// run server in foreground
		api.Serve(config.Conf)
		return
	}

	// run server as background daemon.

	ctx := &daemon.Context{
		PidFileName: "pid",
		PidFilePerm: 0644,
		LogFileName: "log",
		LogFilePerm: 0640,
		WorkDir:     "./",
		Umask:       027,
		Args:        nil,
	}

	if len(daemon.ActiveFlags()) > 0 {
		d, err := ctx.Search()
		if err != nil {
			log.Fatalln("Unable send signal to the daemon:", err)
		}
		log.Println("Stopping server")
		daemon.SendCommands(d)
		return
	}

	if proc, err := ctx.Reborn(); err != nil {
		log.Fatalln(err)
	} else if proc != nil {
		return
	}

	defer ctx.Release()
	go api.Serve(config.Conf)

	if err := daemon.ServeSignals(); err != nil {
		log.Println("Error:", err)
	}
}
