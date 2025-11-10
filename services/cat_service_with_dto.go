package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/nutanix/ntnx-api-golang-mock/global"

	// Import generated DTOs with AUTO-SET $objectType!
	generated "github.com/nutanix/ntnx-api-golang-mock-pc/generated-code/dto/models/mock/v4/config"

	log "github.com/sirupsen/logrus"
)

// CatServiceWithDTO implements Cat_endpoints using auto-generated DTOs
type CatServiceWithDTO struct {
	catMutex sync.Mutex
}

// NewCatServiceWithDTO creates a new instance using generated DTOs
func NewCatServiceWithDTO() *CatServiceWithDTO {
	// Initialize some mock cats using generated constructors
	global.Mutex.Lock()
	defer global.Mutex.Unlock()

	log.Info("🎯 Initializing cats with AUTO-GENERATED DTOs (auto-set $objectType)")

	// Clear existing cats
	global.CatsDTO = make(map[int]*generated.Cat)

	for i := 1; i <= 100; i++ {
		cat := createCatWithAutoDTO(i)
		global.CatsDTO[i] = cat
	}

	log.Infof("✅ Initialized %d cats with auto-set $objectType='mock.v4.config.Cat'", len(global.CatsDTO))
	return &CatServiceWithDTO{}
}

// createCatWithAutoDTO creates a Cat using the generated constructor
// NO MANUAL $objectType - It's AUTO-SET by NewCat()!
func createCatWithAutoDTO(id int) *generated.Cat {
	cat := generated.NewCat() // ← $objectType AUTO-SET HERE!
	//       ^^^^^^^^^^^^^^^^^
	//       This constructor automatically sets:
	//       - ObjectType_ = "mock.v4.config.Cat"
	//       - Reserved_ = map[string]interface{}{"$fv": "v4.r1"}
	//       - UnknownFields_ = map[string]interface{}{}

	catID := id
	cat.CatId = &catID

	catName := fmt.Sprintf("Cat-%d", id)
	cat.CatName = &catName

	catType := "TYPE1"
	cat.CatType = &catType

	description := "A fluffy cat"
	cat.Description = &description

	// Add location with nested objects (also auto-generated)
	if id%2 == 0 {
		location := generated.NewLocation() // ← AUTO-SET: "mock.v4.config.Location"
		city := "San Francisco"
		location.City = &city

		country := generated.NewCountry() // ← AUTO-SET: "mock.v4.config.Country"
		state := "California"
		country.State = &state
		location.Country = country

		cat.Location = location
	}

	return cat
}

// ListCats handles GET /mock/v4/config/cats
func (s *CatServiceWithDTO) ListCats(w http.ResponseWriter, r *http.Request) {
	log.Info("API Handler: ListCats called (using AUTO-GENERATED DTOs)")
	w.Header().Set("Content-Type", "application/json")

	page := s.getIntQueryParam(r, "$page", 1)
	limit := s.getIntQueryParam(r, "$limit", 10)

	global.Mutex.Lock()
	defer global.Mutex.Unlock()

	// Collect all cats
	var allCats []*generated.Cat
	for _, cat := range global.CatsDTO {
		allCats = append(allCats, cat)
	}

	// Paginate
	start := (page - 1) * limit
	end := start + limit
	if start >= len(allCats) {
		allCats = []*generated.Cat{}
	} else if end > len(allCats) {
		allCats = allCats[start:]
	} else {
		allCats = allCats[start:end]
	}

	response := s.buildCatListResponse(allCats, page, limit, len(global.CatsDTO))
	json.NewEncoder(w).Encode(response)
}

// GetCatById handles GET /mock/v4/config/cat/{catId}
func (s *CatServiceWithDTO) GetCatById(w http.ResponseWriter, r *http.Request) {
	log.Info("API Handler: GetCatByID called (using AUTO-GENERATED DTOs)")
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	catIDStr := vars["catId"]
	catID, err := strconv.Atoi(catIDStr)
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid cat ID")
		return
	}

	global.Mutex.Lock()
	cat, found := global.CatsDTO[catID]
	global.Mutex.Unlock()

	if !found {
		s.sendError(w, http.StatusNotFound, fmt.Sprintf("Cat with ID %d not found", catID))
		return
	}

	response := s.buildCatResponse(cat)
	json.NewEncoder(w).Encode(response)
}

