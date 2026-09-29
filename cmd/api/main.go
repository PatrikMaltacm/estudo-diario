package main

import (
	"database/sql"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/PatrikMaltacm/estudo-diario/internal/auth"
	"github.com/PatrikMaltacm/estudo-diario/internal/handler"
	"github.com/PatrikMaltacm/estudo-diario/internal/middleware"
	webstatic "github.com/PatrikMaltacm/estudo-diario/web"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(".env"); err != nil {
		godotenv.Load()
	}

	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		log.Fatal("JWT_SECRET deve ter pelo menos 32 caracteres")
	}

	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		log.Fatal("A variavel de ambiente DATABASE_URL não foi definida")
	}

	db, err := sql.Open("pgx", connString)
	if err != nil {
		log.Fatalf("Erro ao configurar a conexão: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Erro ao conectar no banco de dados: %v", err)
	}

	log.Println("Banco de dados conectado")

	// staticFS expõe os arquivos da pasta web/ embutida no binário
	staticFS, err := fs.Sub(webstatic.FS, ".")
	if err != nil {
		log.Fatalf("Erro ao configurar assets estáticos: %v", err)
	}

	app := &handler.Application{
		DB:        db,
		JWTSecret: []byte(secret),
		StaticFS:  staticFS,
	}

	mux := http.NewServeMux()

	// Rate limiter global (20 req/s, burst 40)
	globalLimiter := middleware.RateLimiter(20, 40)

	// Rate limiter severo (2 req/s, burst 5) para endpoints sensíveis
	severeLimiter := middleware.RateLimiter(2, 5)

	mux.HandleFunc("GET /{$}", app.HomeHandler)
	mux.Handle("POST /user", severeLimiter(http.HandlerFunc(app.CreateUserHandler)))
	mux.Handle("POST /login", severeLimiter(http.HandlerFunc(app.LoginHandler)))
	mux.HandleFunc("GET /ranking", app.RankingHandler)
	mux.HandleFunc("GET /study/today", auth.RequireAuth(app.JWTSecret, app.TodayStudyHandler))
	mux.Handle("POST /study/start", severeLimiter(auth.RequireAuth(app.JWTSecret, app.StartStudyHandler)))
	mux.Handle("POST /send-summary", severeLimiter(auth.RequireAuth(app.JWTSecret, app.SendSummaryHandler)))

	log.Println("Servidor Rodando")
	if err := http.ListenAndServe(":8080", globalLimiter(mux)); err != nil {
		log.Fatal(err)
	}
}
