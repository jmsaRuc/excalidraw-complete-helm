package firebase

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"excalidraw-complete/config"
	"excalidraw-complete/redis"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type (
	// BatchGetRequest represents a request to get multiple documents.
	BatchGetRequest struct {
		Documents []string `json:"documents"`
	}

	// BatchGetEmptyResponse models a "missing" entry in a batch get response.
	BatchGetEmptyResponse struct {
		Missing  string `json:"missing"`
		ReadTime string `json:"readTime"`
	}

	// FoundInfoResponse models the information of a found document.
	FoundInfoResponse struct {
		Name       string      `json:"name"`
		Fields     interface{} `json:"fields"`
		CreateTime string      `json:"createTime"`
		UpdateTime string      `json:"updateTime"`
	}

	// BatchGetExistsResponse models a "found" entry in a batch get response.
	BatchGetExistsResponse struct {
		Found    FoundInfoResponse `json:"found"`
		ReadTime string            `json:"readTime"`
	}

	// UpdateRequest represents an update to a document.
	UpdateRequest struct {
		Name   string      `json:"name"`
		Fields interface{} `json:"fields"`
	}

	// WriteRequest represents a request to write a document.
	WriteRequest struct {
		Update UpdateRequest `json:"update"`
	}

	// BatchCommitRequest represents a request to commit multiple writes.
	BatchCommitRequest struct {
		Writes []WriteRequest `json:"writes"`
	}

	// WriteResult represents the result of a write operation.
	WriteResult struct {
		UpdateTime string `json:"updateTime"`
	}

	// BatchCommitResponse represents the response for a batch commit request.
	BatchCommitResponse struct {
		WriteResults []WriteResult `json:"writeResults"`
		CommitTime   string        `json:"commitTime"`
	}
)

// ListenSession stores information about a Firestore listen session
type ListenSession struct {
	ID           string `json:"id"`
	GSessionID   string `json:"gSessionId"`
	Database     string `json:"database"`
	TargetID     int    `json:"targetId"`
	DocumentPath string `json:"documentPath"`
	AID          int    `json:"aid"` // Array ID - increments with each response
	CreatedAt    int64  `json:"createdAt"`
}

// Bind is a no-op for BatchGetRequest.
func (body *BatchGetRequest) Bind(r *http.Request) (err error) {
	return nil
}

// Bind is a no-op for BatchCommitRequest.
func (body *BatchCommitRequest) Bind(r *http.Request) (err error) {
	return nil
}

// ListenSession stores information about a Firestore listen session
// Session store (local fallback when HA is not active)
var (
	sessions   = make(map[string]*ListenSession)
	sessionsMu sync.RWMutex
)

// generateSessionID creates a random session ID similar to Firebase's format
func generateSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// generateGSessionID creates a random gsessionid
func generateGSessionID() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// create a local in-memory map to store firebase documents if HA is not active
var savedItemsLocal = make(map[string]interface{})

// Session storage helpers for Redis (HA mode)
func getSessionKey(sid string) string {
	return fmt.Sprintf("firestore-session:%s", sid)
}

func getRoomSessionKey(roomID string) string {
	return fmt.Sprintf("firestore-room-session:%s", roomID)
}

func saveSessionToRedis(cacheStore *redis.CacheStore, session *ListenSession) error {
	sessionBytes, err := json.Marshal(session)
	if err != nil {
		return err
	}
	// Session expires after 30 minutes
	return cacheStore.Set(getSessionKey(session.ID), string(sessionBytes), 30*time.Minute)
}

