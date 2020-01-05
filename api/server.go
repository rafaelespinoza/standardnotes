package api

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/rafaelespinoza/standardfile/config"
	"github.com/rafaelespinoza/standardfile/logger"
)

var _Server *server

type server struct {
	conf   config.Config
	http   http.Server
	routes []string
	work   chan bool
}

// newServer initializes a server with request handlers.
func newServer(conf config.Config) (serv *server, err error) {
	r := mux.NewRouter()

	// routes
	addRoute(r, "/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("version " + config.Metadata.Version))
	}, conf.UseCORS, http.MethodGet)

	addRoute(r, "/items/sync", itemsHandlers.syncItems, conf.UseCORS, http.MethodPost)
	addRoute(r, "/items/backup", itemsHandlers.backupItems, conf.UseCORS, http.MethodPost)

	addRoute(r, "/auth/params", authHandlers.getParams, conf.UseCORS, http.MethodGet)
	addRoute(r, "/auth/update", authHandlers.updateUser, conf.UseCORS, http.MethodPost)
	addRoute(r, "/auth/change_pw", authHandlers.changePassword, conf.UseCORS, http.MethodPost)
	addRoute(r, "/auth/sign_in", authHandlers.loginUser, conf.UseCORS, http.MethodPost)
	addRoute(r, "/auth/sign_in.json", authHandlers.loginUser, conf.UseCORS, http.MethodPost)
	if !conf.NoReg {
		addRoute(r, "/auth", authHandlers.registerUser, conf.UseCORS, http.MethodPost)
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

	return &server{
		conf: conf,
		http: http.Server{
			Addr:    conf.Host + ":" + strconv.Itoa(conf.Port),
			Handler: r,
		},
	}, nil
}

func (s *server) listen() error {
	handler := s.http.Handler

	if s.conf.Socket == "" {
		port := strconv.Itoa(s.conf.Port)
		if s.conf.UseCORS {
			handler = handlers.CORS()(handler)
		}
		log.Println("Listening on port " + port)
		return http.ListenAndServe(":"+port, handler)
	}

	os.Remove(s.conf.Socket)
	listener, err := net.Listen("unix", s.conf.Socket)
	if err != nil {
		return err
	}
	server := http.Server{Handler: handler}
	log.Println("Listening on socket " + s.conf.Socket)
	return server.Serve(listener)
}

func (s *server) shutdown() { close(s.work) }

// addRoute sets up one route on router. If cors is true, then it adds an
// additional CORS handler and adds the OPTIONS method to the route.
func addRoute(router *mux.Router, path string, handler http.HandlerFunc, cors bool, mtds ...string) {
	router.HandleFunc(path, handler).Methods(mtds...)
	if cors {
		router.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Headers", "*")
			w.Header().Set("Access-Control-Max-Age", "1728000")
			for _, key := range []string{"Access-Token", "Client", "UID"} {
				w.Header().Add("Access-Control-Expose-Headers", key)
			}
		}).Methods(http.MethodOptions)
	}
}
