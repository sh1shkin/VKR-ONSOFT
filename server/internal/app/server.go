package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tender/server/internal/auth"
	"tender/server/internal/config"
	"tender/server/internal/llm"
	"tender/server/internal/models"
	"tender/server/internal/store"
)

type contextKey string

const userContextKey contextKey = "user"

type Server struct {
	cfg      config.Config
	store    *store.Store
	analyzer *llm.Analyzer
	mux      *http.ServeMux
}

type listResponse[T any] struct {
	Items []T `json:"items"`
}

func NewServer(cfg config.Config) (*Server, error) {
	st, err := store.Open(cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	if cfg.EnableDemoSeed {
		if err := st.SeedDemoData(); err != nil {
			return nil, err
		}
	}
	s := &Server{
		cfg:      cfg,
		store:    st,
		analyzer: llm.New(cfg.LLMServiceURL),
		mux:      http.NewServeMux(),
	}
	s.routes()
	return s, nil
}

func (s *Server) Handler() http.Handler { return s.corsMiddleware(s.mux) }

func (s *Server) routes() {
	s.mux.HandleFunc("/api/health", s.handleHealth)
	s.mux.HandleFunc("/api/auth/register", s.handleRegister)
	s.mux.HandleFunc("/api/auth/login", s.handleLogin)
	s.mux.HandleFunc("/api/auth/me", s.requireAuth(s.handleMe))

	s.mux.HandleFunc("/api/dashboard/summary", s.requireAuth(s.handleDashboardSummary))

	s.mux.HandleFunc("/api/tenders", s.requireAuth(s.handleTenders))
	s.mux.HandleFunc("/api/tenders/", s.requireAuth(s.handleTenderByID))

	s.mux.HandleFunc("/api/companies", s.requireAuth(s.handleCompanies))
	s.mux.HandleFunc("/api/companies/", s.requireAuth(s.handleCompanyByID))

	s.mux.HandleFunc("/api/analysis/requests", s.requireAuth(s.handleAnalysisRequests))
	s.mux.HandleFunc("/api/analysis/requests/", s.requireAuth(s.handleAnalysisRequestByID))
	s.mux.HandleFunc("/api/analysis/results/", s.requireAuth(s.handleAnalysisResultByID))

	s.mux.HandleFunc("/api/integration/messages", s.handleIntegrationMessages)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name == "" || req.Email == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "name, email and password are required")
		return
	}
	if _, exists := s.store.FindUserByEmail(req.Email); exists {
		respondError(w, http.StatusConflict, "user with this email already exists")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}
	user, err := s.store.CreateUser(models.User{
		Name: req.Name, Email: strings.ToLower(req.Email), PasswordHash: hash, Role: "analyst",
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create user")
		return
	}
	token, err := auth.GenerateToken(s.cfg.JWTSecret, user.ID, user.Email, 72*time.Hour)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}
	respondJSON(w, http.StatusCreated, map[string]string{"token": token})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	user, ok := s.store.FindUserByEmail(req.Email)
	if !ok || !auth.ComparePassword(user.PasswordHash, req.Password) {
		respondError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	token, err := auth.GenerateToken(s.cfg.JWTSecret, user.ID, user.Email, 72*time.Hour)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	switch r.Method {
	case http.MethodGet:
		respondJSON(w, http.StatusOK, sanitizeUser(user))
	case http.MethodPut:
		var req struct {
			Name     string  `json:"name"`
			Email    string  `json:"email"`
			Password *string `json:"password"`
		}
		if err := decodeJSON(r, &req); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		updated, err := s.store.UpdateUser(user.ID, func(item *models.User) error {
			if req.Name != "" {
				item.Name = req.Name
			}
			if req.Email != "" {
				if existing, ok := s.store.FindUserByEmail(req.Email); ok && existing.ID != user.ID {
					return errors.New("email already in use")
				}
				item.Email = strings.ToLower(req.Email)
			}
			if req.Password != nil && *req.Password != "" {
				hash, err := auth.HashPassword(*req.Password)
				if err != nil {
					return err
				}
				item.PasswordHash = hash
			}
			return nil
		})
		if err != nil {
			status := http.StatusInternalServerError
			if err.Error() == "email already in use" {
				status = http.StatusConflict
			}
			respondError(w, status, err.Error())
			return
		}
		respondJSON(w, http.StatusOK, sanitizeUser(updated))
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleDashboardSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	user := mustUser(r)
	respondJSON(w, http.StatusOK, s.store.DashboardSummary(user.ID))
}

func (s *Server) handleTenders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	items := s.store.ListTenders(r.URL.Query().Get("search"))
	respondJSON(w, http.StatusOK, listResponse[models.Tender]{Items: items})
}

