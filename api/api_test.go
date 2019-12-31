package api_test

import (
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/rafaelespinoza/standardfile/api"
	"github.com/rafaelespinoza/standardfile/config"
)

// defaultDB tells the test server where the db is. If using sqlite3, use
// ":memory:" if you don't want a file at all.
const defaultDB = ":memory:"

func TestServe(t *testing.T) {
	t.Run("cors", func(t *testing.T) {
		cfg := config.Config{
			Debug:   true,
			DB:      defaultDB,
			Host:    "localhost",
			Port:    7777,
			UseCORS: true,
		}

		type TestCase struct {
			method string
			path   string
		}
		tests := []TestCase{
			{
				method: http.MethodPost,
				path:   "/auth",
			},
			{
				method: http.MethodPost,
				path:   "/auth/change_pw",
			},
			{
				method: http.MethodPost,
				path:   "/auth/sign_in",
			},
			{
				method: http.MethodPost,
				path:   "/auth/update",
			},
			{
				method: http.MethodGet,
				path:   "/auth/params",
			},
			{
				method: http.MethodPost,
				path:   "/items/backup",
			},
			{
				method: http.MethodPost,
				path:   "/items/sync",
			},
		}

		testClient := Client{http: &http.Client{}}
		go api.Serve(cfg)
		// sometimes server is not ready, TODO: synchronize this test better
		time.Sleep(100 * time.Millisecond)

		hold := sync.WaitGroup{}
		const origin = "https://app.standardnotes.org"

		for i, test := range tests {
			hold.Add(1)
			go func(i int, test TestCase) {
				defer hold.Done()
				var req *http.Request
				var res *http.Response
				var err error

				reqURI := "http://" + cfg.Host + ":" + strconv.Itoa(cfg.Port) + test.path
				if req, err = http.NewRequest(http.MethodOptions, reqURI, nil); err != nil {
					t.Fatal(err)
				}
				req.Header.Add("Access-Control-Request-Method", test.method)
				req.Header.Add("Origin", origin)

				if res, err = testClient.http.Do(req); err != nil {
					t.Errorf("test [%d]; unexpected error; %v", i, err)
					return
				}
				if res.StatusCode < 200 || res.StatusCode >= 300 {
					t.Errorf("test [%d]; unexpected response status code %d", i, res.StatusCode)
				}
				switch val := res.Header.Get("Access-Control-Allow-Origin"); val {
				case origin, "*":
					// no op
				default:
					t.Errorf("server does not allow origin %q", origin)
				}
				if val := res.Header.Get("Access-Control-Allow-Methods"); val != test.method {
					t.Errorf("server does not allow method %q", test.method)
				}
			}(i, test)
		}

		hold.Wait()
		defer api.Shutdown()
	})
}

type Client struct {
	http *http.Client
}
