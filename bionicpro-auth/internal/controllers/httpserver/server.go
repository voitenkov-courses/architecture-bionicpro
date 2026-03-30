package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gorilla/mux"
	cv "github.com/nirasan/go-oauth-pkce-code-verifier"
	"github.com/voitenkov-courses/architecture-bionicpro/bionicpro-auth/internal/config"
	"github.com/voitenkov-courses/architecture-bionicpro/bionicpro-auth/internal/services/auth"
	"github.com/voitenkov-courses/architecture-bionicpro/bionicpro-auth/internal/services/session"
	"golang.org/x/oauth2"
)

type Server struct {
	host         string
	port         string
	server       *http.Server
	auth         *auth.AuthContext
	logger       Logger
	randomSource *rand.Rand
	sessionStore *session.SessionStore
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

func NewServer(cfg *config.Config, auth *auth.AuthContext, logger Logger, randomSource *rand.Rand, sessionStore *session.SessionStore) *Server {
	return &Server{
		host:         cfg.Server.Host,
		port:         cfg.Server.Port,
		auth:         auth,
		logger:       logger,
		randomSource: randomSource,
		sessionStore: sessionStore,
	}
}

func (s *Server) Start(ctx context.Context, host string) error {
	router := mux.NewRouter()

	// Health-check handler
	router.HandleFunc("/health", s.healthcheckHandler).Methods("GET")

	// Все запросы /api/reports проксируются к downstream-сервисам с Bearer Token
	router.HandleFunc("/api/reports", s.proxyHandler).Methods("GET")
	router.HandleFunc("/api/reports", s.proxyHandler).Methods("POST")

	// Authorization handlers
	router.HandleFunc("/auth/login", s.loginHandler).Methods("GET")
	router.HandleFunc("/auth/callback", s.callbackHandler).Methods("GET")
	router.HandleFunc("/auth/logout", s.logoutHandler).Methods("POST")

	// Add logging middleware
	router.Use(s.loggingMiddleware)

	server := &http.Server{
		Addr:              net.JoinHostPort(s.host, s.port),
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

	s.logger.Info("Auth server (BFF): http://" + net.JoinHostPort(s.host, s.port))
	s.logger.Infof("Keycloack: %v/realms/%v/", s.auth.Cfg.KeycloakURL, s.auth.Cfg.KeycloakRealm)
	s.logger.Infof("Frontend: %v", s.auth.Cfg.FrontendURL)
	s.logger.Info("Callback: http://" + net.JoinHostPort(s.host, s.port) + "/auth/callback")

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

// GET/POST /api/**
func (s *Server) proxyHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Извлекаем session_id из куки
	cookie, err := r.Cookie(s.auth.Cfg.SessionCookieName)
	if err != nil {
		s.logger.Errorf("No session cookie: %v", err)
		http.Error(w, "No session cookie", http.StatusUnauthorized)
		return
	}

	// 2. Атомарно получаем и удаляем старую сессию
	sessionID := cookie.Value
	sessionData := s.sessionStore.GetAndDeleteSession(sessionID)
	if sessionData == nil {
		s.logger.Warn("Session not found or expire: %v", sessionID)
		// Очищаем cookie
		cookie = &http.Cookie{
			Name:   s.auth.Cfg.SessionCookieName,
			Value:  "",
			Path:   "/",
			MaxAge: -1, // Удаление куки
		}
		http.SetCookie(w, cookie)
		http.Error(w, "Session expired. Please login again.", http.StatusUnauthorized)
		return
	}

	// 3. Если access_token истёк — обновляем через refresh_token
	// Проверяем, истёк ли access_token, с запасом 10 секунд
	if sessionData.AccessTokenExpiresAt.Compare(time.Now()) < 0 {
		s.logger.Debug("Access token expired for user %q, refreshing...", sessionData.Username)
		ctx := r.Context()
		ts := s.auth.Oauth2Config.TokenSource(ctx, &oauth2.Token{RefreshToken: sessionData.RefreshToken})

		newToken, err := ts.Token()
		if err != nil {
			s.logger.Infof("Token refresh failed for user %q, session terminated", sessionData.Username)
			// Очищаем cookie
			cookie = &http.Cookie{
				Name:   s.auth.Cfg.SessionCookieName,
				Value:  "",
				Path:   "/",
				MaxAge: -1, // Удаление куки
			}
			http.SetCookie(w, cookie)
			http.Error(w, "Failed to refresh token", http.StatusUnauthorized)
			return
		}

		sessionData.AccessToken = newToken.AccessToken
		sessionData.RefreshToken = newToken.RefreshToken
		sessionData.AccessTokenExpiresAt = newToken.Expiry
		s.logger.Debug("Token refreshed for user %q", sessionData.Username)
	}

	// 4. Генерируем новый session_id (ротация — session fixation protection)
	newSessionID := generateSecureID()
	sessionData.CreatedAt = time.Now()
	sessionData.LastAccessedAt = time.Now()

	// 5. Сохраняет сессию под новым session_id
	s.sessionStore.PutSession(newSessionID, *sessionData)

	// 6. Обновляем cookie
	cookie = &http.Cookie{
		Name:     s.auth.Cfg.SessionCookieName,
		Value:    newSessionID,
		Path:     "/",
		MaxAge:   int(s.auth.Cfg.SessionTTLSeconds.Seconds()),
		HttpOnly: true, // Доступ только через HTTP, защита от XSS
		// Secure:   true, // Только HTTPS
		SameSite: http.SameSiteStrictMode, // Защита от CSRF
	}
	http.SetCookie(w, cookie)

	// 7. Проксируем запрос к downstream API с Authorization: Bearer <token>
	// 7.1 Определяем downstream URL
	path := r.URL.RawPath

	// 7.2 Убираем /api prefix при проксировании
	downstreamPath := strings.TrimPrefix(path, "/api")
	var downstreamBase string

	if strings.HasPrefix(path, "/api/reports") {
		downstreamBase = s.auth.Cfg.ReportServiceURL
	} else {
		downstreamBase = s.auth.Cfg.APIBaseURL
	}

	downstreamURL := downstreamBase + downstreamPath
	query := r.URL.RawQuery
	if query != "" {
		downstreamURL = downstreamURL + "?" + query
	}

	// 7.3 Проксируем метод и тело
	client := http.Client{}

	if r.Method == http.MethodPost {
		if r.Body != nil {
			defer r.Body.Close()
			reqBody, err := io.ReadAll(r.Body)
			if err != nil {
				s.logger.Errorf("Error reading proxied request body: %v", err)
				http.Error(w, "Error reading proxied request body", http.StatusInternalServerError)
				return
			}

			req, err := http.NewRequest("POST", downstreamURL, bytes.NewBuffer(reqBody))
			if err != nil {
				s.logger.Errorf("Error creating proxied request: %v", err)
				http.Error(w, "Error creating proxied reques", http.StatusInternalServerError)
				return
			}

			req.Header.Add("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				s.logger.Errorf("Error reading proxied response: %v", err)
				http.Error(w, "Error reading proxied response", http.StatusInternalServerError)
				return
			}

			if resp.Body != nil {
				defer resp.Body.Close()
				respBody, err := io.ReadAll(r.Body)
				if err != nil {
					s.logger.Errorf("Error reading proxied response body: %v", err)
					http.Error(w, "Error reading proxied response body", http.StatusInternalServerError)
					return
				}
				w.WriteHeader(resp.StatusCode)
				w.Header().Set("Content-Type", "application/json")
				w.Write(respBody)
			} else {
				w.WriteHeader(resp.StatusCode)
			}
		} else {
			req, err := http.NewRequest("POST", downstreamURL, nil)
			if err != nil {
				s.logger.Errorf("Error creating proxied request: %v", err)
				http.Error(w, "Error creating proxied reques", http.StatusInternalServerError)
				return
			}

			req.Header.Add("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				s.logger.Errorf("Error reading proxied response: %v", err)
				http.Error(w, "Error reading proxied response", http.StatusInternalServerError)
				return
			}

			if resp.Body != nil {
				defer resp.Body.Close()
				respBody, err := io.ReadAll(r.Body)
				if err != nil {
					s.logger.Errorf("Error reading proxied response body: %v", err)
					http.Error(w, "Error reading proxied response body", http.StatusInternalServerError)
					return
				}
				w.WriteHeader(resp.StatusCode)
				w.Header().Set("Content-Type", "application/json")
				w.Write(respBody)
			} else {
				w.WriteHeader(resp.StatusCode)
			}
		}
	} else {
		req, err := http.NewRequest("GET", downstreamURL, nil)
		if err != nil {
			s.logger.Errorf("Error creating proxied request: %v", err)
			http.Error(w, "Error creating proxied reques", http.StatusInternalServerError)
			return
		}

		req.Header.Add("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			s.logger.Errorf("Error reading proxied response: %v", err)
			http.Error(w, "Error reading proxied response", http.StatusInternalServerError)
			return
		}

		if resp.Body != nil {
			defer resp.Body.Close()
			respBody, err := io.ReadAll(r.Body)
			if err != nil {
				s.logger.Errorf("Error reading proxied response body: %v", err)
				http.Error(w, "Error reading proxied response body", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(resp.StatusCode)
			w.Header().Set("Content-Type", "application/json")
			w.Write(respBody)
		} else {
			w.WriteHeader(resp.StatusCode)
		}
	}
}

// GET /auth/login
func (s *Server) loginHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Генерируем Code Verifier для PKCE
	var v, _ = cv.CreateCodeVerifier()

	verifier := v.String()

	// 2. Генерируем State
	state := s.generateRandomState()

	// 3. Получаем URL для Keycloack
	redirectURL := s.auth.Oauth2Config.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))

	// 4. Сохраняем State и Code Verifier (серверная сторона, фронтенд его не видит)
	s.sessionStore.PutState(session.State(state), verifier)
	s.logger.Infof("Login initiated, redirecting to Keycloak (state: %v, verifyer: %v)", state, verifier)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// POST /auth/logout
func (s *Server) logoutHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Извлекаем session_id из куки
	cookie, err := r.Cookie(s.auth.Cfg.SessionCookieName)
	if err != nil {
		s.logger.Errorf("No session cookie: %v", err)
		http.Error(w, "No session cookie", http.StatusNotFound)
		return
	}

	// 2. Удаляем серверную сессию
	sessionID := cookie.Value
	if sessionID != "" {
		s.sessionStore.DeleteSession(sessionID)
		s.logger.Infof("Logout: session %q removed)", sessionID)
	}

	// 3. Очищаем cookie (Max-Age=0)
	cookie = &http.Cookie{
		Name:   s.auth.Cfg.SessionCookieName,
		Value:  "",
		Path:   "/",
		MaxAge: -1, // Удаление куки
	}

	http.SetCookie(w, cookie)

	// 4. Редиректим на Keycloak logout (SSO logout для всех клиентов)
	logoutURL := s.auth.GetLogoutURL()
	if logoutURL == "" {
		s.logger.Error("Get empty LogoutURL from config")
		http.Error(w, "Get empty LogoutURL from config", http.StatusInternalServerError)
		return
	}

	parsedLogoutURL, err := url.Parse(logoutURL)
	if err != nil {
		s.logger.Errorf("Can't parse LogoutURL from config: %v", err)
		http.Error(w, "Can't parse LogoutURL from config", http.StatusInternalServerError)
		return
	}

	params := url.Values{}
	params.Add("post_logout_redirect_uri", s.auth.Cfg.FrontendURL)
	params.Add("client_id", s.auth.Cfg.ClientID)
	parsedLogoutURL.RawQuery = params.Encode()
	redirectURL := parsedLogoutURL.String()
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// GET /auth/callback?code=...&state=...
// Keycloak редиректит сюда после успешной аутентификации.
func (s *Server) callbackHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Получаем временный authorization code, который отправил Keycloak
	code := r.URL.Query().Get("code")
	// Получаем state, который отправил Keycloak
	state := r.URL.Query().Get("state")

	error := r.URL.Query().Get("error")
	if error != "" {
		s.logger.Warnf("Keycloak returned error: %v - %v", error, r.URL.Query().Get("error_description"))
		redirectURL := s.auth.Cfg.FrontendURL + "?error=" + error
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	// Проверяем наличие обязательных параметров
	if code == "" || state == "" {
		s.logger.Warn("Missing code or state in callback")
		http.Error(w, "Missing code or state", http.StatusUnauthorized)
		return
	}

	// 1. Проверяем state (защита от CSRF-атаки) и извлекаем code verifier
	codeVerifier := s.sessionStore.GetAndDeleteState(session.State(state))
	if codeVerifier == nil {
		s.logger.Warn("Invalid or expired state: %v", state)
		http.Error(w, "Invalid or expired state. Please login again.", http.StatusUnauthorized)
		return
	}

	// 2. Обмениваем authorization code и code verifier  на настоящие токены
	token, err := s.auth.Oauth2Config.Exchange(ctx, code, oauth2.VerifierOption(*codeVerifier))
	if err != nil {
		s.logger.Errorf("Failed to exchange token: %v", err)
		redirectURL := s.auth.Cfg.FrontendURL + "?error=token_exchange_failed"
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	// id_token - это JWT, который содержит данные о пользователе (имя, email и прочее)
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		s.logger.Error("No id_token field")
		http.Error(w, "No id_token field", http.StatusInternalServerError)
		return
	}

	// 3. Выполняем верификацию id_token
	verifier := s.auth.OidcProvider.Verifier(&oidc.Config{ClientID: s.auth.Cfg.ClientID})

	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		http.Error(w, "Failed to verify ID token", http.StatusInternalServerError)
		return
	}

	var claims map[string]any

	if err := idToken.Claims(&claims); err != nil {
		http.Error(w, "Failed to parse claims", http.StatusInternalServerError)
		return
	}

	// 4. Создаём серверную сессию
	sessionID := generateSecureID()
	sessionData := session.Session{
		AccessToken:          token.AccessToken,
		RefreshToken:         token.RefreshToken,
		UserID:               fmt.Sprintf("%v", claims["sub"]),
		Username:             fmt.Sprintf("%v", claims["username"]),
		Email:                fmt.Sprintf("%v", claims["email"]),
		Roles:                fmt.Sprintf("%v", claims["roles"]),
		CreatedAt:            time.Now(),
		LastAccessedAt:       time.Now(),
		AccessTokenExpiresAt: token.Expiry,
	}

	s.sessionStore.PutSession(sessionID, sessionData)
	s.logger.Infof("Login succesful: user=%q, roles=%q (session: %v)", claims["name"], claims["roles"], sessionID)

	// 5. Отдаём фронтенду HttpOnly/SameSite cookie и редиректим на главную
	cookie := &http.Cookie{
		Name:     s.auth.Cfg.SessionCookieName,
		Value:    sessionID,
		Path:     "/",
		MaxAge:   int(s.auth.Cfg.SessionTTLSeconds.Seconds()),
		HttpOnly: true, // Доступ только через HTTP, защита от XSS
		// Secure:   true, // Только HTTPS
		SameSite: http.SameSiteStrictMode, // Защита от CSRF
	}

	http.SetCookie(w, cookie)
	redirectURL := s.auth.Cfg.FrontendURL
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (s *Server) generateRandomState() string {
	return fmt.Sprintf("%d", s.randomSource.IntN(100000))
}

func (s *Server) Cleanup() {
	removed := s.sessionStore.CleanupStates()
	if removed > 0 {
		s.logger.Infof("Cleanup states: removed %v expired states", removed)
	}

	removed = s.sessionStore.CleanupSessions()
	if removed > 0 {
		s.logger.Infof("Cleanup sessions: removed %v expired sessions", removed)
	}
}

func (s *Server) ScheduledCleanup(ctx context.Context) error {
	ticker := time.NewTicker(s.auth.Cfg.CleanupPeriodSeconds)
	stop := make(chan bool)

	go func() {
		defer func() { stop <- true }()
		for {
			select {
			case <-ticker.C:
				s.Cleanup()
			case <-stop:
				return
			}
		}
	}()

	<-ctx.Done()
	s.logger.Errorf("%v", ctx.Err())
	ticker.Stop()
	stop <- true
	<-stop
	return nil
}

func generateSecureID() string {
	// Можем использовать метод OAuth 2.0 для генерации Code Verifier
	return oauth2.GenerateVerifier()
}
