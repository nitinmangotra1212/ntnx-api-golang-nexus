/*
 * Cat API Endpoints Interface
 * Following Nutanix gRPC Gateway pattern
 *
 * Generated/Manual interface definition for Cat service operations
 */

package config

import (
	"net/http"
)

// Cat_endpoints defines the interface for Cat API operations
// This follows the Nutanix pattern from the Confluence guide
type Cat_endpoints interface {
	// ========================================
	// Synchronous Operations (Direct Response)
	// ========================================

	// GetCatById retrieves a single cat by ID
	// GET /mock/v4/config/cat/{catId}
	GetCatById(w http.ResponseWriter, r *http.Request)

	// ListCats retrieves a paginated list of cats
	// GET /mock/v4/config/cats
	ListCats(w http.ResponseWriter, r *http.Request)

	// CreateCat creates a new cat
	// POST /mock/v4/config/cats
	CreateCat(w http.ResponseWriter, r *http.Request)

	// UpdateCatById updates an existing cat
	// PUT /mock/v4/config/cat/{catId}
	UpdateCatById(w http.ResponseWriter, r *http.Request)

	// DeleteCatById deletes a cat
	// DELETE /mock/v4/config/cat/{catId}
	DeleteCatById(w http.ResponseWriter, r *http.Request)

	// ========================================
	// Asynchronous Operations (Task-based)
	// ========================================

	// GetCatById_Process retrieves a cat asynchronously
	// Returns task ID immediately, processes in background
	// GET /mock/v4/config/cat/{catId}/async
	GetCatById_Process(w http.ResponseWriter, r *http.Request)

	// ========================================
	// Task Management (Task Server)
	// ========================================

	// CreateTask creates a new task (Task Server endpoint)
	// POST /tasks
	CreateTask(w http.ResponseWriter, r *http.Request)

	// PutTask updates an existing task (Task Server endpoint)
	// PUT /tasks/{taskId}
	PutTask(w http.ResponseWriter, r *http.Request)

	// PollTask retrieves task status (Task Server endpoint)
	// GET /tasks/{taskId}
	PollTask(w http.ResponseWriter, r *http.Request)
}

// Cat_endpointsImplWrapper wraps the service implementation
// This allows for middleware, logging, etc.
type Cat_endpointsImplWrapper struct {
	svcImpl Cat_endpoints
}

// NewCat_endpointsImplWrapper creates a new wrapper
func NewCat_endpointsImplWrapper(impl Cat_endpoints) *Cat_endpointsImplWrapper {
	return &Cat_endpointsImplWrapper{
		svcImpl: impl,
	}
}

// Wrapper methods that delegate to the implementation
func (w *Cat_endpointsImplWrapper) GetCatById(rw http.ResponseWriter, r *http.Request) {
	w.svcImpl.GetCatById(rw, r)
}

func (w *Cat_endpointsImplWrapper) ListCats(rw http.ResponseWriter, r *http.Request) {
	w.svcImpl.ListCats(rw, r)
}

func (w *Cat_endpointsImplWrapper) CreateCat(rw http.ResponseWriter, r *http.Request) {
	w.svcImpl.CreateCat(rw, r)
}

func (w *Cat_endpointsImplWrapper) UpdateCatById(rw http.ResponseWriter, r *http.Request) {
	w.svcImpl.UpdateCatById(rw, r)
}

func (w *Cat_endpointsImplWrapper) DeleteCatById(rw http.ResponseWriter, r *http.Request) {
	w.svcImpl.DeleteCatById(rw, r)
}

func (w *Cat_endpointsImplWrapper) GetCatById_Process(rw http.ResponseWriter, r *http.Request) {
	w.svcImpl.GetCatById_Process(rw, r)
}

func (w *Cat_endpointsImplWrapper) CreateTask(rw http.ResponseWriter, r *http.Request) {
	w.svcImpl.CreateTask(rw, r)
}

func (w *Cat_endpointsImplWrapper) PutTask(rw http.ResponseWriter, r *http.Request) {
	w.svcImpl.PutTask(rw, r)
}

func (w *Cat_endpointsImplWrapper) PollTask(rw http.ResponseWriter, r *http.Request) {
	w.svcImpl.PollTask(rw, r)
}
