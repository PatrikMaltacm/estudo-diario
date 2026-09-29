package study

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"
	_ "time/tzdata" // embute os fusos horários no binário (não depende do SO)

	"github.com/PatrikMaltacm/estudo-diario/internal/model"
)

const (
	StudyDuration = 25 * time.Minute
	SubmitGrace   = 30 * time.Second // tolerância para latência de rede
)

var BrLoc = mustLoadLocation("America/Sao_Paulo")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		log.Fatalf("fuso horário inválido: %v", err)
	}
	return loc
}

// Today devolve a data de hoje no fuso de Brasília (AAAA-MM-DD).
func Today() string {
	return time.Now().In(BrLoc).Format("2006-01-02")
}

// FetchRandomTopic busca um artigo aleatório na Wikipédia em português.
func FetchRandomTopic(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://pt.wikipedia.org/api/rest_v1/page/random/summary", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "EstudoDiario/1.0 (EstudoDiario.com.br)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("wikipédia retornou status %d", resp.StatusCode)
	}

	var s model.Subject
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return "", err
	}
	if s.Title == "" {
		return "", errors.New("wikipédia retornou assunto vazio")
	}
	return s.Title, nil
}

// GetStatus consulta o banco e retorna o estado atual do estudo do usuário.
func GetStatus(ctx context.Context, db *sql.DB, userID string) (*model.StudyStatus, error) {
	var (
		topic       string
		startedAt   time.Time
		submittedAt sql.NullTime
		evaluation  []byte
	)

	err := db.QueryRowContext(ctx,
		`SELECT topic, started_at, submitted_at, evaluation
		   FROM study_sessions
		  WHERE user_id = $1 AND study_date = $2`,
		userID, Today(),
	).Scan(&topic, &startedAt, &submittedAt, &evaluation)

	if errors.Is(err, sql.ErrNoRows) {
		return &model.StudyStatus{Status: "not_started"}, nil
	}
	if err != nil {
		return nil, err
	}

	st := &model.StudyStatus{Topic: topic}
	left := time.Until(startedAt.Add(StudyDuration))

	switch {
	case submittedAt.Valid:
		st.Status = "submitted"
		st.Evaluation = evaluation
	case left > 0:
		st.Status = "in_progress"
		st.SecondsLeft = int(left.Seconds())
	default:
		st.Status = "expired"
	}
	return st, nil
}

// ClaimSubmission "reserva" o envio de forma atômica: só funciona se a sessão
// mais recente ainda não foi enviada e está dentro dos 25 min.
// Isso impede envio duplo (inclusive requisições simultâneas) e envio fora do prazo.
func ClaimSubmission(ctx context.Context, db *sql.DB, userID, summary string) (int64, string, error) {
	var id int64
	var topic string

	err := db.QueryRowContext(ctx, `
		UPDATE study_sessions
		   SET submitted_at = now(), summary = $2
		 WHERE id = (SELECT id FROM study_sessions
		              WHERE user_id = $1
		              ORDER BY started_at DESC LIMIT 1)
		   AND submitted_at IS NULL
		   AND now() <= started_at + make_interval(secs => $3)
		RETURNING id, topic`,
		userID, summary, (StudyDuration+SubmitGrace).Seconds(),
	).Scan(&id, &topic)

	return id, topic, err
}

// ReleaseSubmission devolve a sessão ao estado "não enviada" se algo falhou
// do nosso lado (OpenAI fora do ar, erro no banco...), para o usuário não
// perder o dia por um erro que não foi dele.
func ReleaseSubmission(ctx context.Context, db *sql.DB, sessionID int64) {
	_, err := db.ExecContext(ctx,
		`UPDATE study_sessions SET submitted_at = NULL, summary = NULL WHERE id = $1`,
		sessionID,
	)
	if err != nil {
		log.Printf("Erro ao liberar sessão %d: %v", sessionID, err)
	}
}

// SaveEvaluation grava a avaliação e soma os pontos na mesma transação.
func SaveEvaluation(ctx context.Context, db *sql.DB, sessionID int64, userID string, ev *model.SummaryEvaluation) error {
	raw, err := json.Marshal(ev)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`UPDATE study_sessions SET evaluation = $2 WHERE id = $1`,
		sessionID, raw,
	); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE users SET points = points + $2 WHERE id = $1`,
		userID, ev.Score,
	); err != nil {
		return err
	}

	return tx.Commit()
}