func (s *Server) handleTenderByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	id, ok := parseIDFromPath(r.URL.Path, "/api/tenders/")
	if !ok {
		respondError(w, http.StatusBadRequest, "invalid tender id")
		return
	}
	item, exists := s.store.GetTender(id)
	if !exists {
		respondError(w, http.StatusNotFound, "tender not found")
		return
	}
	respondJSON(w, http.StatusOK, item)
}

func (s *Server) handleCompanies(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	switch r.Method {
	case http.MethodGet:
		items := s.store.ListCompanies(user.ID, r.URL.Query().Get("search"))
		respondJSON(w, http.StatusOK, listResponse[models.Company]{Items: items})
	case http.MethodPost:
		var req models.Company
		if err := decodeJSON(r, &req); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		req.OwnerID = user.ID
		item, err := s.store.CreateCompany(req)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to create company")
			return
		}
		respondJSON(w, http.StatusCreated, item)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleCompanyByID(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	id, ok := parseIDFromPath(r.URL.Path, "/api/companies/")
	if !ok {
		respondError(w, http.StatusBadRequest, "invalid company id")
		return
	}
	switch r.Method {
	case http.MethodGet:
		item, exists := s.store.GetCompany(id, user.ID)
		if !exists {
			respondError(w, http.StatusNotFound, "company not found")
			return
		}
		respondJSON(w, http.StatusOK, item)
	case http.MethodPut:
		var req models.Company
		if err := decodeJSON(r, &req); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		item, err := s.store.UpdateCompany(id, user.ID, req)
		if err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondJSON(w, http.StatusOK, item)
	case http.MethodDelete:
		if err := s.store.DeleteCompany(id, user.ID); err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondJSON(w, http.StatusOK, map[string]bool{"deleted": true})
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleAnalysisRequests(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	switch r.Method {
	case http.MethodGet:
		requests := s.store.ListAnalysisRequests(user.ID)
		items := make([]models.AnalysisRequestListItem, 0, len(requests))
		for _, req := range requests {
			tender, _ := s.store.GetTender(req.TenderID)
			company, _ := s.store.GetCompany(req.CompanyID, user.ID)
			items = append(items, models.AnalysisRequestListItem{
				ID: req.ID, TenderSubject: tender.PurchaseSubject, CompanyName: company.CompanyName,
				Status: req.Status, FinalDecisionLabel: req.FinalDecisionLabel, ResultID: req.ResultID, CreatedAt: req.CreatedAt,
			})
		}
		respondJSON(w, http.StatusOK, listResponse[models.AnalysisRequestListItem]{Items: items})
	case http.MethodPost:
		var req struct {
			TenderID  int64 `json:"tenderId"`
			CompanyID int64 `json:"companyId"`
		}
		if err := decodeJSON(r, &req); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		tender, ok := s.store.GetTender(req.TenderID)
		if !ok {
			respondError(w, http.StatusNotFound, "tender not found")
			return
		}
		company, ok := s.store.GetCompany(req.CompanyID, user.ID)
		if !ok {
			respondError(w, http.StatusNotFound, "company not found")
			return
		}

		createdReq, err := s.store.CreateAnalysisRequest(models.AnalysisRequest{
			TenderID: req.TenderID, CompanyID: req.CompanyID, UserID: user.ID, Status: "processing",
		})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to create analysis request")
			return
		}

		result, err := s.analyzer.Analyze(tender, company)
		if err != nil {
			_, _ = s.store.UpdateAnalysisRequest(createdReq.ID, func(item *models.AnalysisRequest) error {
				item.Status = "failed"
				return nil
			})
			respondError(w, http.StatusBadGateway, fmt.Sprintf("analysis failed: %v", err))
			return
		}
		result.AnalysisRequestID = createdReq.ID
		savedResult, err := s.store.CreateAnalysisResult(result)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to save analysis result")
			return
		}
		updatedReq, err := s.store.UpdateAnalysisRequest(createdReq.ID, func(item *models.AnalysisRequest) error {
			item.Status = "completed"
			item.ResultID = savedResult.ID
			item.FinalDecisionLabel = savedResult.FinalDecision.Label
			return nil
		})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to update analysis request")
			return
		}
		respondJSON(w, http.StatusCreated, map[string]any{
			"id":       updatedReq.ID,
			"resultId": savedResult.ID,
			"status":   updatedReq.Status,
		})
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleAnalysisRequestByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	user := mustUser(r)
	id, ok := parseIDFromPath(r.URL.Path, "/api/analysis/requests/")
	if !ok {
		respondError(w, http.StatusBadRequest, "invalid analysis request id")
		return
	}
	item, exists := s.store.GetAnalysisRequest(id)
	if !exists || item.UserID != user.ID {
		respondError(w, http.StatusNotFound, "analysis request not found")
		return
	}
	respondJSON(w, http.StatusOK, item)
}

func (s *Server) handleAnalysisResultByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	user := mustUser(r)
	id, ok := parseIDFromPath(r.URL.Path, "/api/analysis/results/")
	if !ok {
		respondError(w, http.StatusBadRequest, "invalid analysis result id")
		return
	}
	item, exists := s.store.GetAnalysisResult(id)
	if !exists {
		respondError(w, http.StatusNotFound, "analysis result not found")
		return
	}
	req, ok := s.store.GetAnalysisRequest(item.AnalysisRequestID)
	if !ok || req.UserID != user.ID {
		respondError(w, http.StatusNotFound, "analysis result not found")
		return
	}
	respondJSON(w, http.StatusOK, item)
}

