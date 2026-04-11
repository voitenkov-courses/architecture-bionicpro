package httpserver

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	"github.com/voitenkov-courses/architecture-bionicpro/reports-service/internal/config"
	"github.com/voitenkov-courses/architecture-bionicpro/reports-service/internal/services/reports"
)

type Server struct {
	host    string
	port    string
	server  *http.Server
	reports Reports
	logger  Logger
}

type Reports interface {
	GetReportByUserID(ctx context.Context, userID uint32) (*reports.UserReports, error)
}

type Logger interface {
	Error(msg ...interface{})
	Errorf(format string, args ...interface{})
	Info(msg ...interface{})
	Infof(format string, args ...interface{})
	Warn(msg ...interface{})
	Warnf(format string, args ...interface{})
	Debug(msg ...interface{})
	LogHTTPRequest(request *http.Request, duration time.Duration, statusCode int)
}

type Claims struct {
	jwt.RegisteredClaims
}

func NewServer(cfg *config.Config, reports Reports, logger Logger) *Server {
	return &Server{
		host:    cfg.Server.Host,
		port:    cfg.Server.Port,
		logger:  logger,
		reports: reports,
	}
}

func (s *Server) Start(ctx context.Context, host string) error {
	router := mux.NewRouter()

	// Health-check handler
	router.HandleFunc("/health", s.healthcheckHandler).Methods("GET")
	router.HandleFunc("/reports/myreports", s.getMyReportHandler).Methods("GET")
	router.HandleFunc("/reports/{UserID}", s.getReportByUserIDHandler).Methods("GET")

	// Add logging middleware
	router.Use(s.loggingMiddleware)

	server := &http.Server{
		Addr: net.JoinHostPort(s.host, s.port),
		// Addr:              ":" + s.port,
		Handler:           router,
		ReadHeaderTimeout: time.Second * 5,
		BaseContext:       func(_ net.Listener) context.Context { return ctx },
	}

	s.server = server

	go func() {
		err := server.ListenAndServe()
		if err != nil {
			s.logger.Error(err)
		}
	}()

	s.logger.Info("Reports server: http://" + net.JoinHostPort(s.host, s.port))

	<-ctx.Done()
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *Server) healthcheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "up"})
}

// GET /reports/myreport
func (s *Server) getMyReportHandler(w http.ResponseWriter, r *http.Request) {
	userIDFromToken := s.getUserIDFromToken(w, r)
	if userIDFromToken == nil {
		return
	}

	userID, err := strconv.Atoi(*userIDFromToken)
	if err != nil {
		s.logger.Error("UserID from token claim is not convertable to numeric type")
		http.Error(w, "UserID from token claim is not convertable to numeric type", http.StatusBadRequest)
		return
	}

	reports, err := s.reports.GetReportByUserID(r.Context(), uint32(userID))
	if err != nil {
		s.logger.Errorf("Report request failed for UserID %q: %v", userID, err)
		http.Error(w, "Report request failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(reports)
}

// GET /reports/{UserID}
func (s *Server) getReportByUserIDHandler(w http.ResponseWriter, r *http.Request) {
	userIDFromToken := s.getUserIDFromToken(w, r)
	if userIDFromToken == nil {
		return
	}

	vars := mux.Vars(r)
	requestedUserID, ok := vars["UserID"]
	if !ok || requestedUserID == "" {
		s.logger.Error("Failed to get id path parameter.")
		http.Error(w, "Failed to get id path parameter", http.StatusBadRequest)
		return
	}

	if *userIDFromToken != requestedUserID {
		s.logger.Error("Access denied. You can only view your own report.")
		http.Error(w, "Access denied. You can only view your own report.", http.StatusUnauthorized)
		return
	}

	userID, err := strconv.Atoi(*userIDFromToken)
	if err != nil {
		s.logger.Error("UserID from token claim is not convertable to numeric type")
		http.Error(w, "UserID from token claim is not convertable to numeric type", http.StatusBadRequest)
		return
	}

	reports, err := s.reports.GetReportByUserID(r.Context(), uint32(userID))
	if err != nil {
		s.logger.Errorf("Report request failed for UserID %q: %v", userID, err)
		http.Error(w, "Report request failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(reports)
}

func (s *Server) getUserIDFromToken(w http.ResponseWriter, r *http.Request) *string {
	const bearerPrefix = "Bearer "

	var userID string

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, bearerPrefix) {
		s.logger.Error("Missing or invalid Authorization header")
		http.Error(w, "Missing or invalid Authorization header", http.StatusUnauthorized)
		return nil
	}

	userID = r.Header.Get("X-User-Id")
	// Если в заголовке нет UserID, пытаемся достать UserID из JWT Claim
	if userID == "" {
		tokenString := strings.TrimPrefix(authHeader, bearerPrefix)
		claims := &Claims{}

		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
			return []byte("AllYourBase"), nil
		})
		if err != nil {
			s.logger.Errorf("Error parsing token: %v", err)
			http.Error(w, "Cannot parse authorization token", http.StatusUnauthorized)
			return nil
		}

		claims, ok := token.Claims.(*Claims)
		if !ok {
			s.logger.Error("Unknown claims type, cannot proceed")
			http.Error(w, "Unknown claims type, cannot proceed", http.StatusUnauthorized)
			return nil
		}

		userID = claims.Subject
		if userID == "" {
			s.logger.Error("Token missing 'sub' claim")
			http.Error(w, "Token missing 'sub' claim", http.StatusUnauthorized)
			return nil
		}

		return &userID
	}

	return &userID
}
