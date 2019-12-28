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
	r := mux.NewRouter()

	// routes
	addRoute(r, "/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("version " + config.Metadata.Version))
	}, conf.UseCORS, http.MethodGet)

	addRoute(r, "/items/sync", itemsHandlers.SyncItems, conf.UseCORS, http.MethodPost)
	addRoute(r, "/items/backup", itemsHandlers.BackupItems, conf.UseCORS, http.MethodPost)

	addRoute(r, "/auth/params", authHandlers.GetParams, conf.UseCORS, http.MethodGet)
	addRoute(r, "/auth/update", authHandlers.UpdateUser, conf.UseCORS, http.MethodPost)
	addRoute(r, "/auth/change_pw", authHandlers.ChangePassword, conf.UseCORS, http.MethodPost)
	addRoute(r, "/auth/sign_in", authHandlers.LoginUser, conf.UseCORS, http.MethodPost)
	addRoute(r, "/auth/sign_in.json", authHandlers.LoginUser, conf.UseCORS, http.MethodPost)
	if !conf.NoReg {
		addRoute(r, "/auth", authHandlers.RegisterUser, conf.UseCORS, http.MethodPost)
	}

	// middleware
	r.Use(func(next http.Handler) http.Handler {
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
		r.Use(mux.CORSMethodMiddleware(r))
	}

	server := http.Server{
		Addr:    conf.Host + ":" + strconv.Itoa(conf.Port),
		Handler: r,
	}
	return server.Handler
}

// addRoute sets up one route on router. If cors is true, then it adds an
// additional CORS handler and adds the OPTIONS method to the route.
func addRoute(router *mux.Router, path string, handler http.HandlerFunc, cors bool, mtds ...string) {
	if cors {
		router.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "https://app.standardnotes.org")
			w.Header().Set("Access-Control-Max-Age", "86400")
			w.Header().Set("Access-Control-Allow-Headers", "*")
			for _, key := range []string{"Access-Token", "Client", "UID"} {
				w.Header().Add("Access-Control-Expose-Headers", key)
			}
		}).Methods(http.MethodOptions)
	}
	router.HandleFunc(path, handler).Methods(mtds...)
}
