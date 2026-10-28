package main

import (
	"context"
	"database/sql"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/PatrikMaltacm/estudo-diario/internal/auth"
	"github.com/PatrikMaltacm/estudo-diario/internal/handler"
	"github.com/PatrikMaltacm/estudo-diario/internal/i18n"
	"github.com/PatrikMaltacm/estudo-diario/internal/middleware"
	"github.com/PatrikMaltacm/estudo-diario/internal/repository"
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

	if err := repository.EnsureSchema(context.Background(), db); err != nil {
		log.Fatalf("Erro ao aplicar schema do banco: %v", err)
	}

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
	mux.HandleFunc("GET /robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Robots-Tag", "all")
		w.Write([]byte("User-agent: *\nAllow: /\n\nSitemap: https://estudodiario.online/sitemap.xml\n"))
	})
	mux.HandleFunc("GET /sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		sitemap := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"
        xmlns:xhtml="http://www.w3.org/1999/xhtml">
  <url>
    <loc>https://estudodiario.online/</loc>
    <xhtml:link rel="alternate" hreflang="pt-BR" href="https://estudodiario.online/?lang=pt-BR"/>
    <xhtml:link rel="alternate" hreflang="en-US" href="https://estudodiario.online/?lang=en-US"/>
    <xhtml:link rel="alternate" hreflang="es-ES" href="https://estudodiario.online/?lang=es-ES"/>
    <xhtml:link rel="alternate" hreflang="x-default" href="https://estudodiario.online/"/>
    <changefreq>daily</changefreq>
    <priority>1.0</priority>
  </url>
  <url>
    <loc>https://estudodiario.online/?lang=pt-BR</loc>
    <xhtml:link rel="alternate" hreflang="pt-BR" href="https://estudodiario.online/?lang=pt-BR"/>
    <xhtml:link rel="alternate" hreflang="en-US" href="https://estudodiario.online/?lang=en-US"/>
    <xhtml:link rel="alternate" hreflang="es-ES" href="https://estudodiario.online/?lang=es-ES"/>
    <xhtml:link rel="alternate" hreflang="x-default" href="https://estudodiario.online/"/>
    <changefreq>daily</changefreq>
    <priority>0.9</priority>
  </url>
  <url>
    <loc>https://estudodiario.online/?lang=en-US</loc>
    <xhtml:link rel="alternate" hreflang="pt-BR" href="https://estudodiario.online/?lang=pt-BR"/>
    <xhtml:link rel="alternate" hreflang="en-US" href="https://estudodiario.online/?lang=en-US"/>
    <xhtml:link rel="alternate" hreflang="es-ES" href="https://estudodiario.online/?lang=es-ES"/>
    <xhtml:link rel="alternate" hreflang="x-default" href="https://estudodiario.online/"/>
    <changefreq>daily</changefreq>
    <priority>0.9</priority>
  </url>
  <url>
    <loc>https://estudodiario.online/?lang=es-ES</loc>
    <xhtml:link rel="alternate" hreflang="pt-BR" href="https://estudodiario.online/?lang=pt-BR"/>
    <xhtml:link rel="alternate" hreflang="en-US" href="https://estudodiario.online/?lang=en-US"/>
    <xhtml:link rel="alternate" hreflang="es-ES" href="https://estudodiario.online/?lang=es-ES"/>
    <xhtml:link rel="alternate" hreflang="x-default" href="https://estudodiario.online/"/>
    <changefreq>daily</changefreq>
    <priority>0.9</priority>
  </url>
</urlset>`
		w.Write([]byte(sitemap))
	})

	// Card visual para redes sociais (Open Graph / Twitter Card)
	mux.HandleFunc("GET /og-image.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=604800")
		svgCard := `<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="630" viewBox="0 0 1200 630">
  <defs>
    <linearGradient id="bg" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#0f172a"/>
      <stop offset="100%" stop-color="#1e293b"/>
    </linearGradient>
    <linearGradient id="accent" x1="0%" y1="0%" x2="100%" y2="0%">
      <stop offset="0%" stop-color="#38bdf8"/>
      <stop offset="100%" stop-color="#818cf8"/>
    </linearGradient>
  </defs>
  <rect width="1200" height="630" fill="url(#bg)"/>
  <rect x="50" y="50" width="1100" height="530" rx="24" fill="none" stroke="#334155" stroke-width="2"/>
  <text x="100" y="190" font-size="70" font-weight="bold" fill="url(#accent)" font-family="system-ui, -apple-system, sans-serif">📖 Estudo Diário</text>
  <text x="100" y="270" font-size="36" font-weight="600" fill="#f8fafc" font-family="system-ui, -apple-system, sans-serif">Aprenda algo novo em 25 minutos por dia</text>
  <text x="100" y="360" font-size="24" fill="#94a3b8" font-family="system-ui, -apple-system, sans-serif">Tema aleatório da Wikipédia • Pomodoro de 25 min • Feedback com IA • Ranking Global</text>
  <rect x="100" y="430" width="300" height="54" rx="12" fill="#38bdf8"/>
  <text x="250" y="465" font-size="20" font-weight="bold" fill="#0f172a" text-anchor="middle" font-family="system-ui, -apple-system, sans-serif">Comece Gratuitamente</text>
  <text x="1050" y="510" font-size="22" fill="#64748b" text-anchor="end" font-family="system-ui, -apple-system, sans-serif">estudodiario.online</text>
</svg>`
		w.Write([]byte(svgCard))
	})

	// Favicon direto da aplicação
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Cache-Control", "public, max-age=604800")
		w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><text y=".9em" font-size="90">📖</text></svg>`))
	})

	// Rate limiter global (20 req/s, burst 40)
	globalLimiter := middleware.RateLimiter(20, 40)

	// Rate limiter severo (2 req/s, burst 5) para endpoints sensíveis
	severeLimiter := middleware.RateLimiter(2, 5)

	mux.HandleFunc("GET /{$}", app.HomeHandler)
	mux.Handle("POST /user", severeLimiter(http.HandlerFunc(app.CreateUserHandler)))
	mux.Handle("POST /login", severeLimiter(http.HandlerFunc(app.LoginHandler)))
	mux.HandleFunc("GET /unsubscribe", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/#unsubscribe", http.StatusSeeOther)
	})
	mux.Handle("POST /unsubscribe", severeLimiter(auth.RequireAuth(app.JWTSecret, app.UnsubscribeHandler)))
	mux.HandleFunc("GET /ranking", app.RankingHandler)
	mux.HandleFunc("GET /study/today", auth.RequireAuth(app.JWTSecret, app.TodayStudyHandler))
	mux.Handle("POST /study/start", severeLimiter(auth.RequireAuth(app.JWTSecret, app.StartStudyHandler)))
	mux.Handle("POST /send-summary", severeLimiter(auth.RequireAuth(app.JWTSecret, app.SendSummaryHandler)))

	log.Println("Servidor Rodando")
	if err := http.ListenAndServe(":8080", i18n.Middleware(globalLimiter(mux))); err != nil {
		log.Fatal(err)
	}
}
