package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/PatrikMaltacm/estudo-diario/internal/i18n"
	"github.com/PatrikMaltacm/estudo-diario/internal/model"
)

// EvaluateSummary envia o tópico e o resumo do aluno para a OpenAI e
// retorna uma avaliação estruturada no idioma solicitado.
func EvaluateSummary(
	ctx context.Context,
	topic string,
	summary string,
	lang string,
) (*model.SummaryEvaluation, error) {

	aiModel := os.Getenv("OPENAI_MODEL")

	if aiModel == "" {
		aiModel = "gpt-5-nano"
	}

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"score": map[string]any{
				"type":    "integer",
				"minimum": 0,
				"maximum": 10,
			},
			"understanding_level": map[string]any{
				"type": "string",
				"enum": []string{
					"very_low",
					"low",
					"basic",
					"good",
					"very_good",
					"excellent",
				},
			},
			"feedback": map[string]any{
				"type": "string",
			},
			"missing_points": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "string",
				},
			},
		},
		"required": []string{
			"score",
			"understanding_level",
			"feedback",
			"missing_points",
		},
		"additionalProperties": false,
	}

	aiLang := i18n.AILanguageName(lang)
	instructions := fmt.Sprintf(`
			You are an educational evaluator.

			The student studied the given topic for a short period of time
			and then wrote a summary from their understanding.

			Your job is to evaluate how well the student understood the topic.

			SCORING:

			0-2 = very_low
			3-4 = low
			5-6 = basic
			7-8 = good
			9 = very_good
			10 = excellent

			The understanding_level MUST exactly match the score.

			Focus on:

			- factual understanding
			- important concepts
			- ability to explain the topic
			- coherence
			- whether the summary demonstrates actual understanding

			Do not require the student to mention every possible detail.

			The feedback and each item in missing_points must be written in %s.

			The feedback should be concise and useful.

			Mention important concepts the student missed in missing_points.

			Return only the requested JSON.
			`, aiLang)

	var promptInput string
	switch i18n.MatchLocale(lang) {
	case "en-US":
		promptInput = fmt.Sprintf("TOPIC:\n%s\n\nSTUDENT'S SUMMARY:\n%s", topic, summary)
	case "es-ES":
		promptInput = fmt.Sprintf("TEMA:\n%s\n\nRESUMEN DEL ESTUDIANTE:\n%s", topic, summary)
	default:
		promptInput = fmt.Sprintf("TEMA:\n%s\n\nRESUMO DO ALUNO:\n%s", topic, summary)
	}

	payload := map[string]any{
		"model":             aiModel,
		"store":             false,
		"instructions":      instructions,
		"input":             promptInput,
		"max_output_tokens": 2500,
		"text": map[string]any{
			"format": map[string]any{
				"type":   "json_schema",
				"name":   "summary_evaluation",
				"strict": true,
				"schema": schema,
			},
		},
	}

	// 'reasoning.effort' só é suportado por modelos de raciocínio (ex: o1, o3, gpt-5).
	// Modelos como gpt-4o e gpt-4o-mini rejeitam esse parâmetro com erro 400.
	if effort := os.Getenv("OPENAI_REASONING_EFFORT"); effort != "" {
		payload["reasoning"] = map[string]any{
			"effort": effort,
		}
	} else if strings.HasPrefix(aiModel, "o1") || strings.HasPrefix(aiModel, "o3") || strings.HasPrefix(aiModel, "gpt-5") {
		payload["reasoning"] = map[string]any{
			"effort": "low",
		}
	}

	requestBody, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf(
			"erro ao criar requisição: %w",
			err,
		)
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://api.openai.com/v1/responses",
		bytes.NewReader(requestBody),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"erro ao criar request: %w",
			err,
		)
	}

	apiKey := os.Getenv("OPENAI_API_KEY")

	if apiKey == "" {
		return nil, fmt.Errorf(
			"OPENAI_API_KEY não foi definida",
		)
	}

	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	request.Header.Set(
		"Authorization",
		"Bearer "+apiKey,
	)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf(
			"erro ao chamar OpenAI: %w",
			err,
		)
	}

	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		errorBody, _ := io.ReadAll(response.Body)

		return nil, fmt.Errorf(
			"OpenAI retornou status %d: %s",
			response.StatusCode,
			string(errorBody),
		)
	}

	var result struct {
		Status string `json:"status"`

		IncompleteDetails *struct {
			Reason string `json:"reason"`
		} `json:"incomplete_details"`

		Output []struct {
			Type string `json:"type"`

			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}

	if err := json.NewDecoder(
		response.Body,
	).Decode(&result); err != nil {
		return nil, fmt.Errorf(
			"erro ao ler resposta da OpenAI: %w",
			err,
		)
	}

	if result.Status == "incomplete" {
		if result.IncompleteDetails != nil {
			return nil, fmt.Errorf(
				"resposta incompleta: %s",
				result.IncompleteDetails.Reason,
			)
		}

		return nil, fmt.Errorf(
			"resposta incompleta",
		)
	}

	var outputText string

	for _, output := range result.Output {
		if output.Type != "message" {
			continue
		}

		for _, content := range output.Content {
			if content.Type == "output_text" {
				outputText = content.Text
				break
			}
		}

		if outputText != "" {
			break
		}
	}

	if outputText == "" {
		return nil, fmt.Errorf(
			"a OpenAI não retornou conteúdo de texto",
		)
	}

	var evaluation model.SummaryEvaluation

	if err := json.Unmarshal(
		[]byte(outputText),
		&evaluation,
	); err != nil {
		return nil, fmt.Errorf(
			"erro ao interpretar avaliação da OpenAI: %w",
			err,
		)
	}

	return &evaluation, nil
}