func (s *Server) handleIntegrationMessages(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if _, ok := s.authenticateRequest(r); !ok {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		items := s.store.ListIntegrationMessages()
		respondJSON(w, http.StatusOK, listResponse[models.IntegrationMessage]{Items: items})
	case http.MethodPost:
		if s.cfg.IntegrationAPIKey != "" && r.Header.Get("X-Integration-Key") != s.cfg.IntegrationAPIKey {
			respondError(w, http.StatusUnauthorized, "invalid integration key")
			return
		}
		var req struct {
			ExternalMessageID string        `json:"externalMessageId"`
			EventType         string        `json:"eventType"`
			Description       string        `json:"description"`
			Tender            models.Tender `json:"tender"`
		}
		if err := decodeJSON(r, &req); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		if req.EventType == "" {
			req.EventType = "tender_received"
		}
		payloadBytes, _ := json.Marshal(req)
		msg, err := s.store.CreateIntegrationMessage(models.IntegrationMessage{
			ExternalMessageID: req.ExternalMessageID,
			EventType:         req.EventType,
			ProcessingStatus:  "processed",
			Description:       req.Description,
			Payload:           string(payloadBytes),
		})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to save integration message")
			return
		}
		req.Tender.SourceMessageID = msg.ExternalMessageID
		tender, err := s.store.CreateTender(req.Tender)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to save tender")
			return
		}
		respondJSON(w, http.StatusCreated, map[string]any{"message": msg, "tender": tender})
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) authenticateRequest(r *http.Request) (models.User, bool) {
	header := r.Header.Get("Authorization")
	if header == "" || !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return models.User{}, false
	}
	token := strings.TrimSpace(header[7:])
	claims, err := auth.ParseToken(s.cfg.JWTSecret, token)
	if err != nil {
		return models.User{}, false
	}
	user, ok := s.store.GetUser(claims.UserID)
	return user, ok
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := s.authenticateRequest(r)
		if !ok {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		ctx := context.WithValue(r.Context(), userContextKey, user)
		next(w, r.WithContext(ctx))
	}
}

func mustUser(r *http.Request) models.User {
	user, _ := r.Context().Value(userContextKey).(models.User)
	return user
}

func sanitizeUser(user models.User) map[string]any {
	return map[string]any{
		"id":        user.ID,
		"name":      user.Name,
		"email":     user.Email,
		"role":      user.Role,
		"createdAt": user.CreatedAt,
		"updatedAt": user.UpdatedAt,
	}
}

func respondJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"message": message})
}

func methodNotAllowed(w http.ResponseWriter) {
	respondError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func parseIDFromPath(path, prefix string) (int64, bool) {
	raw := strings.TrimPrefix(path, prefix)
	if raw == path || raw == "" {
		return 0, false
	}
	id, err := strconv.ParseInt(strings.Trim(raw, "/"), 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = s.cfg.ClientOrigin
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Integration-Key")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
