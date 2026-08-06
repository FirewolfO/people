package service

import (
	"errors"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"people/internal/model"
	"people/internal/store"
)

const (
	testClientID     = "permission-ui"
	testClientSecret = "permission-secret"
	testRedirectURI  = "http://localhost:5173/oauth/callback"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	database, err := store.Open(filepath.Join(t.TempDir(), "people.db"), testClientID, testClientSecret, []string{testRedirectURI})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return New(database, time.Hour)
}

func queryValue(t *testing.T, rawURL, key string) string {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	value := parsed.Query().Get(key)
	if value == "" {
		t.Fatalf("%s missing from %s", key, rawURL)
	}
	return value
}

func TestNewEmployeeMustSetPasswordBeforeOAuth(t *testing.T) {
	svc := newTestService(t)
	created, err := svc.CreateEmployee(EmployeeInput{
		EmployeeNo: "E001", Username: "alice", DisplayName: "Alice", Role: model.RoleEmployee, Status: model.StatusEnabled,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created.MustChangePassword || created.PasswordHash != "" {
		t.Fatalf("new employee = %#v, want password unset and change required", created)
	}

	employee, sessionToken, err := svc.Login("alice", "any value is ignored before password setup")
	if err != nil {
		t.Fatalf("first Login() error = %v", err)
	}
	if _, err := svc.Authorize(employee, testClientID, testRedirectURI, "state"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Authorize() before password setup error = %v, want forbidden", err)
	}
	_, session, err := svc.AuthenticateSession(sessionToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ChangePassword(employee, session.ID, "", "new-password-123"); err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}
	if _, _, err := svc.Login("alice", "wrong-password"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("Login() with wrong configured password error = %v, want unauthorized", err)
	}
	configured, _, err := svc.Login("alice", "new-password-123")
	if err != nil {
		t.Fatalf("Login() with configured password error = %v", err)
	}
	if configured.MustChangePassword {
		t.Fatal("MustChangePassword remained true after password setup")
	}
}

func TestOAuthRequiresClientSecretAndConsumesCode(t *testing.T) {
	svc := newTestService(t)
	employee, _, err := svc.Login("admin", "admin")
	if err != nil {
		t.Fatal(err)
	}
	redirect, err := svc.Authorize(employee, testClientID, testRedirectURI, "expected-state")
	if err != nil {
		t.Fatal(err)
	}
	code := queryValue(t, redirect, "code")
	if _, err := svc.ExchangeCode(testClientID, "", code, testRedirectURI); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("ExchangeCode() without secret error = %v, want unauthorized", err)
	}
	result, err := svc.ExchangeCode(testClientID, testClientSecret, code, testRedirectURI)
	if err != nil {
		t.Fatal(err)
	}
	if result.AccessToken == "" || result.User == nil || result.User.Username != "admin" {
		t.Fatalf("token result = %#v", result)
	}
	if _, err := svc.ExchangeCode(testClientID, testClientSecret, code, testRedirectURI); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("reused code error = %v, want unauthorized", err)
	}
	if _, err := svc.ClientCredentials(testClientID, "", "employees.read"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("ClientCredentials() without secret error = %v, want unauthorized", err)
	}
}
