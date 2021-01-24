package http

import (
	// Core packages
	"encoding/json"
	basehttp "net/http"
	"os"
	"regexp"
	"time"

	// DocHQ specific packages
	"github.com/DocHQ/logging"

	// 3rd party packages
	"github.com/gorilla/mux"
)

// Simple cover for the mux router, saves another import at the service level
type Router struct {
	*mux.Router
}

// A copy of the router for internal passing betweeen functions
var r *Router

// function New is just your normal new function for a HTTP router, this allows
// the calling applicaiton to add routes specific to that microservice
func New() *Router {
	// Create a new instance of the router interface to be passed back to the caller
	var r *Router = &Router{}
	r.Router = mux.NewRouter() // this init's some internal stuff so can't from outside

	// Create a default logging middleware layer that tells us every htto request going
	// through the http server,
	r.Use(func(next basehttp.Handler) basehttp.Handler {
		return basehttp.HandlerFunc(func(w basehttp.ResponseWriter, r *basehttp.Request) {
			start := time.Now()
			path := r.RequestURI
			sw := statusWriter{ResponseWriter: w}

			next.ServeHTTP(&sw, r)

			// Where Google spams the /health endpoint constantly,
			// skip it in the logs
			if path == "/health" && os.Getenv("DEBUG_WITH_HEALTH") != "true" {
				return
			}

			header := ""
			var re = regexp.MustCompile(`(?m).*\s(.*)$`)
			header = re.ReplaceAllString(r.Header.Get("Authorization"), " Token:$1")

			end := time.Now()
			latency := end.Sub(start)

			logging.Infof(
				"HTTP Request :- time:%v ip:%v latency:%v method:%v path:%v status:%v%v",
				end.Format(time.RFC3339),
				r.RemoteAddr,
				latency,
				r.Method,
				path,
				sw.status,
				header,
			)
		})
	})

	// Due to angular being a thing, we need to make sure we respond correctly
	// to any OPTIONS requests otherwise it just wont make the request
	r.Use(func(next basehttp.Handler) basehttp.Handler {
		return basehttp.HandlerFunc(func(w basehttp.ResponseWriter, r *basehttp.Request) {
			if r.Method == basehttp.MethodOptions {
				if err := json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"}); err != nil {
					logging.Error(err)
				}
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	// Default routes that should be consistent across all services
	// this is a health endpoint to show the internal keep-alives that the service
	// is there
	r.HandleFunc("/health", func(w basehttp.ResponseWriter, r *basehttp.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"}); err != nil {
			logging.Error(err)
		}
	}).Methods("GET")

	// Return the resulting router
	return r
}

// Authorize is middleware to ensure security
func Authorize(serviceKey string) (mw func(basehttp.Handler) basehttp.Handler) {
	mw = func(next basehttp.Handler) basehttp.Handler {
		return basehttp.HandlerFunc(func(w basehttp.ResponseWriter, r *basehttp.Request) {
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			ctx := r.Context()

			if r.Method == basehttp.MethodOptions {
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
				w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			if r.URL.Path == "/health" {
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			tokenString := r.Header.Get("Authorization")
			if len(tokenString) == 0 {
				logging.Error("no authorisation token presented on : " + r.URL.Path)
				RespondError(w, r, basehttp.StatusForbidden, "Not authorized")
				return
			}

			if tokenString != serviceKey {
				logging.Error("authorisation token presented but not valid")
				RespondError(w, r, basehttp.StatusUnauthorized, "Not authorized")
				return
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
	return
}
