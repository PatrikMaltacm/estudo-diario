package handler

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/PatrikMaltacm/estudo-diario/internal/i18n"
)

func TestHomeHandlerWithLanguage(t *testing.T) {
	fakeFS := fstest.MapFS{
		"index.html": &fstest.MapFile{
			Data: []byte("<!DOCTYPE html><html><body><h1>Estudo Diário</h1></body></html>"),
		},
	}

	app := &Application{
		StaticFS: fs.FS(fakeFS),
	}

	handler := i18n.Middleware(http.HandlerFunc(app.HomeHandler))

	tests := []struct {
		acceptLang     string
		expectedLangHdr string
	}{
		{"pt-BR", "pt-BR"},
		{"en-US,en;q=0.9", "en-US"},
		{"es-ES", "es-ES"},
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept-Language", tt.acceptLang)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Esperava 200 OK, obteve %d", rec.Code)
		}

		contentLang := rec.Header().Get("Content-Language")
		if contentLang != tt.expectedLangHdr {
			t.Errorf("Para Accept-Language %q, esperava Content-Language %q, obteve %q", tt.acceptLang, tt.expectedLangHdr, contentLang)
		}
	}
}

func TestValidationTranslations(t *testing.T) {
	app := &Application{}

	handler := i18n.Middleware(http.HandlerFunc(app.CreateUserHandler))

	// Requisição com body vazio e Accept-Language em inglês
	req := httptest.NewRequest("POST", "/user", nil)
	req.Header.Set("Accept-Language", "en-US")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Esperava status 400 Bad Request, obteve %d", rec.Code)
	}

	body := rec.Body.String()
	if body != "Invalid data\n" {
		t.Errorf("Esperava 'Invalid data\\n', obteve %q", body)
	}
}
