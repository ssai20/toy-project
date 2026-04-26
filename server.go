package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Server struct {
	db     *sql.DB
	router *chi.Mux
	logger *slog.Logger
	config *Config
}

func NewServer(config *Config, logger *slog.Logger) (*Server, error) {
	db, err := initDB(config, logger)
	if err != nil {
		return nil, err
	}

	return &Server{
		db:     db,
		router: chi.NewRouter(),
		logger: logger,
		config: config,
	}, nil
}
func initDB(config *Config, logger *slog.Logger) (*sql.DB, error) {
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		config.DBUser, config.DBPassword, config.DBHost,
		config.DBPort, config.DBName, config.DBSSLMode)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("database connection failed: %w", err)
	}

	if err := createTable(db, logger); err != nil {
		return nil, err
	}

	return db, nil
}

func createTable(db *sql.DB, logger *slog.Logger) error {
	queryUserId := `
			CREATE TABLE IF NOT EXISTS user_ids (
			    id SERIAL PRIMARY KEY,
			    user_id VARCHAR(100) UNIQUE NOT NULL,
			    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			);
CREATE INDEX IF NOT EXISTS idx_user_id ON user_ids(user_id);
`
	if _, err := db.Exec(queryUserId); err != nil {
		return fmt.Errorf("failed to create table user_ids: %w", err)
	}

	queryCreds := `CREATE TABLE IF NOT EXISTS credentials (
    					id SERIAL PRIMARY KEY,
    					login VARCHAR(100) UNIQUE NOT NULL,
    					password VARCHAR(100)
    					);`
	if _, err := db.Exec(queryCreds); err != nil {
		return fmt.Errorf("failed to create table credentials: %w", err)
	}

	logger.Info("Tables ready")
	return nil
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	userID := r.FormValue("user_id")
	fmt.Println(userID)
	_, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	query := `
			INSERT INTO user_ids (user_id)
			VALUES ($1)
			ON CONFLICT (user_id) DO NOTHING
			RETURNING id
	`
	var id int64

	err := s.db.QueryRow(query, userID).Scan(&id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		s.logger.Error("Failed to save user", "user_id", userID, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, `{"status":"success", "user_id":"%s", "db_id":%d}`, userID, id)
}

func (s *Server) setupRouters() {
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.Timeout(60 * time.Second))

	s.router.Handle("/templates/*", http.StripPrefix("/templates/", http.FileServer(http.Dir("./templates"))))

	s.router.Route("/api/v1", func(r chi.Router) {
		//r.Use(s.basicAuthMiddleware)

		r.Get("/user", s.handleUserPage)
		r.Post("/user", s.handleCreateUser)
		r.Get("/user/{id}/history", s.handleUserHistory)
		r.Get("/uiks", s.uiksAddresses)
		r.Get("/results", s.results)
	})

	s.router.Get("/", s.handleHome)
	s.router.Get("/health", s.handleHealth)
}

func (s *Server) basicAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()

		if !ok || user != "admin" || pass != "mypassword123" {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) uiksAddresses(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("templates/uiks.html")
	w.Header().Set("Content-Type", "text/html;charset=utf-8")
	if err := tmpl.Execute(w, nil); err != nil {
		s.logger.Error("Failed to execute uiks", "error", err)
	}
	if err != nil {
		s.logger.Error("Failed to parse template", "error", err)
		http.Error(w, "Internal Server UIK error", http.StatusInternalServerError)
		return
	}
}

func (s *Server) results(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("templates/result.html")
	w.Header().Set("Content-Type", "text/html;charset=utf-8")
	if err := tmpl.Execute(w, nil); err != nil {
		s.logger.Error("Failed to execute results", "error", err)
	}
	if err != nil {
		s.logger.Error("Failrd to parse template", "error", err)
		http.Error(w, "Internal Server Results error", http.StatusInternalServerError)
		return
	}
}

var counter int64

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("templates/home.html")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, strconv.FormatInt(atomic.LoadInt64(&counter), 10))

	if err := tmpl.Execute(w, nil); err != nil {
		s.logger.Error("Failed to execute template", "error", err)
	}
	if err != nil {
		s.logger.Error("Failed to parse template", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	atomic.AddInt64(&counter, 1)
}

func (s *Server) handleUserPage(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("templates/user.html")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, nil); err != nil {
		s.logger.Error("Failed to execute template", "error", err)
	}

	if err != nil {
		s.logger.Error("Failed to parse template", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

}

func (s *Server) handleUserHistory(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var createdAt time.Time
	query := `SELECT created_at FROM user_ids WHERE user_id = $1`

	err := s.db.QueryRowContext(ctx, query, userID).Scan(&createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		s.logger.Error("Failed to get user history", "user_id", userID, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"user_id":"%s","created_at":"%s"}`, userID, createdAt.Format(time.RFC3339))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	if err := s.db.PingContext(ctx); err != nil {
		s.logger.Warn("Health check failed", "error", err)
		http.Error(w, "Database connection failed", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"healthy","timestamp":"%s"}`, time.Now().Format(time.RFC3339))
}

func check(err error) {
	if err != nil {
		log.Fatalf("Error is %s", err)
	}
}

func (s *Server) start() {
	srv := &http.Server{
		Addr:         ":" + s.config.AppPort,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		s.logger.Error("Server forced to shutdown", "error", err)
	}

	s.db.Close()
	s.logger.Info("Server exited")
}
