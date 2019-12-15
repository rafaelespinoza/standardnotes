package api

import (
	"fmt"
	"net/http"
)

// the following stuff is experimental

type expServer interface {
	registerGet(route string, handler http.HandlerFunc) error
	registerPost(route string, handler http.HandlerFunc) error
	registerPatch(route string, handler http.HandlerFunc) error
	registerPut(route string, handler http.HandlerFunc) error
	registerDelete(route string, handler http.HandlerFunc) error
}

type expMux struct {
	*http.ServeMux
	handlers map[string]map[string]http.HandlerFunc
}

func newMux() (*expMux, error) {
	handlers := make(map[string]map[string]http.HandlerFunc)
	return &expMux{
		handlers: handlers,
	}, nil
}

func (m *expMux) registerGet(route string, handler http.HandlerFunc) error {
	if _, ok := m.handlers[http.MethodGet][route]; ok {
		return fmt.Errorf(
			"handler already registered for method %q route %q",
			http.MethodGet, route,
		)
	}
	m.handlers[http.MethodGet][route] = handler
	return nil
}
