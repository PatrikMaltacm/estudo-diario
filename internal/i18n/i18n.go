package i18n

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"golang.org/x/text/language"
)

//go:embed locales/*.json
var localesFS embed.FS

const DefaultLocale = "pt-BR"

var (
	// Suporte aos idiomas principais: pt-BR, en-US e es-ES.
	SupportedTags = []language.Tag{
		language.BrazilianPortuguese, // pt-BR (padrão)
		language.AmericanEnglish,     // en-US
		language.EuropeanSpanish,     // es-ES
	}

	matcher = language.NewMatcher(SupportedTags)

	// translations armazena o mapa [locale][key]message em memória.
	translations   = make(map[string]map[string]string)
	translationsMu sync.RWMutex
)

type contextKey string

const localeContextKey = contextKey("i18n_locale")

func init() {
	if err := loadTranslations(); err != nil {
		log.Fatalf("Erro ao inicializar traduções i18n: %v", err)
	}
}

func loadTranslations() error {
	translationsMu.Lock()
	defer translationsMu.Unlock()

	files := map[string]string{
		"pt-BR": "locales/pt-BR.json",
		"en-US": "locales/en-US.json",
		"es-ES": "locales/es-ES.json",
	}

	for locale, filename := range files {
		data, err := localesFS.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("falha ao ler arquivo %s: %w", filename, err)
		}

		var dict map[string]string
		if err := json.Unmarshal(data, &dict); err != nil {
			return fmt.Errorf("falha ao decodificar JSON %s: %w", filename, err)
		}

		translations[locale] = dict
	}
	return nil
}

// MatchLocale resolve uma string de idioma (ou cabeçalho Accept-Language) para o locale canônico suportado.
func MatchLocale(acceptLanguage string) string {
	acceptLanguage = strings.TrimSpace(acceptLanguage)
	if acceptLanguage == "" {
		return DefaultLocale
	}

	tags, _, err := language.ParseAcceptLanguage(acceptLanguage)
	if err != nil || len(tags) == 0 {
		tag, err := language.Parse(acceptLanguage)
		if err != nil {
			return DefaultLocale
		}
		tags = []language.Tag{tag}
	}

	tag, _, _ := matcher.Match(tags...)
	switch tag {
	case language.AmericanEnglish, language.English:
		return "en-US"
	case language.EuropeanSpanish, language.Spanish:
		return "es-ES"
	case language.BrazilianPortuguese, language.Portuguese:
		return "pt-BR"
	default:
		// Se for outro dialeto ou base
		base, _ := tag.Base()
		switch base.String() {
		case "en":
			return "en-US"
		case "es":
			return "es-ES"
		case "pt":
			return "pt-BR"
		default:
			return DefaultLocale
		}
	}
}

// WikiLanguage retorna o prefixo de subdomínio da Wikipédia (ex: pt, en, es).
func WikiLanguage(locale string) string {
	canonical := MatchLocale(locale)
	switch canonical {
	case "en-US":
		return "en"
	case "es-ES":
		return "es"
	default:
		return "pt"
	}
}

// AILanguageName retorna o nome do idioma em inglês para inclusão nas instruções do prompt da OpenAI.
func AILanguageName(locale string) string {
	canonical := MatchLocale(locale)
	switch canonical {
	case "en-US":
		return "English"
	case "es-ES":
		return "Spanish"
	default:
		return "Brazilian Portuguese"
	}
}

// WithLocale associa um locale a um context.Context.
func WithLocale(ctx context.Context, locale string) context.Context {
	return context.WithValue(ctx, localeContextKey, MatchLocale(locale))
}

// FromContext extrai o locale do context.Context, retornando o DefaultLocale se não presente.
func FromContext(ctx context.Context) string {
	if ctx == nil {
		return DefaultLocale
	}
	if v, ok := ctx.Value(localeContextKey).(string); ok && v != "" {
		return v
	}
	return DefaultLocale
}

// Translate traduz uma chave para o idioma informado com fallback para pt-BR e formatação opcional.
func Translate(locale, key string, args ...any) string {
	canonical := MatchLocale(locale)

	translationsMu.RLock()
	defer translationsMu.RUnlock()

	dict, ok := translations[canonical]
	if ok {
		if msg, exists := dict[key]; exists {
			if len(args) > 0 {
				return fmt.Sprintf(msg, args...)
			}
			return msg
		}
	}

	// Fallback para pt-BR
	if canonical != DefaultLocale {
		if defDict, defOk := translations[DefaultLocale]; defOk {
			if msg, exists := defDict[key]; exists {
				if len(args) > 0 {
					return fmt.Sprintf(msg, args...)
				}
				return msg
			}
		}
	}

	if len(args) > 0 {
		return fmt.Sprintf(key, args...)
	}
	return key
}

// T é um helper para tradução extraindo o idioma diretamente do contexto da requisição.
func T(ctx context.Context, key string, args ...any) string {
	return Translate(FromContext(ctx), key, args...)
}

// Middleware detecta o idioma a partir de ?lang= ou do cabeçalho Accept-Language.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rawLang string

		if q := r.URL.Query().Get("lang"); q != "" {
			rawLang = q
		} else if q := r.URL.Query().Get("locale"); q != "" {
			rawLang = q
		} else {
			rawLang = r.Header.Get("Accept-Language")
		}

		locale := MatchLocale(rawLang)
		ctx := WithLocale(r.Context(), locale)

		w.Header().Set("Content-Language", locale)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
