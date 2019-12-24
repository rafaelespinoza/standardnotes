package api

import (
	"fmt"
	"io"
	"net"

	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/rafaelespinoza/standardfile/config"
	"github.com/rafaelespinoza/standardfile/db"
	"github.com/rafaelespinoza/standardfile/logger"
)

var _Work chan bool

func init() {
	_Work = make(chan bool)
}

// Shutdown closes the server's internal channel.
func Shutdown() {
	close(_Work)
}

// Serve is the main workhorse of this package. It maps request routes to
// handlers and listens on the configured socket.
func Serve(cfg config.Config) {
	db.Init(cfg.DB)
	log.Println("Started StandardFile Server", config.Metadata.Version)
	log.Println("Loaded config:", config.Metadata.LoadedConfig)

	if cfg.Debug {
		log.Println("Debug on")
	}

	router := makeRouter(cfg)

	defer func(cfg config.Config) {
		if len(cfg.Socket) != 0 {
			os.Remove(cfg.Socket)
		}
	}(cfg)

	// listen
	go func(r http.Handler, cfg config.Config) {
		if len(cfg.Socket) == 0 {
			log.Println("Listening on port " + strconv.Itoa(cfg.Port))
			err := http.ListenAndServe(":"+strconv.Itoa(cfg.Port), r)
			if err != nil {
				log.Println(err)
			}
		} else {
			os.Remove(cfg.Socket)
			unixListener, err := net.Listen("unix", cfg.Socket)
			if err != nil {
				panic(err)
			}
			server := http.Server{
				Handler: r,
			}
			log.Println("Listening on socket " + cfg.Socket)
			server.Serve(unixListener)
		}
	}(
		router,
		cfg,
	)
	<-_Work
	log.Println("Server stopped")
	os.Exit(0)
}

// makeRouter sets up an http.Server with request handlers.
func makeRouter(conf config.Config) http.Handler {
	router := mux.NewRouter()

	// routes
	router.HandleFunc("/", Dashboard).Methods(http.MethodGet)

	router.HandleFunc("/items/sync", itemsHandlers.SyncItems).Methods(http.MethodPost)
	router.HandleFunc("/items/backup", itemsHandlers.BackupItems).Methods(http.MethodPost)

	router.HandleFunc("/auth/params", authHandlers.GetParams).Methods(http.MethodGet)
	router.HandleFunc("/auth/update", authHandlers.UpdateUser).Methods(http.MethodPost)
	router.HandleFunc("/auth/change_pw", authHandlers.ChangePassword).Methods(http.MethodPost)
	router.HandleFunc("/auth/sign_in", authHandlers.LoginUser).Methods(http.MethodPost)
	router.HandleFunc("/auth/sign_in.json", authHandlers.LoginUser).Methods(http.MethodPost)
	if !conf.NoReg {
		router.HandleFunc("/auth", authHandlers.RegisterUser).Methods(http.MethodPost)
	}

	// middleware
	router.Use(func(next http.Handler) http.Handler {
		return handlers.CustomLoggingHandler(
			os.Stdout,
			next,
			func(w io.Writer, p handlers.LogFormatterParams) {
				fmt.Fprintf(
					w,
					"%s %d %s %q %d %d\n",
					p.TimeStamp.UTC().Format(logger.TimeFormat),
					p.StatusCode,
					p.Request.Method,
					p.Request.RequestURI,
					p.Request.ContentLength,
					p.Size,
				)
			},
		)
	})
	if conf.UseCORS {
		router.Use(mux.CORSMethodMiddleware(router))
	}

	server := http.Server{
		Addr:    conf.Host + ":" + strconv.Itoa(conf.Port),
		Handler: router,
	}
	return server.Handler
}