func getSessionFromRedis(cacheStore *redis.CacheStore, sid string) (*ListenSession, error) {
	data, err := cacheStore.Get(getSessionKey(sid))
	if err != nil {
		return nil, err
	}
	var session ListenSession
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// saveRoomSessionMapping stores a mapping from roomID to sessionID for consistent sessions
func saveRoomSessionMapping(cacheStore *redis.CacheStore, roomID string, session *ListenSession) error {
	sessionBytes, err := json.Marshal(session)
	if err != nil {
		return err
	}
	// Room session mapping expires after 30 minutes
	return cacheStore.Set(getRoomSessionKey(roomID), string(sessionBytes), 30*time.Minute)
}

// getSessionByRoomID retrieves an existing session for a room
func getSessionByRoomID(cacheStore *redis.CacheStore, roomID string) (*ListenSession, error) {
	data, err := cacheStore.Get(getRoomSessionKey(roomID))
	if err != nil {
		return nil, err
	}
	var session ListenSession
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// extractRoomID extracts the room ID from a document path
// e.g., "projects/excalidraw-room-persistence/databases/(default)/documents/scenes/abc123" -> "abc123"
func extractRoomID(docPath string) string {
	parts := strings.Split(docPath, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

// HandleBatchCommit handles batch commit requests.
func HandleBatchCommit(config *config.Config, redisClient *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var cacheStore *redis.CacheStore
		var cancel context.CancelFunc
		var ctx context.Context
		if config.HAActive {
			ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			var err error
			cacheStore, err = redis.NewCacheStore(ctx, redisClient)
			if err != nil {
				logrus.Errorf("Failed to create cache store: %v", err)
				render.Status(r, http.StatusInternalServerError)
				cancel()
				return
			}
		}

		projectID := chi.URLParam(r, "project_id")
		databaseID := chi.URLParam(r, "database_id")
		_ = projectID
		_ = databaseID

		data := &BatchCommitRequest{}
		// Seems like requests is text/plain but content is json ...
		if err := render.DecodeJSON(r.Body, data); err != nil {
			logrus.Errorf("Failed to decode JSON: %v", err)
			render.Status(r, http.StatusBadRequest)
			if config.HAActive {
				cancel()
			}
			return
		}

		// the timestamps must be UTC, i.e. no zone offsets allowed
		timestamp := time.Now().UTC().Format(time.RFC3339)

		// extract document ID from path
		// - path format: "projects/excalidraw-room-persistence/databases/(default)/documents/scenes/abc123"
		docID := extractRoomID(data.Writes[0].Update.Name)

		// if HA is active, get existing saved items from redis, then update with new items
		if config.HAActive {
			savedItemsRedis := make(map[string]interface{})
			savedData, err := cacheStore.GetSavedData("firebase-documents")
			if err != nil || len(savedData) == 0 {
				logrus.Info("No saved items yet, continuing")
			} else {
				savedItemsRedis = savedData
			}

			savedItemsRedis[docID] = data.Writes[0].Update.Fields
			savedItemsLocal = savedItemsRedis
		} else {
			savedItemsLocal[docID] = data.Writes[0].Update.Fields
		}

		if config.HAActive {
			if err := cacheStore.SetSavedData("firebase-documents", savedItemsLocal); err != nil {
				logrus.Errorf("Failed to save data to redis: %v", err)
				render.Status(r, http.StatusInternalServerError)
				cancel()
				return
			}
		}

		render.Status(r, http.StatusOK)
		render.JSON(w, r, BatchCommitResponse{
			CommitTime: timestamp,
			WriteResults: []WriteResult{
				WriteResult{UpdateTime: timestamp},
			},
		})
		if config.HAActive {
			cancel()
		}
	}
}

// HandleBatchGet handles batch get requests.
func HandleBatchGet(config *config.Config, redisClient *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var cacheStore *redis.CacheStore
		var cancel context.CancelFunc
		var ctx context.Context
		if config.HAActive {
			ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			var err error
			cacheStore, err = redis.NewCacheStore(ctx, redisClient)
			if err != nil {
				logrus.Errorf("Failed to create cache store: %v", err)
				render.Status(r, http.StatusInternalServerError)
				cancel()
				return
			}
		}
		projectID := chi.URLParam(r, "project_id")
		databaseID := chi.URLParam(r, "database_id")
		logrus.Infof("Got %v and %v", projectID, databaseID)
		data := &BatchGetRequest{}

		// Seems like requests is text/plain but content is json ...
		if err := render.DecodeJSON(r.Body, data); err != nil {
			logrus.Errorf("Failed to decode JSON: %v", err)
			render.Status(r, http.StatusBadRequest)
			if config.HAActive {
				cancel()
			}
			return
		}

		// extract document ID from path
		// - path format: "projects/excalidraw-room-persistence/databases/(default)/documents/scenes/abc123"
		docPath := data.Documents[0]
		key := extractRoomID(docPath)
		logrus.Infof("Got key %v and docPath %v", key, docPath)

		// if HA is active, get saved items from redis, else use local map
		if config.HAActive {
			savedData, err := cacheStore.GetSavedData("firebase-documents")
			if err != nil {
				logrus.Errorf("Failed to get data from redis: %v", err)
				cancel()
			}
			// Only override local cache if we actually have data.
			if len(savedData) > 0 {
				savedItemsLocal = savedData
			} else {
				logrus.Info("Redis returned empty or nil; keeping existing local cache")
			}
		}

		fields, ok := savedItemsLocal[key]

		// the timestamps must be UTC, i.e. no zone offsets allowed
		timestamp := time.Now().UTC().Format(time.RFC3339)
		if !ok {
			logrus.Infof("Missing key, in docpath: %v", docPath)
			render.JSON(w, r, []BatchGetEmptyResponse{BatchGetEmptyResponse{
				Missing:  docPath,
				ReadTime: timestamp,
			}})
			render.Status(r, http.StatusOK)
			if config.HAActive {
				cancel()
			}
			return
		}
		logrus.Infof("Existing key, in docpath: %v", docPath)
		render.Status(r, http.StatusOK)
		render.JSON(w, r, []BatchGetExistsResponse{BatchGetExistsResponse{
			Found: FoundInfoResponse{
				Name:       docPath,
				Fields:     fields,
				CreateTime: timestamp,
				UpdateTime: timestamp,
			},
			ReadTime: timestamp,
		}})
		if config.HAActive {
			cancel()
		}
	}
}

// HandleFetchDocument handles fetching a document for Firestore Listen requests.
func HandleFetchDocument(config *config.Config, redisClient *redis.Client) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		// Check if this is a session initialization or data request
		sid := r.URL.Query().Get("SID")
		reqType := r.URL.Query().Get("TYPE")
		ridParam := r.URL.Query().Get("RID")

		// Set common headers
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Pragma", "no-cache")

		// If SID is present, this is a data streaming request
		if sid != "" {
			handleDataRequest(w, r, config, redisClient, sid, reqType, ridParam)
			return
		}

		// This is a session initialization request
		handleSessionInit(w, r, config, redisClient)
	}
}

