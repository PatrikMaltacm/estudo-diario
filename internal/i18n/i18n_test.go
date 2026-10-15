package i18n

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMatchLocale(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "pt-BR"},
		{"pt-BR", "pt-BR"},
		{"pt", "pt-BR"},
		{"pt-PT", "pt-BR"},
		{"en-US", "en-US"},
		{"en", "en-US"},
		{"en-GB", "en-US"},
		{"es-ES", "es-ES"},
		{"es", "es-ES"},
		{"es-MX", "es-ES"},
		{"fr-FR,fr;q=0.9,en;q=0.8", "en-US"},
		{"de-DE", "pt-BR"}, // fallback
	}

	for _, tt := range tests {
		got := MatchLocale(tt.input)
		if got != tt.expected {
			t.Errorf("MatchLocale(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}

func TestTranslate(t *testing.T) {
	ptMsg := Translate("pt-BR", "error.invalid_credentials")
	if ptMsg != "Credenciais inválidas" {
		t.Errorf("Esperava 'Credenciais inválidas', obteve %q", ptMsg)
	}

	enMsg := Translate("en-US", "error.invalid_credentials")
	if enMsg != "Invalid credentials" {
		t.Errorf("Esperava 'Invalid credentials', obteve %q", enMsg)
	}

	esMsg := Translate("es-ES", "error.invalid_credentials")
	if esMsg != "Credenciales inválidas" {
		t.Errorf("Esperava 'Credenciales inválidas', obteve %q", esMsg)
	}

	// Fallback de chave inexistente em es-ES mas existente em pt-BR
	fallbackMsg := Translate("de-DE", "error.not_found")
	if fallbackMsg != "Página não encontrada" {
		t.Errorf("Esperava 'Página não encontrada', obteve %q", fallbackMsg)
	}
}

func TestContextAndMiddleware(t *testing.T) {
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locale := FromContext(r.Context())
		msg := T(r.Context(), "error.empty_email")
		w.Header().Set("X-Test-Locale", locale)
		w.Write([]byte(msg))
	}))

	req := httptest.NewRequest("GET", "/test?lang=en", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Test-Locale") != "en-US" {
		t.Errorf("Esperava locale en-US, obteve %s", rec.Header().Get("X-Test-Locale"))
	}
	if rec.Body.String() != "Email cannot be empty" {
		t.Errorf("Esperava 'Email cannot be empty', obteve %q", rec.Body.String())
	}
}

func TestHelpers(t *testing.T) {
	if WikiLanguage("en-US") != "en" {
		t.Errorf("WikiLanguage(en-US) falhou: %s", WikiLanguage("en-US"))
	}
	if WikiLanguage("es-ES") != "es" {
		t.Errorf("WikiLanguage(es-ES) falhou: %s", WikiLanguage("es-ES"))
	}
	if WikiLanguage("pt-BR") != "pt" {
		t.Errorf("WikiLanguage(pt-BR) falhou: %s", WikiLanguage("pt-BR"))
	}

	if AILanguageName("en-US") != "English" {
		t.Errorf("AILanguageName(en-US) falhou: %s", AILanguageName("en-US"))
	}
	if AILanguageName("es-ES") != "Spanish" {
		t.Errorf("AILanguageName(es-ES) falhou: %s", AILanguageName("es-ES"))
	}
	if AILanguageName("pt-BR") != "Brazilian Portuguese" {
		t.Errorf("AILanguageName(pt-BR) falhou: %s", AILanguageName("pt-BR"))
	}
}
