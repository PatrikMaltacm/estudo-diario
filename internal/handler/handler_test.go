package handler

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/PatrikMaltacm/estudo-diario/internal/auth"
	"github.com/PatrikMaltacm/estudo-diario/internal/i18n"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
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

type recordedExec struct {
	query string
	args  []any
}

type fakeDeleteStore struct {
	hashesByID map[string][]byte
	execs      []recordedExec
	committed  bool
	rolledBack bool
}

type fakeDriver struct {
	store *fakeDeleteStore
}

func (d *fakeDriver) Open(string) (driver.Conn, error) {
	return &fakeConn{store: d.store}, nil
}

type fakeConn struct {
	store *fakeDeleteStore
}

func (c *fakeConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not implemented") }
func (c *fakeConn) Close() error                       { return nil }
func (c *fakeConn) Begin() (driver.Tx, error)          { return c.BeginTx(context.Background(), driver.TxOptions{}) }

func (c *fakeConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return &fakeTx{store: c.store}, nil
}

func (c *fakeConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if !strings.Contains(query, "SELECT password FROM users WHERE id = $1") || len(args) != 1 {
		return nil, errors.New("unexpected query")
	}
	userID, _ := args[0].Value.(string)
	hash, ok := c.store.hashesByID[userID]
	if !ok {
		return &fakeRows{done: true}, nil
	}
	return &fakeRows{hash: hash}, nil
}

func (c *fakeConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	plainArgs := make([]any, len(args))
	for i, a := range args {
		plainArgs[i] = a.Value
	}
	c.store.execs = append(c.store.execs, recordedExec{query: query, args: plainArgs})

	if strings.Contains(query, "DELETE FROM users WHERE id = $1") {
		userID, _ := plainArgs[0].(string)
		if _, ok := c.store.hashesByID[userID]; !ok {
			return driver.RowsAffected(0), nil
		}
		delete(c.store.hashesByID, userID)
	}
	return driver.RowsAffected(1), nil
}

type fakeTx struct {
	store *fakeDeleteStore
}

func (t *fakeTx) Commit() error {
	t.store.committed = true
	return nil
}

func (t *fakeTx) Rollback() error {
	t.store.rolledBack = true
	return nil
}

type fakeRows struct {
	hash []byte
	done bool
}

func (r *fakeRows) Columns() []string { return []string{"password"} }
func (r *fakeRows) Close() error      { return nil }
func (r *fakeRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	dest[0] = r.hash
	return nil
}

type fakeConnector struct {
	driver driver.Driver
}

func (c *fakeConnector) Connect(context.Context) (driver.Conn, error) { return c.driver.Open("") }
func (c *fakeConnector) Driver() driver.Driver                        { return c.driver }

func openFakeDeleteDB(store *fakeDeleteStore) *sql.DB {
	return sql.OpenDB(&fakeConnector{driver: &fakeDriver{store: store}})
}

func TestDeleteUserHandlerSecurity(t *testing.T) {
	secret := []byte("super-secret-jwt-key-at-least-32-chars!!")
	ownerHash, err := bcrypt.GenerateFromPassword([]byte("MinhaSenha123"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("erro ao gerar hash: %v", err)
	}
	victimHash, err := bcrypt.GenerateFromPassword([]byte("SenhaDaVitima"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("erro ao gerar hash: %v", err)
	}

	newHandler := func(store *fakeDeleteStore) (*sql.DB, http.Handler) {
		db := openFakeDeleteDB(store)
		app := &Application{
			DB:        db,
			JWTSecret: secret,
		}
		return db, i18n.Middleware(auth.RequireAuth(app.JWTSecret, app.DeleteUserHandler))
	}

	t.Run("rejects missing token", func(t *testing.T) {
		store := &fakeDeleteStore{hashesByID: map[string][]byte{"owner-1": ownerHash}}
		db, h := newHandler(store)
		defer db.Close()

		req := httptest.NewRequest(http.MethodPost, "/user/delete", strings.NewReader(`{"password":"MinhaSenha123"}`))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("esperava 401, obteve %d", rec.Code)
		}
		if len(store.execs) != 0 {
			t.Fatalf("nenhuma query DELETE deveria rodar sem token")
		}
	})

	t.Run("rejects forged token signed with wrong secret or alg none", func(t *testing.T) {
		store := &fakeDeleteStore{hashesByID: map[string][]byte{"owner-1": ownerHash}}
		db, h := newHandler(store)
		defer db.Close()

		forgedToken, _ := auth.GenerateToken([]byte("wrong-secret-from-github-issue-12345"), "owner-1")
		noneToken, _ := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.RegisteredClaims{Subject: "owner-1"}).SignedString(jwt.UnsafeAllowNoneSignatureType)

		for _, tok := range []string{forgedToken, noneToken} {
			req := httptest.NewRequest(http.MethodPost, "/user/delete", strings.NewReader(`{"password":"MinhaSenha123"}`))
			req.Header.Set("Authorization", "Bearer "+tok)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("esperava 401 para token forjado, obteve %d", rec.Code)
			}
		}
	})

	t.Run("rejects wrong password with 403 and keeps account", func(t *testing.T) {
		store := &fakeDeleteStore{hashesByID: map[string][]byte{"owner-1": ownerHash}}
		db, h := newHandler(store)
		defer db.Close()

		tok, _ := auth.GenerateToken(secret, "owner-1")
		req := httptest.NewRequest(http.MethodPost, "/user/delete", strings.NewReader(`{"password":"SenhaErrada"}`))
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Accept-Language", "en-US")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("esperava 403, obteve %d (%s)", rec.Code, rec.Body.String())
		}
		if strings.TrimSpace(rec.Body.String()) != "Incorrect password" {
			t.Fatalf("esperava 'Incorrect password', obteve %q", rec.Body.String())
		}
		if len(store.execs) != 0 {
			t.Fatalf("conta não deveria ser removida com senha incorreta")
		}
	})

	t.Run("ignores spoofed user_id in body and deletes only JWT subject", func(t *testing.T) {
		store := &fakeDeleteStore{
			hashesByID: map[string][]byte{
				"owner-1":  ownerHash,
				"victim-2": victimHash,
			},
		}
		db, h := newHandler(store)
		defer db.Close()

		tok, _ := auth.GenerateToken(secret, "owner-1")
		req := httptest.NewRequest(
			http.MethodPost,
			"/user/delete",
			strings.NewReader(`{"id":"victim-2","user_id":"victim-2","password":"MinhaSenha123"}`),
		)
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("esperava 200, obteve %d (%s)", rec.Code, rec.Body.String())
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("esperava Cache-Control: no-store, obteve %q", rec.Header().Get("Cache-Control"))
		}
		if !store.committed {
			t.Fatalf("transação de exclusão deveria ter feito commit")
		}
		if _, exists := store.hashesByID["owner-1"]; exists {
			t.Fatalf("conta do dono do token deveria ter sido removida")
		}
		if _, exists := store.hashesByID["victim-2"]; !exists {
			t.Fatalf("conta de outro usuário jamais pode ser afetada")
		}
		if len(store.execs) != 2 || store.execs[0].args[0] != "owner-1" || store.execs[1].args[0] != "owner-1" {
			t.Fatalf("esperava DELETE em study_sessions e users apenas para owner-1, obteve %+v", store.execs)
		}
	})
}
