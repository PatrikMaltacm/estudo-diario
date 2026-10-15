package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"net/mail"

	"github.com/PatrikMaltacm/estudo-diario/internal/ai"
	"github.com/PatrikMaltacm/estudo-diario/internal/auth"
	"github.com/PatrikMaltacm/estudo-diario/internal/i18n"
	"github.com/PatrikMaltacm/estudo-diario/internal/model"
	"github.com/PatrikMaltacm/estudo-diario/internal/repository"
	"github.com/PatrikMaltacm/estudo-diario/internal/study"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Application mantém as dependências compartilhadas pelos handlers.
type Application struct {
	DB        *sql.DB
	JWTSecret []byte
	// StaticFS é o sistema de arquivos com os assets estáticos (web/).
	StaticFS fs.FS
}

// writeJSON escreve uma resposta JSON com o status HTTP fornecido.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// isValidEmail valida o formato do e-mail usando net/mail da stdlib.
func isValidEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

// HomeHandler serve o index.html embutido no binário.
func (app *Application) HomeHandler(w http.ResponseWriter, r *http.Request) {
	data, err := fs.ReadFile(app.StaticFS, "index.html")
	if err != nil {
		http.Error(w, i18n.T(r.Context(), "error.not_found"), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

// CreateUserHandler cria um novo usuário no sistema.
func (app *Application) CreateUserHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var user model.UserRequestDTO
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, i18n.T(r.Context(), "error.invalid_data"), http.StatusBadRequest)
		return
	}

	if user.Name == "" {
		http.Error(w, i18n.T(r.Context(), "error.empty_name"), http.StatusBadRequest)
		return
	}

	if user.Email == "" {
		http.Error(w, i18n.T(r.Context(), "error.empty_email"), http.StatusBadRequest)
		return
	}

	if !isValidEmail(user.Email) {
		http.Error(w, i18n.T(r.Context(), "error.invalid_email"), http.StatusBadRequest)
		return
	}

	// o bcrypt só considera os primeiros 72 bytes da senha
	if len(user.Password) < 8 || len(user.Password) > 72 {
		http.Error(w, i18n.T(r.Context(), "error.password_length"), http.StatusBadRequest)
		return
	}

	exists, err := repository.UserExistsByEmail(user.Email, app.DB)
	if err != nil {
		log.Printf("Erro ao verificar e-mail: %v", err)
		http.Error(w, i18n.T(r.Context(), "error.internal"), http.StatusInternalServerError)
		return
	}
	if exists {
		http.Error(w, i18n.T(r.Context(), "error.email_in_use"), http.StatusConflict)
		return
	}

	user.Id = uuid.NewString()
	user.Points = 0

	if err := repository.InsertNewUser(&user, app.DB); err != nil {
		log.Printf("Erro ao salvar usuário: %v", err)
		http.Error(w, i18n.T(r.Context(), "error.save_user"), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"message": i18n.T(r.Context(), "success.user_created"),
	})
}

// LoginHandler autentica o usuário e devolve um Bearer token JWT.
func (app *Application) LoginHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req model.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, i18n.T(r.Context(), "error.invalid_data"), http.StatusBadRequest)
		return
	}

	if req.Email == "" || req.Password == "" {
		http.Error(w, i18n.T(r.Context(), "error.email_password_required"), http.StatusBadRequest)
		return
	}

	userID, hash, err := repository.GetCredentialsByEmail(req.Email, app.DB)
	if errors.Is(err, sql.ErrNoRows) {
		bcrypt.CompareHashAndPassword(auth.DummyHash, []byte(req.Password))
		http.Error(w, i18n.T(r.Context(), "error.invalid_credentials"), http.StatusUnauthorized)
		return
	}
	if err != nil {
		http.Error(w, i18n.T(r.Context(), "error.internal"), http.StatusInternalServerError)
		return
	}

	if err := bcrypt.CompareHashAndPassword(hash, []byte(req.Password)); err != nil {
		http.Error(w, i18n.T(r.Context(), "error.invalid_credentials"), http.StatusUnauthorized)
		return
	}

	token, err := auth.GenerateToken(app.JWTSecret, userID)
	if err != nil {
		http.Error(w, i18n.T(r.Context(), "error.token_generate"), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]any{
		"token":      token,
		"token_type": "Bearer",
		"expires_in": int(auth.TokenTTL.Seconds()),
	})
}

