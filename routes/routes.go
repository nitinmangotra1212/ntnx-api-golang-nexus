/*
 * Route definitions following Nutanix gRPC Gateway pattern
 * Using gorilla/mux for routing (as per Confluence guide)
 */

package routes

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/nutanix/ntnx-api-golang-mock/interfaces/apis/mock/v4/config"
	log "github.com/sirupsen/logrus"
)

// RouteKey uniquely identifies a route
type RouteKey struct {
	Path   string
	Method string
}

// HandlerMap returns a map of routes to handlers
// Following the pattern from Confluence guide
func HandlerMap(wrapper *config.Cat_endpointsImplWrapper) map[RouteKey]http.HandlerFunc {
	r := make(map[RouteKey]http.HandlerFunc)

	// ========================================
	// API Handler Server Routes (Port 9009)
	// ========================================

	// Synchronous Cat operations
	r[RouteKey{Path: "/mock/v4/config/cats", Method: "GET"}] = wrapper.ListCats
	r[RouteKey{Path: "/mock/v4/config/cat/{catId}", Method: "GET"}] = wrapper.GetCatById
	r[RouteKey{Path: "/mock/v4/config/cats", Method: "POST"}] = wrapper.CreateCat
	r[RouteKey{Path: "/mock/v4/config/cat/{catId}", Method: "PUT"}] = wrapper.UpdateCatById
	r[RouteKey{Path: "/mock/v4/config/cat/{catId}", Method: "DELETE"}] = wrapper.DeleteCatById

	// Asynchronous Cat operations
	r[RouteKey{Path: "/mock/v4/config/cat/{catId}/async", Method: "GET"}] = wrapper.GetCatById_Process

	// ========================================
	// Task Server Routes (Port 9010)
	// ========================================

	r[RouteKey{Path: "/tasks", Method: "POST"}] = wrapper.CreateTask
	r[RouteKey{Path: "/tasks/{taskId}", Method: "PUT"}] = wrapper.PutTask
	r[RouteKey{Path: "/tasks/{taskId}", Method: "GET"}] = wrapper.PollTask

	return r
}

// SetupAPIServerRouter creates the router for API Handler Server (Port 9009)
func SetupAPIServerRouter(wrapper *config.Cat_endpointsImplWrapper) *mux.Router {
	router := mux.NewRouter()

	// Health check
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"UP","server":"api-handler","port":"9009"}`))
	}).Methods("GET")

	// Cat CRUD operations
	router.HandleFunc("/mock/v4/config/cats", wrapper.ListCats).Methods("GET")
	router.HandleFunc("/mock/v4/config/cat/{catId}", wrapper.GetCatById).Methods("GET")
	router.HandleFunc("/mock/v4/config/cats", wrapper.CreateCat).Methods("POST")
	router.HandleFunc("/mock/v4/config/cat/{catId}", wrapper.UpdateCatById).Methods("PUT")
	router.HandleFunc("/mock/v4/config/cat/{catId}", wrapper.DeleteCatById).Methods("DELETE")

	// Async operations
	router.HandleFunc("/mock/v4/config/cat/{catId}/async", wrapper.GetCatById_Process).Methods("GET")

	// Logging middleware
	router.Use(loggingMiddleware)

	log.Info("✅ API Handler Server routes configured")
	return router
}

// SetupTaskServerRouter creates the router for Task Server (Port 9010)
func SetupTaskServerRouter(wrapper *config.Cat_endpointsImplWrapper) *mux.Router {
	router := mux.NewRouter()

	// Health check
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"UP","server":"task-server","port":"9010"}`))
	}).Methods("GET")

	// Task operations
	router.HandleFunc("/tasks", wrapper.CreateTask).Methods("POST")
	router.HandleFunc("/tasks/{taskId}", wrapper.PutTask).Methods("PUT")
	router.HandleFunc("/tasks/{taskId}", wrapper.PollTask).Methods("GET")

	// Logging middleware
	router.Use(loggingMiddleware)

	log.Info("✅ Task Server routes configured")
	return router
}

// loggingMiddleware logs all requests
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Infof("📥 %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}
