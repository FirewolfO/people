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
	department, err := svc.CreateDepartment(DepartmentInput{Code: "engineering", Name: "研发部", Status: model.StatusEnabled})
	if err != nil {
		t.Fatal(err)
	}
	created, err := svc.CreateEmployee(EmployeeInput{
		EmployeeNo: "E001", Username: "alice", DisplayName: "Alice", DepartmentID: department.ID, Role: model.RoleEmployee, Status: model.StatusEnabled,
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

func TestEmployeeRequiresManagedDepartment(t *testing.T) {
	svc := newTestService(t)
	input := EmployeeInput{EmployeeNo: "E002", Username: "bob", DisplayName: "Bob", Role: model.RoleEmployee, Status: model.StatusEnabled}
	if _, err := svc.CreateEmployee(input); !errors.Is(err, ErrInvalid) {
		t.Fatalf("CreateEmployee() without department error = %v, want invalid", err)
	}
	input.DepartmentID = "dep_missing"
	if _, err := svc.CreateEmployee(input); !errors.Is(err, ErrInvalid) {
		t.Fatalf("CreateEmployee() with unknown department error = %v, want invalid", err)
	}
	parent, err := svc.CreateDepartment(DepartmentInput{Code: "business", Name: "业务部", Status: model.StatusEnabled})
	if err != nil {
		t.Fatal(err)
	}
	department, err := svc.CreateDepartment(DepartmentInput{ParentID: parent.ID, Code: "sales", Name: "销售部", Status: model.StatusEnabled})
	if err != nil {
		t.Fatal(err)
	}
	input.DepartmentID = department.ID
	created, err := svc.CreateEmployee(input)
	if err != nil {
		t.Fatal(err)
	}
	if created.DepartmentID != department.ID || created.Department != department.Name {
		t.Fatalf("created employee department = %#v", created)
	}
	if err := svc.DeleteDepartment(department.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("DeleteDepartment() referenced error = %v, want conflict", err)
	}
	if err := svc.DeleteDepartment(parent.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("DeleteDepartment() with child error = %v, want conflict", err)
	}
	if _, err := svc.UpdateDepartment(parent.ID, DepartmentInput{ParentID: department.ID, Code: parent.Code, Name: parent.Name, Status: model.StatusEnabled}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("UpdateDepartment() cycle error = %v, want invalid", err)
	}
	updated, err := svc.UpdateDepartment(department.ID, DepartmentInput{Code: "sales", Name: "全球销售部", Status: model.StatusEnabled})
	if err != nil {
		t.Fatal(err)
	}
	if updated.EmployeeCount != 1 {
		t.Fatalf("updated department employee count = %d, want 1", updated.EmployeeCount)
	}
	employees, err := svc.ListEmployees("", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(employees.Items) != 2 || employees.Items[0].Department != "全球销售部" {
		t.Fatalf("employee department was not synchronized: %#v", employees.Items)
	}
}

func TestAdminMayHaveNoDepartment(t *testing.T) {
	svc := newTestService(t)
	created, err := svc.CreateEmployee(EmployeeInput{
		EmployeeNo: "A002", Username: "operator", DisplayName: "Operator", Role: model.RoleAdmin, Status: model.StatusEnabled,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.DepartmentID != "" || created.Department != "" {
		t.Fatalf("admin department = %#v", created)
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

func TestOAuthAccountSwitchDoesNotReplacePeopleSession(t *testing.T) {
	svc := newTestService(t)
	department, err := svc.CreateDepartment(DepartmentInput{Code: "engineering", Name: "研发部", Status: model.StatusEnabled})
	if err != nil {
		t.Fatal(err)
	}
	alice, err := svc.CreateEmployee(EmployeeInput{
		EmployeeNo: "E003", Username: "alice", DisplayName: "Alice", DepartmentID: department.ID,
		Role: model.RoleEmployee, Status: model.StatusEnabled,
	})
	if err != nil {
		t.Fatal(err)
	}
	alice, setupToken, err := svc.Login("alice", "initial setup")
	if err != nil {
		t.Fatal(err)
	}
	_, setupSession, err := svc.AuthenticateSession(setupToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ChangePassword(alice, setupSession.ID, "", "alice-password-123"); err != nil {
		t.Fatal(err)
	}

	_, adminToken, err := svc.Login("admin", "admin")
	if err != nil {
		t.Fatal(err)
	}
	var sessionsBefore int64
	if err := svc.store.DB.Model(&model.Session{}).Count(&sessionsBefore).Error; err != nil {
		t.Fatal(err)
	}
	selected, err := svc.AuthenticateOAuthAccount("alice", "alice-password-123")
	if err != nil {
		t.Fatalf("AuthenticateOAuthAccount() error = %v", err)
	}
	redirect, err := svc.Authorize(selected, testClientID, testRedirectURI, "switched-state")
	if err != nil {
		t.Fatal(err)
	}
	current, _, err := svc.AuthenticateSession(adminToken)
	if err != nil || current.Username != "admin" {
		t.Fatalf("original People session = %#v, %v", current, err)
	}
	var sessionsAfter int64
	if err := svc.store.DB.Model(&model.Session{}).Count(&sessionsAfter).Error; err != nil {
		t.Fatal(err)
	}
	if sessionsAfter != sessionsBefore {
		t.Fatalf("session count after OAuth account switch = %d, want %d", sessionsAfter, sessionsBefore)
	}
	result, err := svc.ExchangeCode(testClientID, testClientSecret, queryValue(t, redirect, "code"), testRedirectURI)
	if err != nil {
		t.Fatal(err)
	}
	if result.User == nil || result.User.Username != "alice" {
		t.Fatalf("switched OAuth identity = %#v", result.User)
	}
	if _, err := svc.AuthenticateOAuthAccount("alice", "wrong-password"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("wrong switched account password error = %v, want unauthorized", err)
	}
}
