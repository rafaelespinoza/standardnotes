package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"syscall"

	"github.com/sevlyar/go-daemon"
)

type Args struct {
	signal  bool
	migrate bool
	ver     bool
	cfgPath string
}

var (
	_Work         chan bool
	_LoadedConfig = "using flags"
	_Args         = Args{}
	// _Version string will be set by linker
	_Version = "dev"
	// _BuildTime string will be set by linker
	_BuildTime = "N/A"
)

func init() {
	_Work = make(chan bool)
	flag.BoolVar(&_Args.signal, "stop", false, `shutdown server`)
	flag.BoolVar(&_Args.migrate, "migrate", false, `perform DB migrations`)
	flag.BoolVar(&_Args.ver, "v", false, `show version`)
	flag.StringVar(&_Args.cfgPath, "c", ".", `config file location`)
	flag.StringVar(&_Args.cfgPath, "config", ".", `config file location`)
}

func main() {
	flag.Parse()
	if err := initConf(_Args.cfgPath, &_Config); err != nil {
		log.Println(err)
		os.Exit(1)
	}

	if _Args.ver {
		socket := "no"
		if len(_Config.Socket) > 0 {
			socket = _Config.Socket
		}
		fmt.Println(`        Version:           ` + _Version + `
        Built:             ` + _BuildTime + `
        Go Version:        ` + runtime.Version() + `
        OS/Arch:           ` + runtime.GOOS + "/" + runtime.GOARCH + `
        Loaded Config:     ` + _LoadedConfig + `
        No Registrations:  ` + strconv.FormatBool(_Config.NoReg) + `
        CORS Enabled:      ` + strconv.FormatBool(_Config.UseCORS) + `
        Run in Foreground: ` + strconv.FormatBool(_Config.Foreground) + `
        Webserver Port:    ` + strconv.Itoa(_Config.Port) + `
        Socket:            ` + socket + `
        DB Path:           ` + _Config.DB + `
        Debug:             ` + strconv.FormatBool(_Config.Debug))
		return
	}

	if _Args.migrate {
		Migrate(_Config)
		return
	}

	if _Config.Port == 0 {
		_Config.Port = 8888
	}

	if _Config.Foreground {
		Worker(_Config)
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

	go Worker(_Config)

	if err := daemon.ServeSignals(); err != nil {
		log.Println("Error:", err)
	}
}

func termHandler(sig os.Signal) error {
	close(_Work)
	return daemon.ErrStop
}