// CreateCat handles POST /mock/v4/config/cats
func (s *CatServiceWithDTO) CreateCat(w http.ResponseWriter, r *http.Request) {
	log.Info("API Handler: CreateCat called (using AUTO-GENERATED DTOs)")
	w.Header().Set("Content-Type", "application/json")

	var catCreate struct {
		CatName     string  `json:"catName"`
		CatType     string  `json:"catType"`
		Description *string `json:"description,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&catCreate); err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	global.Mutex.Lock()
	defer global.Mutex.Unlock()

	// Find next available ID
	newID := len(global.CatsDTO) + 1

	// Use generated constructor - $objectType AUTO-SET!
	cat := generated.NewCat()
	cat.CatId = &newID
	cat.CatName = &catCreate.CatName
	cat.CatType = &catCreate.CatType
	if catCreate.Description != nil {
		cat.Description = catCreate.Description
	}

	global.CatsDTO[newID] = cat

	response := s.buildCatResponse(cat)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)

	log.Infof("✅ Created cat with ID %d (auto-set $objectType)", newID)
}

// UpdateCatById handles PUT /mock/v4/config/cat/{catId}
func (s *CatServiceWithDTO) UpdateCatById(w http.ResponseWriter, r *http.Request) {
	log.Info("API Handler: UpdateCatByID called (using AUTO-GENERATED DTOs)")
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	catIDStr := vars["catId"]
	catID, err := strconv.Atoi(catIDStr)
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid cat ID")
		return
	}

	var updates struct {
		CatName     *string `json:"catName,omitempty"`
		CatType     *string `json:"catType,omitempty"`
		Description *string `json:"description,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	global.Mutex.Lock()
	defer global.Mutex.Unlock()

	cat, found := global.CatsDTO[catID]
	if !found {
		s.sendError(w, http.StatusNotFound, fmt.Sprintf("Cat with ID %d not found", catID))
		return
	}

	// Update fields
	if updates.CatName != nil {
		cat.CatName = updates.CatName
	}
	if updates.CatType != nil {
		cat.CatType = updates.CatType
	}
	if updates.Description != nil {
		cat.Description = updates.Description
	}

	response := s.buildCatResponse(cat)
	json.NewEncoder(w).Encode(response)

	log.Infof("✅ Updated cat with ID %d", catID)
}

// DeleteCatById handles DELETE /mock/v4/config/cat/{catId}
func (s *CatServiceWithDTO) DeleteCatById(w http.ResponseWriter, r *http.Request) {
	log.Info("API Handler: DeleteCatByID called (using AUTO-GENERATED DTOs)")
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	catIDStr := vars["catId"]
	catID, err := strconv.Atoi(catIDStr)
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid cat ID")
		return
	}

	global.Mutex.Lock()
	defer global.Mutex.Unlock()

	if _, found := global.CatsDTO[catID]; !found {
		s.sendError(w, http.StatusNotFound, fmt.Sprintf("Cat with ID %d not found", catID))
		return
	}

	delete(global.CatsDTO, catID)

	response := map[string]interface{}{
		"$objectType": "mock.v4.config.DeleteCatApiResponse",
		"$reserved": map[string]string{
			"$fv": "v4.r1",
		},
		"message": fmt.Sprintf("Cat with ID %d deleted successfully", catID),
	}
	json.NewEncoder(w).Encode(response)

	log.Infof("✅ Deleted cat with ID %d", catID)
}

// GetCatById_Process handles async GET (gRPC Gateway pattern)
func (s *CatServiceWithDTO) GetCatById_Process(w http.ResponseWriter, r *http.Request) {
	log.Info("API Handler: GetCatByID_Process (async) called")
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	catIDStr := vars["catId"]
	catID, err := strconv.Atoi(catIDStr)
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid cat ID")
		return
	}

	// Create a new task
	taskID := uuid.New().String()
	task := &global.Task{
		TaskId:             taskID,
		PercentageComplete: 0,
		Status:             "PENDING",
		Message:            fmt.Sprintf("Fetching cat %d asynchronously", catID),
	}

	// Store locally (for API Handler)
	global.Mutex.Lock()
	global.Tasks[taskID] = task
	global.Mutex.Unlock()

	// ✅ POST task to Task Server so it can track it
	client := &http.Client{}
	taskURL := fmt.Sprintf("http://%s:%s/tasks", global.TaskServerHost, global.TaskServerPort)
	taskJSON, _ := json.Marshal(task)

	log.Infof("📤 Registering task %s with Task Server: %s", taskID, taskURL)
	log.Debugf("Task payload: %s", string(taskJSON))

	req, err := http.NewRequest("POST", taskURL, bytes.NewBuffer(taskJSON))
	if err != nil {
		log.Errorf("Failed to create POST request: %v", err)
		s.sendError(w, http.StatusInternalServerError, "Failed to create task")
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("Failed to register task with Task Server: %v", err)
		s.sendError(w, http.StatusInternalServerError, "Failed to create task")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		log.Errorf("Task Server returned non-OK status: %d - %s", resp.StatusCode, string(bodyBytes))
		s.sendError(w, http.StatusInternalServerError, "Failed to register task")
		return
	}

	log.Infof("✅ Task %s registered successfully with Task Server", taskID)

	// Respond immediately with task ID
	response := map[string]interface{}{
		"$objectType": "mock.v4.config.AsyncTaskResponse",
		"$reserved": map[string]string{
			"$fv": "v4.r1",
		},
		"taskId":  taskID,
		"message": fmt.Sprintf("Poll at Task Server (port %s) with taskId: %s", global.TaskServerPort, taskID),
		"pollUrl": fmt.Sprintf("http://%s:%s/tasks/%s", global.TaskServerHost, global.TaskServerPort, taskID),
	}
	json.NewEncoder(w).Encode(response)

	// Start asynchronous processing
	go s.processGetCatTaskWithDTO(taskID, catID)
}

