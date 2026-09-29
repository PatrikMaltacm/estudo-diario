package model

import "encoding/json"

// Subject representa a resposta da Wikipedia para um artigo aleatório.
type Subject struct {
	Title string `json:"title"`
}

// UserResponseDTO é o payload de resposta com dados públicos do usuário.
type UserResponseDTO struct {
	Id     string `json:"id"`
	Name   string `json:"name"`
	Points int    `json:"points"`
	Email  string `json:"email"`
}

// UserRequestDTO é o payload de criação/atualização de usuário.
type UserRequestDTO struct {
	Id       string `json:"id"`
	Name     string `json:"name"`
	Points   int    `json:"points"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// StudyStatus descreve o estado do estudo do dia para um usuário.
type StudyStatus struct {
	Status      string          `json:"status"` // not_started | in_progress | expired | submitted
	Topic       string          `json:"topic,omitempty"`
	SecondsLeft int             `json:"seconds_left,omitempty"`
	Evaluation  json.RawMessage `json:"evaluation,omitempty"`
}

// SummaryRequest é o payload do endpoint de envio de resumo.
type SummaryRequest struct {
	Topic   string `json:"topic"`
	Summary string `json:"summary"`
}

// LoginRequest é o payload do endpoint de login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// SummaryEvaluation é a resposta estruturada da avaliação da IA.
type SummaryEvaluation struct {
	Score              int      `json:"score"`
	UnderstandingLevel string   `json:"understanding_level"`
	Feedback           string   `json:"feedback"`
	MissingPoints      []string `json:"missing_points"`
}

// SimilarityResult representa o resultado da análise de similaridade por cópia.
type SimilarityResult struct {
	Level string `json:"level"`
	Score int    `json:"score"`
}
