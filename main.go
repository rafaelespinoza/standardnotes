package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"syscall"

	"github.com/rafaelespinoza/standardfile/api"
	"github.com/rafaelespinoza/standardfile/config"
	"github.com/rafaelespinoza/standardfile/db"
	"github.com/sevlyar/go-daemon"
)

type Args struct {
	signal  bool
	migrate bool
	ver     bool
	cfgPath string
}

var (
	_LoadedConfig = "using flags"
	_Args         = Args{}
	// _Version string will be set by linker
	_Version = "dev"
	// _BuildTime string will be set by linker
	_BuildTime = "N/A"
)

func init() {
	flag.BoolVar(&_Args.signal, "stop", false, `shutdown server`)
	flag.BoolVar(&_Args.migrate, "migrate", false, `perform DB migrations`)
	flag.BoolVar(&_Args.ver, "v", false, `show version`)
	flag.StringVar(&_Args.cfgPath, "c", ".", `config file location`)
	flag.StringVar(&_Args.cfgPath, "config", ".", `config file location`)
}

func main() {
	flag.Parse()
	if err := config.InitConf(_Args.cfgPath); err != nil {
		log.Println(err)
		os.Exit(1)
	}

	conf := config.Conf

	if _Args.ver {
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
        Run in Foreground: ` + strconv.FormatBool(conf.Foreground) + `
        Webserver Port:    ` + strconv.Itoa(conf.Port) + `
        Socket:            ` + socket + `
        DB Path:           ` + conf.DB + `
        Debug:             ` + strconv.FormatBool(conf.Debug))
		return
	}

	if _Args.migrate {
		db.Migrate(conf)
		return
	}

	if conf.Port == 0 {
		conf.Port = 8888
	}

	if conf.Foreground {
		api.Serve(conf)
		return
	}

	daemon.AddCommand(daemon.BoolFlag(&_Args.signal), syscall.SIGTERM, termHandler)

	cntxt := &daemon.Context{
		PidFileName: "pid",
		PidFilePerm: 0644,
		LogFileName: "log",
		LogFilePerm: 0640,
		WorkDir:     "./",
		Umask:       027,
		Args:        nil,
	}

	if len(daemon.ActiveFlags()) > 0 {
		d, err := cntxt.Search()
		if err != nil {
			log.Fatalln("Unable send signal to the daemon:", err)
		}
		log.Println("Stopping server")
		daemon.SendCommands(d)
		return
	}

	d, err := cntxt.Reborn()
	if err != nil {
		log.Fatalln(err)
	}
	if d != nil {
		return
	}
	defer cntxt.Release()

	go api.Serve(conf)

	if err := daemon.ServeSignals(); err != nil {
		log.Println("Error:", err)
	}
}

func termHandler(sig os.Signal) error {
	api.Shutdown()
	return daemon.ErrStop
}