// processGetCatTaskWithDTO processes the async task using generated DTOs
func (s *CatServiceWithDTO) processGetCatTaskWithDTO(taskID string, catID int) {
	log.Infof("Starting async processing for task %s (cat ID: %d)", taskID, catID)
	client := &http.Client{}
	taskURL := fmt.Sprintf("http://%s:%s/tasks/%s", global.TaskServerHost, global.TaskServerPort, taskID)

	// Simulate subtask 1
	time.Sleep(3 * time.Second)
	s.updateTask(client, taskURL, taskID, 33, "Fetching data from source 1")

	// Simulate subtask 2
	time.Sleep(3 * time.Second)
	s.updateTask(client, taskURL, taskID, 67, "Processing data")

	// Simulate subtask 3
	time.Sleep(3 * time.Second)
	global.Mutex.Lock()
	cat, found := global.CatsDTO[catID]
	global.Mutex.Unlock()

	if found {
		// Convert generated Cat to map for JSON response
		catJSON, _ := json.Marshal(cat)
		var catMap map[string]interface{}
		json.Unmarshal(catJSON, &catMap)

		taskUpdate := map[string]interface{}{
			"taskId":             taskID,
			"percentageComplete": 100,
			"status":             "COMPLETED",
			"message":            "Cat retrieved successfully",
			"objectReturned":     catMap,
		}
		jsonBody, _ := json.Marshal(taskUpdate)
		req, _ := http.NewRequest("PUT", taskURL, bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		client.Do(req)
	} else {
		s.updateTask(client, taskURL, taskID, 100, "Cat not found", "FAILED")
	}
	log.Infof("Async processing for task %s completed", taskID)
}

// Task Server endpoints (same as before)
func (s *CatServiceWithDTO) CreateTask(w http.ResponseWriter, r *http.Request) {
	log.Info("Task Server: CreateTask called")
	w.Header().Set("Content-Type", "application/json")

	var task global.Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	global.Mutex.Lock()
	global.Tasks[task.TaskId] = &task
	global.Mutex.Unlock()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(task)
}

func (s *CatServiceWithDTO) PutTask(w http.ResponseWriter, r *http.Request) {
	log.Info("Task Server: PutTask called")
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	taskID := vars["taskId"]

	var updatedTask global.Task
	if err := json.NewDecoder(r.Body).Decode(&updatedTask); err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	global.Mutex.Lock()
	defer global.Mutex.Unlock()

	if existingTask, found := global.Tasks[taskID]; found {
		existingTask.PercentageComplete = updatedTask.PercentageComplete
		existingTask.Status = updatedTask.Status
		existingTask.Message = updatedTask.Message
		if updatedTask.ObjectReturned != nil {
			existingTask.ObjectReturned = updatedTask.ObjectReturned
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(existingTask)
	} else {
		s.sendError(w, http.StatusNotFound, "Task not found")
	}
}

func (s *CatServiceWithDTO) PollTask(w http.ResponseWriter, r *http.Request) {
	log.Info("Task Server: PollTask called")
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	taskID := vars["taskId"]

	global.Mutex.Lock()
	defer global.Mutex.Unlock()

	if task, found := global.Tasks[taskID]; found {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(task)
	} else {
		s.sendError(w, http.StatusNotFound, "Task not found")
	}
}

// Helper functions

func (s *CatServiceWithDTO) buildCatResponse(cat *generated.Cat) map[string]interface{} {
	// The cat already has $objectType auto-set!
	// Just convert to JSON and wrap in response
	catJSON, _ := json.Marshal(cat)
	var catMap map[string]interface{}
	json.Unmarshal(catJSON, &catMap)

	return map[string]interface{}{
		"$objectType": "mock.v4.config.GetCatApiResponse",
		"$reserved": map[string]string{
			"$fv": "v4.r1",
		},
		"data": catMap, // Cat with auto-set $objectType!
	}
}

func (s *CatServiceWithDTO) buildCatListResponse(cats []*generated.Cat, page, limit, total int) map[string]interface{} {
	catData := make([]map[string]interface{}, len(cats))
	for i, cat := range cats {
		// Each cat has $objectType auto-set by NewCat()!
		catJSON, _ := json.Marshal(cat)
		var catMap map[string]interface{}
		json.Unmarshal(catJSON, &catMap)
		catData[i] = catMap
	}

	// Build metadata (same as before)
	totalPages := (total + limit - 1) / limit
	hasNext := page < totalPages
	hasPrevious := page > 1
	isTruncated := total > (page * limit)

	flags := []map[string]interface{}{
		{
			"$objectType": "common.v1.config.Flag",
			"$reserved":   map[string]string{"$fv": "v1.r0"},
			"name":        "hasError",
			"value":       false,
		},
		{
			"$objectType": "common.v1.config.Flag",
			"$reserved":   map[string]string{"$fv": "v1.r0"},
			"name":        "isPaginated",
			"value":       true,
		},
		{
			"$objectType": "common.v1.config.Flag",
			"$reserved":   map[string]string{"$fv": "v1.r0"},
			"name":        "isTruncated",
			"value":       isTruncated,
		},
	}

	baseURL := fmt.Sprintf("http://%s:%s/mock/v4/config/cats", global.APIServerHost, global.APIServerPort)
	links := []map[string]interface{}{
		{
			"$objectType": "common.v1.response.ApiLink",
			"$reserved":   map[string]string{"$fv": "v1.r0"},
			"href":        fmt.Sprintf("%s?$page=%d&$limit=%d", baseURL, page, limit),
			"rel":         "self",
		},
		{
			"$objectType": "common.v1.response.ApiLink",
			"$reserved":   map[string]string{"$fv": "v1.r0"},
			"href":        fmt.Sprintf("%s?$page=1&$limit=%d", baseURL, limit),
			"rel":         "first",
		},
	}

	if hasPrevious {
		links = append(links, map[string]interface{}{
			"$objectType": "common.v1.response.ApiLink",
			"$reserved":   map[string]string{"$fv": "v1.r0"},
			"href":        fmt.Sprintf("%s?$page=%d&$limit=%d", baseURL, page-1, limit),
			"rel":         "previous",
		})
	}

	if hasNext {
		links = append(links, map[string]interface{}{
			"$objectType": "common.v1.response.ApiLink",
			"$reserved":   map[string]string{"$fv": "v1.r0"},
			"href":        fmt.Sprintf("%s?$page=%d&$limit=%d", baseURL, page+1, limit),
			"rel":         "next",
		})
	}

	links = append(links, map[string]interface{}{
		"$objectType": "common.v1.response.ApiLink",
		"$reserved":   map[string]string{"$fv": "v1.r0"},
		"href":        fmt.Sprintf("%s?$page=%d&$limit=%d", baseURL, totalPages, limit),
		"rel":         "last",
	})

	return map[string]interface{}{
		"$objectType": "mock.v4.config.ListCatsApiResponse",
		"$reserved": map[string]string{
			"$fv": "v4.r1",
		},
		"data": catData,
		"metadata": map[string]interface{}{
			"$objectType": "common.v1.response.ApiResponseMetadata",
			"$reserved": map[string]string{
				"$fv": "v1.r0",
			},
			"flags":                 flags,
			"links":                 links,
			"totalAvailableResults": total,
		},
	}
}

func (s *CatServiceWithDTO) updateTask(client *http.Client, taskURL, taskID string, percentage int, message string, status ...string) {
	currentStatus := "IN_PROGRESS"
	if len(status) > 0 {
		currentStatus = status[0]
	}
	taskUpdate := map[string]interface{}{
		"taskId":             taskID,
		"percentageComplete": percentage,
		"status":             currentStatus,
		"message":            message,
	}
	jsonBody, _ := json.Marshal(taskUpdate)
	req, _ := http.NewRequest("PUT", taskURL, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("Failed to update task %s: %v", taskID, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Errorf("Task Server returned non-OK status for task %s: %d - %s", taskID, resp.StatusCode, string(body))
	}
}

func (s *CatServiceWithDTO) sendError(w http.ResponseWriter, statusCode int, message string) {
	w.WriteHeader(statusCode)
	response := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    statusCode,
			"message": message,
		},
	}
	json.NewEncoder(w).Encode(response)
	log.Errorf("❌ Error: %d - %s", statusCode, message)
}

func (s *CatServiceWithDTO) getIntQueryParam(r *http.Request, key string, defaultValue int) int {
	val := r.URL.Query().Get(key)
	if val == "" {
		return defaultValue
	}
	intVal, err := strconv.Atoi(val)
	if err != nil {
		return defaultValue
	}
	return intVal
}