// TodayStudyHandler retorna o estado do estudo do dia para o usuário autenticado.
func (app *Application) TodayStudyHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, i18n.T(r.Context(), "error.unauthorized"), http.StatusUnauthorized)
		return
	}

	st, err := study.GetStatus(r.Context(), app.DB, userID)
	if err != nil {
		log.Printf("Erro ao buscar estudo de hoje: %v", err)
		http.Error(w, i18n.T(r.Context(), "error.internal"), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// StartStudyHandler inicia o timer de 25 min. É idempotente: chamar de novo
// (F5, outra aba) devolve a sessão existente, sem reiniciar o relógio.
func (app *Application) StartStudyHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, i18n.T(r.Context(), "error.unauthorized"), http.StatusUnauthorized)
		return
	}

	st, err := study.GetStatus(r.Context(), app.DB, userID)
	if err != nil {
		log.Printf("Erro ao buscar estudo de hoje: %v", err)
		http.Error(w, i18n.T(r.Context(), "error.internal"), http.StatusInternalServerError)
		return
	}

	if st.Status == "not_started" {
		lang := i18n.FromContext(r.Context())
		topic, err := study.FetchRandomTopic(r.Context(), lang)
		if err != nil {
			log.Printf("Erro ao sortear assunto: %v", err)
			http.Error(w, i18n.T(r.Context(), "error.topic_fetch"), http.StatusBadGateway)
			return
		}

		_, err = app.DB.ExecContext(r.Context(),
			`INSERT INTO study_sessions (user_id, study_date, topic, language)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (user_id, study_date) DO NOTHING`,
			userID, study.Today(), topic, lang,
		)
		if err != nil {
			log.Printf("Erro ao criar sessão: %v", err)
			http.Error(w, i18n.T(r.Context(), "error.internal"), http.StatusInternalServerError)
			return
		}

		// relê do banco: se duas abas iniciarem juntas, ambas veem a mesma sessão
		st, err = study.GetStatus(r.Context(), app.DB, userID)
		if err != nil {
			http.Error(w, i18n.T(r.Context(), "error.internal"), http.StatusInternalServerError)
			return
		}
	}

	writeJSON(w, http.StatusOK, st)
}

// SendSummaryHandler recebe o resumo do aluno, avalia com IA e grava o resultado.
func (app *Application) SendSummaryHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, i18n.T(r.Context(), "error.unauthorized"), http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req struct {
		Summary string `json:"summary"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, i18n.T(r.Context(), "error.invalid_json"), http.StatusBadRequest)
		return
	}

	if n := len([]rune(req.Summary)); n < 100 || n > 2000 {
		http.Error(w, i18n.T(r.Context(), "error.summary_length"), http.StatusBadRequest)
		return
	}

	// 1. reserva o envio (valida prazo e duplicidade no próprio UPDATE)
	sessionID, topic, sessionLang, err := study.ClaimSubmission(r.Context(), app.DB, userID, req.Summary)
	if errors.Is(err, sql.ErrNoRows) {
		msg, code := i18n.T(r.Context(), "error.create_session"), http.StatusConflict

		if st, stErr := study.GetStatus(r.Context(), app.DB, userID); stErr == nil {
			switch st.Status {
			case "not_started":
				msg = i18n.T(r.Context(), "error.start_study_first")
			case "submitted":
				msg = i18n.T(r.Context(), "error.already_submitted")
			case "expired":
				msg, code = i18n.T(r.Context(), "error.time_expired"), http.StatusForbidden
			}
		}
		http.Error(w, msg, code)
		return
	}
	if err != nil {
		log.Printf("Erro ao reservar envio: %v", err)
		http.Error(w, i18n.T(r.Context(), "error.internal"), http.StatusInternalServerError)
		return
	}

	// contexto que sobrevive se o cliente fechar a aba no meio da avaliação
	ctx := context.WithoutCancel(r.Context())

	// 2. avalia com a IA no idioma registrado na sessão
	evaluation, err := ai.EvaluateSummary(r.Context(), topic, req.Summary, sessionLang)
	if err != nil {
		log.Printf("Erro ao avaliar resumo: %v", err)
		study.ReleaseSubmission(ctx, app.DB, sessionID)
		http.Error(w, i18n.T(r.Context(), "error.evaluate_summary"), http.StatusInternalServerError)
		return
	}

	// 3. grava avaliação + pontos
	if err := study.SaveEvaluation(ctx, app.DB, sessionID, userID, evaluation); err != nil {
		log.Printf("Erro ao salvar avaliação: %v", err)
		study.ReleaseSubmission(ctx, app.DB, sessionID)
		http.Error(w, i18n.T(r.Context(), "error.save_evaluation"), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, evaluation)
}

// RankingEntry representa uma entrada no ranking público.
type RankingEntry struct {
	Position int    `json:"position"`
	Name     string `json:"name"`
	Points   int    `json:"points"`
}

// RankingHandler retorna os top 50 usuários ordenados por pontos (endpoint público).
func (app *Application) RankingHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := app.DB.QueryContext(r.Context(),
		`SELECT name, points FROM users ORDER BY points DESC LIMIT 50`,
	)
	if err != nil {
		log.Printf("Erro ao buscar ranking: %v", err)
		http.Error(w, i18n.T(r.Context(), "error.internal"), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var entries []RankingEntry
	pos := 1
	for rows.Next() {
		var e RankingEntry
		if err := rows.Scan(&e.Name, &e.Points); err != nil {
			log.Printf("Erro ao ler ranking: %v", err)
			continue
		}
		e.Position = pos
		entries = append(entries, e)
		pos++
	}

	if err := rows.Err(); err != nil {
		log.Printf("Erro ao iterar no ranking: %v", err)
		http.Error(w, "Erro interno", http.StatusInternalServerError)
		return
	}

	if entries == nil {
		entries = []RankingEntry{}
	}
	writeJSON(w, http.StatusOK, entries)
}
