package main

import (
	//"database/sql"
	//"fmt"
	//"html/template"
	//"log"
	"log/slog"
	"toyProject/messagebroker"

	//"net/http"
	"os"

	//"github.com/go-chi/chi/v5"
	//"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

//
//var db *sql.DB
//
//func main() {
//	dbHost := getEnv("DB_HOST", "localhost")
//	dbPort := getEnv("DB_PORT", "5432")
//	dbUser := getEnv("DB_USER", "myuser")
//	dbPassword := getEnv("DB_PASSWORD", "mypassword")
//	dbName := getEnv("DB_NAME", "mydatabase")
//	dbSSLMode := getEnv("DB_SSLMODE", "disable")
//
//	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", dbUser, dbPassword, dbHost, dbPort, dbName, dbSSLMode)
//
//	var err error
//	db, err = sql.Open("postgres", connStr)
//	if err != nil {
//		panic(err)
//	}
//	defer func(db *sql.DB) {
//		err := db.Close()
//		if err != nil {
//			panic(err)
//		}
//	}(db)
//
//	err = db.Ping()
//	if err != nil {
//		panic(fmt.Sprintf("Cannot connect to DB: %v", err))
//	}
//	fmt.Println("✅ Connected to database")
//	_, err = db.Exec(`
//			CREATE TABLE IF NOT EXISTS user_ids (
//			    id SERIAL PRIMARY KEY,
//			    user_id VARCHAR(100) UNIQUE NOT NULL,
//			    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
//			)
//`)
//	if err != nil {
//		panic(fmt.Sprintf("Error creating table: %v", err))
//	}
//	fmt.Println("✅ Table ready")
//
//	r := chi.NewRouter()
//
//	r.Use(middleware.Logger)
//	r.Use(middleware.Recoverer)
//	r.Use(middleware.RequestID)
//
//	r.Get("/user", handler)
//	r.Post("/user/create", create)
//	r.Get("/bd/{id}", history)
//
//	//http.HandleFunc("/user/{id}", handler)
//	//http.HandleFunc("/bd/{id}", history)
//	templateText := "App is running...\nAction: {{.}}\n"
//	tmpl, err := template.New("test").Parse(templateText)
//	check(err)
//	err = tmpl.Execute(os.Stdout, "Postgress too...")
//	check(err)
//	appPort := getEnv("APP_PORT", "8086")
//	err = http.ListenAndServe(":"+appPort, r)
//	if err != nil {
//		panic(err)
//	}
//}
//
//func executeTemplate(text string, data interface{}) {
//	tmpl, err := template.New("test").Parse(text)
//	check(err)
//	err = tmpl.Execute(os.Stdout, data)
//	check(err)
//}
//
////func getEnv(key, defaultValue string) string {
////	if value := os.Getenv(key); value != "" {
////		return value
////	}
////	return defaultValue
////}
//
//func check(error error) {
//	if error != nil {
//		log.Fatal(error)
//	}
//}
//
//func create(res http.ResponseWriter, req *http.Request) {
//	id := req.FormValue("signature")
//	_, err := res.Write([]byte(id))
//	check(err)
//	fmt.Fprintf(res, "\nOk!")
//
//	if id == "favicon.ico" {
//		res.WriteHeader(http.StatusNoContent)
//		return
//	}
//	_, err = db.Exec(`
//			INSERT INTO user_ids (user_id)
//			VALUES ($1)
//			ON CONFLICT (user_id) DO NOTHING
//`, id)
//	if err != nil {
//		fmt.Fprintf(res, "Error saving: %v", err)
//		return
//	}
//
//	fmt.Fprintf(res, "Hello Sairan! Сохранил данные в таблицу: %s", id)
//
//	http.Redirect(res, req, "/user", http.StatusCreated)
//
//}
//
//func history(res http.ResponseWriter, req *http.Request) {
//	//id := req.PathValue("id")
//	id := chi.URLParam(req, "id")
//
//	var t string
//	err := db.QueryRow(`
//			SELECT created_at FROM user_ids WHERE user_id = $1
//`, id).Scan(&t)
//	if err != nil {
//		fmt.Fprintf(res, "Error selecting id: %v", err)
//		return
//	}
//	fmt.Fprintf(res, "Hello Sairan! Прими время сохранения %s: %s", id, t)
//}
//
//func handler(res http.ResponseWriter, req *http.Request) {
//	//id := req.PathValue("id")
//	//id := chi.URLParam(req, "id")
//
//	html, err := template.ParseFiles("view.html")
//	check(err)
//	err = html.Execute(res, nil)
//	check(err)
//
//}

func main() {
	if err := godotenv.Load(); err != nil {
		slog.Warn("No .env file found")
	}

	config := loadConfig()

	logger := setupLogger(config.Env)

	server, err := NewServer(config, logger)
	if err != nil {
		logger.Error("Failed to create server", "error", err)
		os.Exit(1)
	}
	server.setupRouters()

	go messagebroker.CreateKafkaConsumer(server.db, config.KafkaBroker, config.KafkaCert, config.KafkaKey, config.KafkaCA, "credential-group", "test_topic")

	server.start()

}