// HandleCors handles CORS preflight requests for the Firebase handler.
func HandleCors(allowedOrigins []string) http.HandlerFunc {
	allowedOriginsStr := strings.Join(allowedOrigins, ", ")
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", allowedOriginsStr)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, Content-Length, X-CSRF-Token, Token, session, Origin, Host, Connection, Accept-Encoding, Accept-Language, X-Requested-With, X-Goog-Api-Client, X-Firebase-GMPID, X-HTTP-Session-Id, X-Firebase-Client")
		w.Header().Set("Access-Control-Expose-Headers", "X-HTTP-Session-Id, X-Goog-Channel-Id, X-Goog-Channel-Token")
		w.WriteHeader(http.StatusOK)
	}
}

func handleSessionInit(w http.ResponseWriter, r *http.Request, cfg *config.Config, redisClient *redis.Client) {
	var cacheStore *redis.CacheStore
	var cancel context.CancelFunc
	var ctx context.Context

	if cfg.HAActive {
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var err error
		cacheStore, err = redis.NewCacheStore(ctx, redisClient)
		if err != nil {
			logrus.Errorf("Failed to create cache store for session init: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			cancel()
			return
		}
	}

	if err := r.ParseForm(); err != nil {
		logrus.Errorf("Failed to parse form: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		cancel()
		return
	}

	// Get the listen payload
	rawPayload := r.FormValue("req0___data__")
	if rawPayload == "" {
		rawPayload = r.FormValue("req0__data__")
	}

	var docPath string
	var targetID int
	var database string

	if rawPayload != "" {
		type listenRequest struct {
			Database  string `json:"database"`
			AddTarget struct {
				Documents struct {
					Documents []string `json:"documents"`
				} `json:"documents"`
				TargetID int `json:"targetId"`
			} `json:"addTarget"`
		}

		var req listenRequest
		if err := json.Unmarshal([]byte(rawPayload), &req); err != nil {
			logrus.Errorf("Failed to decode listen payload: %v", err)
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		database = req.Database
		targetID = req.AddTarget.TargetID
		if len(req.AddTarget.Documents.Documents) > 0 {
			docPath = req.AddTarget.Documents.Documents[0]
		}
	}

	// Extract room ID from document path
	roomID := extractRoomID(docPath)

	var session *ListenSession
	var sessionID, gSessionID string

	// In HA mode, check if we already have a session for this room
	if cfg.HAActive && cacheStore != nil && roomID != "" {
		existingSession, err := getSessionByRoomID(cacheStore, roomID)
		if err == nil && existingSession != nil {
			// Reuse existing session for this room
			session = existingSession
			sessionID = session.ID
			gSessionID = session.GSessionID
			// Update target ID and document path if different
			session.TargetID = targetID
			session.DocumentPath = docPath
			logrus.Debugf("Reusing existing session for room %s: SID=%s", roomID, sessionID)
		}
	}

	// If no existing session, create a new one
	if session == nil {
		sessionID = generateSessionID()
		gSessionID = generateGSessionID()

		session = &ListenSession{
			ID:           sessionID,
			GSessionID:   gSessionID,
			Database:     database,
			TargetID:     targetID,
			DocumentPath: docPath,
			AID:          0,
			CreatedAt:    time.Now().Unix(),
		}
	}

	// Store in Redis for HA mode, or local map for single instance
	if cfg.HAActive && cacheStore != nil {
		// Save session by SID
		if err := saveSessionToRedis(cacheStore, session); err != nil {
			logrus.Errorf("Failed to save session to Redis: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			cancel()
			return
		}
		// Also save room -> session mapping for consistency across replicas
		if roomID != "" {
			if err := saveRoomSessionMapping(cacheStore, roomID, session); err != nil {
				logrus.Warnf("Failed to save room session mapping to Redis: %v", err)
			}
		}
		logrus.Debugf("Session saved to Redis: SID=%s, RoomID=%s", sessionID, roomID)
	} else {
		sessionsMu.Lock()
		sessions[sessionID] = session
		sessionsMu.Unlock()
	}

	// Set the gsessionid header
	w.Header().Set("X-HTTP-Session-Id", gSessionID)

	// Send session initialization response
	// Format: <length>\n<data>\n
	// Data format: [[0,["c","<sessionId>","",8,12,30000]]]
	// - "c" indicates connection
	// - sessionId is the SID to use in subsequent requests
	// - "" is host prefix (empty)
	// - 8 is the protocol version
	// - 12 is the server version
	// - 30000 is the timeout in ms
	responseData := fmt.Sprintf(`[[0,["c","%s","",8,12,30000]]]`, sessionID)
	responseLen := len(responseData)

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "%d\n%s", responseLen, responseData)

	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	logrus.Debugf("Session initialized: SID=%s, GSessionID=%s, DocPath=%s", sessionID, gSessionID, docPath)
}

func handleDataRequest(w http.ResponseWriter, r *http.Request, cfg *config.Config, redisClient *redis.Client, sid string, reqType string, ridParam string) {
	var cacheStore *redis.CacheStore
	var session *ListenSession
	var exists bool
	var cancel context.CancelFunc
	var ctx context.Context
	// Get session from Redis (HA mode) or local map
	if cfg.HAActive {
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var err error
		cacheStore, err = redis.NewCacheStore(ctx, redisClient)
		if err != nil {
			logrus.Errorf("Failed to create cache store: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			cancel()
			return
		}

		session, err = getSessionFromRedis(cacheStore, sid)
		if err != nil {
			logrus.Warnf("Session not found in Redis: %s, error: %v", sid, err)
			http.Error(w, "Session not found", http.StatusBadRequest)
			cancel()
			return
		}
		exists = session != nil
	} else {
		sessionsMu.RLock()
		session, exists = sessions[sid]
		sessionsMu.RUnlock()
	}

	if !exists || session == nil {
		logrus.Warnf("Session not found: %s", sid)
		http.Error(w, "Session not found", http.StatusBadRequest)
		cancel()
		return
	}

	// Handle bind request (RID=rpc, TYPE=xmlhttp) - this is a long-polling request for data
	if ridParam == "rpc" && reqType == "xmlhttp" {
		handleLongPoll(w, r, cfg, redisClient, cacheStore, session)
		cancel()
		return
	}

	// Handle regular requests (could be sending more data or acknowledgments)
	if err := r.ParseForm(); err != nil {
		logrus.Errorf("Failed to parse form: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		cancel()
		return
	}

	// Check if there's new data being sent
	rawPayload := r.FormValue("req0___data__")
	if rawPayload == "" {
		rawPayload = r.FormValue("req0__data__")
	}

	if rawPayload != "" {
		// Parse and potentially update the session with new target info
		type listenRequest struct {
			Database  string `json:"database"`
			AddTarget struct {
				Documents struct {
					Documents []string `json:"documents"`
				} `json:"documents"`
				TargetID int `json:"targetId"`
			} `json:"addTarget"`
		}

		var req listenRequest
		if err := json.Unmarshal([]byte(rawPayload), &req); err == nil {
			if len(req.AddTarget.Documents.Documents) > 0 {
				session.DocumentPath = req.AddTarget.Documents.Documents[0]
			}
			session.TargetID = req.AddTarget.TargetID

			// Save updated session
			if cfg.HAActive && cacheStore != nil {
				if err := saveSessionToRedis(cacheStore, session); err != nil {
					logrus.Errorf("Failed to update session in Redis: %v", err)
				}
			} else {
				sessionsMu.Lock()
				sessions[sid] = session
				sessionsMu.Unlock()
			}
		}
	}

	// Send acknowledgment response
	w.WriteHeader(http.StatusOK)
	// Empty response or simple ack
	responseData := "[]"
	fmt.Fprintf(w, "%d\n%s", len(responseData), responseData)

	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func handleLongPoll(w http.ResponseWriter, r *http.Request, cfg *config.Config, redisClient *redis.Client, cacheStore *redis.CacheStore, session *ListenSession) {
	// Create cache store if not provided
	var cancel context.CancelFunc
	var ctx context.Context
	if cfg.HAActive && cacheStore == nil {
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var err error
		cacheStore, err = redis.NewCacheStore(ctx, redisClient)
		if err != nil {
			logrus.Errorf("Failed to create cache store: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			cancel()
			return
		}
	}
	docPath := session.DocumentPath
	roomID := extractRoomID(session.DocumentPath)
	targetID := session.TargetID
	currentAID := session.AID

	// Load document data
	if cfg.HAActive && cacheStore != nil {
		savedData, err := cacheStore.GetSavedData("firebase-documents")
		if err != nil {
			fmt.Printf("redis get error: %v\n", err)
			cancel()
		}
		// Only override local cache if we actually have data.
		if len(savedData) > 0 {
			savedItemsLocal = savedData
		} else {
			fmt.Println("redis returned empty or nil; keeping existing local cache")
		}
	}

	fields, found := savedItemsLocal[roomID]
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000000Z")

	// Build response array matching excalidraw.com's exact format
	// Format: [[AID, [{object}]], [AID, [{object}]], ...]
	var responses []interface{}

	// 1. First send targetChange ADD
	currentAID++
	addTarget := []interface{}{
		currentAID,
		[]interface{}{
			map[string]interface{}{
				"targetChange": map[string]interface{}{
					"targetChangeType": "ADD",
					"targetIds":        []int{targetID},
				},
			},
		},
	}
	responses = append(responses, addTarget)

	// 2. Send document change or delete
	if found && docPath != "" {
		currentAID++
		docChange := []interface{}{
			currentAID,
			[]interface{}{
				map[string]interface{}{
					"documentChange": map[string]interface{}{
						"document": map[string]interface{}{
							"name":       docPath,
							"fields":     fields,
							"createTime": timestamp,
							"updateTime": timestamp,
						},
						"targetIds": []int{targetID},
					},
				},
			},
		}
		responses = append(responses, docChange)
	}

	// 3. Send CURRENT targetChange
	currentAID++
	resumeToken := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	currentTarget := []interface{}{
		currentAID,
		[]interface{}{
			map[string]interface{}{
				"targetChange": map[string]interface{}{
					"targetChangeType": "CURRENT",
					"targetIds":        []int{targetID},
					"resumeToken":      resumeToken,
					"readTime":         timestamp,
				},
			},
		},
	}
	responses = append(responses, currentTarget)

	// 4. Send resumeToken update (no targetIds)
	currentAID++
	resumeUpdate := []interface{}{
		currentAID,
		[]interface{}{
			map[string]interface{}{
				"targetChange": map[string]interface{}{
					"resumeToken": resumeToken,
					"readTime":    timestamp,
				},
			},
		},
	}
	responses = append(responses, resumeUpdate)

	// 5. Send another resumeToken with slightly different time
	currentAID++
	timestamp2 := time.Now().UTC().Add(30 * time.Millisecond).Format("2006-01-02T15:04:05.000000Z")
	resumeUpdate2 := []interface{}{
		currentAID,
		[]interface{}{
			map[string]interface{}{
				"targetChange": map[string]interface{}{
					"resumeToken": resumeToken,
					"readTime":    timestamp2,
				},
			},
		},
	}
	responses = append(responses, resumeUpdate2)

	// Update session AID and save
	session.AID = currentAID
	if cfg.HAActive && cacheStore != nil {
		if err := saveSessionToRedis(cacheStore, session); err != nil {
			logrus.Errorf("Failed to update session AID in Redis: %v", err)
		}
	} else {
		sessionsMu.Lock()
		sessions[session.ID] = session
		sessionsMu.Unlock()
	}

	// Marshal response
	responseBytes, err := json.Marshal(responses)
	if err != nil {
		logrus.Errorf("Failed to marshal response: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		cancel()
		return
	}

	responseData := string(responseBytes)
	responseLen := len(responseData)

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "%d\n%s", responseLen, responseData)

	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	logrus.Debugf("Long poll response sent: AID=%d, found=%v, docPath=%s", currentAID, found, docPath)
}
