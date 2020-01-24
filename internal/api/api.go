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
	"github.com/rafaelespinoza/standardnotes/internal/config"
	"github.com/rafaelespinoza/standardnotes/internal/db"
	"github.com/rafaelespinoza/standardnotes/internal/logger"
	"github.com/rs/cors"
)

var _Server *server

// Serve is the main workhorse of this package. It maps request routes to
// handlers and listens on the configured socket.
func Serve(cfg config.Config) (err error) {
	if _Server != nil {
		log.Println("server already running")
		return
	}

	db.Init(cfg.DB)
	log.Printf("started StandardNotes Server\n\tconfig:\n\t%+v\n", cfg)

	_Server, err := newServer(cfg)
	if err != nil {
		log.Println(err)
		return
	}

	defer func(cfg config.Config) {
		if len(cfg.Socket) != 0 {
			os.Remove(cfg.Socket)
		}
	}(cfg)

	go _Server.listen()
	<-_Server.work

	log.Println("server stopped")
	os.Exit(0)
	return
}

// Shutdown closes the server's internal channel.
func Shutdown() {
	if _Server != nil {
		_Server.shutdown()
	}
}

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
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("version " + config.Metadata.Version))
	}).Methods(http.MethodGet)

	r.HandleFunc("/items/sync", itemsHandlers.syncItems).Methods(http.MethodPost)
	r.HandleFunc("/items/backup", itemsHandlers.backupItems).Methods(http.MethodPost)

	r.HandleFunc("/auth/params", authHandlers.getParams).Methods(http.MethodGet)
	r.HandleFunc("/auth/update", authHandlers.updateUser).Methods(http.MethodPost)
	r.HandleFunc("/auth/change_pw", authHandlers.changePassword).Methods(http.MethodPost)
	r.HandleFunc("/auth/sign_in", authHandlers.loginUser).Methods(http.MethodPost)
	r.HandleFunc("/auth/sign_in.json", authHandlers.loginUser).Methods(http.MethodPost)
	if !conf.NoReg {
		r.HandleFunc("/auth", authHandlers.registerUser).Methods(http.MethodPost)
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

	var handler http.Handler
	if !conf.UseCORS {
		handler = r
	} else {
		handler = cors.New(
			cors.Options{
				AllowedHeaders: []string{"*"},
				ExposedHeaders: []string{"Access-Token", "Client", "UID"},
				MaxAge:         86400,
			},
		).Handler(r)
	}

	return &server{
		conf: conf,
		http: http.Server{
			Addr:    conf.Host + ":" + strconv.Itoa(conf.Port),
			Handler: handler,
		},
	}, nil
}

func (s *server) listen() error {
	handler := s.http.Handler

	if s.conf.Socket == "" {
		port := strconv.Itoa(s.conf.Port)
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
