package service

import (
	"errors"
	"fmt"
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

func bindTestPosition(t *testing.T, svc *Service, departmentID, positionID string) string {
	t.Helper()
	positions, err := svc.ListPositions("", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, position := range positions {
		if position.ID != positionID {
			continue
		}
		for _, current := range position.DepartmentIDs {
			if current == departmentID {
				return position.ID
			}
		}
		position.DepartmentIDs = append(position.DepartmentIDs, departmentID)
		if _, err := svc.UpdatePosition(position.ID, PositionInput{
			Code: position.Code, Name: position.Name, Description: position.Description,
			DepartmentIDs: position.DepartmentIDs, Status: position.Status,
		}); err != nil {
			t.Fatal(err)
		}
		return position.ID
	}
	t.Fatalf("test position %q not found", positionID)
	return ""
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
		Username: "alice", DisplayName: "Alice", DepartmentID: department.ID,
		PositionID: bindTestPosition(t, svc, department.ID, model.PositionGeneralID), Role: model.RoleEmployee, Status: model.StatusEnabled,
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

func TestBuiltInAdminKeepsManagementPermissionsWithoutPermissionService(t *testing.T) {
	svc := newTestService(t)
	admin, _, err := svc.Login("admin", "admin")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	for _, code := range elevatedPermissions {
		if !svc.HasPermission(admin, code) {
			t.Fatalf("admin missing permission %q", code)
		}
	}
}

func TestEmployeeRequiresManagedDepartment(t *testing.T) {
	svc := newTestService(t)
	input := EmployeeInput{Username: "bob", DisplayName: "Bob", Role: model.RoleEmployee, Status: model.StatusEnabled}
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
	input.PositionID = bindTestPosition(t, svc, department.ID, model.PositionGeneralID)
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

func TestEmployeeRoleCannotBeElevatedThroughEmployeeInput(t *testing.T) {
	svc := newTestService(t)
	department, err := svc.CreateDepartment(DepartmentInput{Code: "security", Name: "安全部", Status: model.StatusEnabled})
	if err != nil {
		t.Fatal(err)
	}
	created, err := svc.CreateEmployee(EmployeeInput{
		Username: "operator", DisplayName: "Operator", DepartmentID: department.ID,
		PositionID: bindTestPosition(t, svc, department.ID, model.PositionGeneralID), Role: model.RoleAdmin, Status: model.StatusDisabled,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Role != model.RoleEmployee || created.Status != model.StatusEnabled {
		t.Fatalf("created employee privilege = %#v", created)
	}
}

func TestPositionManyToManyAndEmployeeAssignment(t *testing.T) {
	svc := newTestService(t)
	engineering, err := svc.CreateDepartment(DepartmentInput{Code: "engineering", Name: "研发部", Status: model.StatusEnabled})
	if err != nil {
		t.Fatal(err)
	}
	platform, err := svc.CreateDepartment(DepartmentInput{Code: "platform", Name: "平台部", Status: model.StatusEnabled})
	if err != nil {
		t.Fatal(err)
	}
	position, err := svc.CreatePosition(PositionInput{
		Code: "api_engineer", Name: "API 工程师", Description: "负责 API 研发",
		DepartmentIDs: []string{engineering.ID, platform.ID}, Status: model.StatusEnabled,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(position.DepartmentIDs) != 2 {
		t.Fatalf("position departments = %#v", position.DepartmentIDs)
	}
	filtered, err := svc.ListPositions("API", platform.ID)
	if err != nil || len(filtered) != 1 || filtered[0].ID != position.ID {
		t.Fatalf("filtered positions = %#v, %v", filtered, err)
	}
	employee, err := svc.CreateEmployee(EmployeeInput{
		Username: "apiuser", DisplayName: "API User", DepartmentID: engineering.ID, PositionID: position.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if employee.PositionID != position.ID || employee.Title != position.Name {
		t.Fatalf("employee position = %#v", employee)
	}
	if _, err := svc.CreateEmployee(EmployeeInput{
		Username: "invalidposition", DisplayName: "Invalid", DepartmentID: engineering.ID, PositionID: "pos_frontend_engineer",
	}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unrelated position error = %v, want invalid", err)
	}
	if _, err := svc.UpdatePosition(position.ID, PositionInput{
		Code: position.Code, Name: position.Name, Description: position.Description,
		DepartmentIDs: []string{platform.ID}, Status: model.StatusEnabled,
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("remove used department error = %v, want conflict", err)
	}
	updated, err := svc.UpdatePosition(position.ID, PositionInput{
		Code: position.Code, Name: "接口工程师", Description: position.Description,
		DepartmentIDs: position.DepartmentIDs, Status: model.StatusEnabled,
	})
	if err != nil {
		t.Fatal(err)
	}
	employee, err = svc.GetEmployee(employee.PublicID)
	if err != nil || employee.Title != updated.Name {
		t.Fatalf("renamed employee position = %#v, %v", employee, err)
	}
	if _, err := svc.UpdatePosition(position.ID, PositionInput{
		Code: position.Code, Name: updated.Name, Description: position.Description,
		DepartmentIDs: position.DepartmentIDs, Status: model.StatusDisabled,
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("disable used position error = %v, want conflict", err)
	}
	if err := svc.DeletePosition(position.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("delete used position error = %v, want conflict", err)
	}
	administrator, err := svc.GetEmployee("people-admin")
	if err != nil || administrator.PositionID != model.PositionSystemAdminID || administrator.Title != "系统管理员" {
		t.Fatalf("administrator position = %#v, %v", administrator, err)
	}
	administrator, err = svc.UpdateEmployee(administrator.PublicID, EmployeeInput{
		Username: "changed-admin", DisplayName: administrator.DisplayName, DepartmentID: platform.ID, PositionID: position.ID,
	})
	if err != nil || administrator.Username != "admin" || administrator.DepartmentID != "" || administrator.PositionID != model.PositionSystemAdminID {
		t.Fatalf("immutable administrator assignment = %#v, %v", administrator, err)
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
		Username: "alice", DisplayName: "Alice", DepartmentID: department.ID,
		PositionID: bindTestPosition(t, svc, department.ID, model.PositionGeneralID),
		Role:       model.RoleEmployee, Status: model.StatusEnabled,
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

func TestEmployeeNumberIsDatabaseGeneratedAndImmutable(t *testing.T) {
	svc := newTestService(t)
	department, err := svc.CreateDepartment(DepartmentInput{Code: "operations", Name: "运营部", Status: model.StatusEnabled})
	if err != nil {
		t.Fatal(err)
	}
	positionID := bindTestPosition(t, svc, department.ID, model.PositionGeneralID)
	first, err := svc.CreateEmployee(EmployeeInput{Username: "first", DisplayName: "First", DepartmentID: department.ID, PositionID: positionID, Role: model.RoleEmployee, Status: model.StatusEnabled})
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.CreateEmployee(EmployeeInput{Username: "second", DisplayName: "Second", DepartmentID: department.ID, PositionID: positionID, Role: model.RoleEmployee, Status: model.StatusEnabled})
	if err != nil {
		t.Fatal(err)
	}
	if first.EmployeeNo == 0 || second.EmployeeNo != first.EmployeeNo+1 {
		t.Fatalf("employee numbers = %d, %d, want consecutive generated values", first.EmployeeNo, second.EmployeeNo)
	}
	updated, err := svc.UpdateEmployee(first.PublicID, EmployeeInput{Username: "first", DisplayName: "First Updated", DepartmentID: department.ID, PositionID: positionID, Role: model.RoleEmployee, Status: model.StatusEnabled})
	if err != nil {
		t.Fatal(err)
	}
	if updated.EmployeeNo != first.EmployeeNo {
		t.Fatalf("updated employee number = %d, want %d", updated.EmployeeNo, first.EmployeeNo)
	}
	page, err := svc.ListEmployees(fmt.Sprintf("%06d", first.EmployeeNo), 1, 20)
	if err != nil || page.Total != 1 || page.Items[0].PublicID != first.PublicID {
		t.Fatalf("padded employee number search = %#v, %v", page, err)
	}
}

func TestDepartureRequiresLeaderThenHRAndDisablesEmployee(t *testing.T) {
	svc := newTestService(t)
	department, err := svc.CreateDepartment(DepartmentInput{Code: "support", Name: "客户支持", Status: model.StatusEnabled})
	if err != nil {
		t.Fatal(err)
	}
	positionID := bindTestPosition(t, svc, department.ID, model.PositionGeneralID)
	leader, err := svc.CreateEmployee(EmployeeInput{Username: "leader", DisplayName: "Leader", DepartmentID: department.ID, PositionID: positionID, Role: model.RoleEmployee, Status: model.StatusEnabled})
	if err != nil {
		t.Fatal(err)
	}
	worker, err := svc.CreateEmployee(EmployeeInput{Username: "worker", DisplayName: "Worker", DepartmentID: department.ID, PositionID: positionID, Role: model.RoleEmployee, Status: model.StatusEnabled})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateDepartment(department.ID, DepartmentInput{Code: department.Code, Name: department.Name, LeaderID: leader.PublicID, Status: model.StatusEnabled}); err != nil {
		t.Fatal(err)
	}
	request, err := svc.CreateDeparture(worker, DepartureInput{Reason: "个人发展", LastWorkingDate: time.Now().UTC().AddDate(0, 0, 14).Format("2006-01-02")})
	if err != nil {
		t.Fatal(err)
	}
	if request.Status != model.DeparturePendingManager {
		t.Fatalf("departure status = %q", request.Status)
	}
	if _, err := svc.ReviewDeparture(worker, request.ID, "manager", ReviewInput{Approved: true}, false); !errors.Is(err, ErrForbidden) {
		t.Fatalf("worker manager review error = %v, want forbidden", err)
	}
	request, err = svc.ReviewDeparture(leader, request.ID, "manager", ReviewInput{Approved: true, Comment: "同意"}, false)
	if err != nil || request.Status != model.DeparturePendingHR {
		t.Fatalf("manager review = %#v, %v", request, err)
	}
	if _, err := svc.ReviewDeparture(leader, request.ID, "hr", ReviewInput{Approved: true}, false); !errors.Is(err, ErrForbidden) {
		t.Fatalf("non-HR review error = %v, want forbidden", err)
	}
	request, err = svc.ReviewDeparture(leader, request.ID, "hr", ReviewInput{Approved: true, Comment: "完成交接"}, true)
	if err != nil || request.Status != model.DepartureApproved {
		t.Fatalf("HR review = %#v, %v", request, err)
	}
	disabled, err := svc.GetEmployee(worker.PublicID)
	if err != nil || disabled.Status != model.StatusDisabled {
		t.Fatalf("employee after approval = %#v, %v", disabled, err)
	}
	notifications, err := svc.ListNotifications(worker.PublicID, false)
	if err != nil || len(notifications) == 0 {
		t.Fatalf("applicant notifications = %#v, %v", notifications, err)
	}
}

func TestDepartmentLeaderDepartureFallsBackToAdministrator(t *testing.T) {
	svc := newTestService(t)
	department, err := svc.CreateDepartment(DepartmentInput{Code: "executive", Name: "管理部", Status: model.StatusEnabled})
	if err != nil {
		t.Fatal(err)
	}
	leader, err := svc.CreateEmployee(EmployeeInput{Username: "director", DisplayName: "Director", DepartmentID: department.ID, PositionID: bindTestPosition(t, svc, department.ID, model.PositionGeneralID)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateDepartment(department.ID, DepartmentInput{Code: department.Code, Name: department.Name, LeaderID: leader.PublicID, Status: model.StatusEnabled}); err != nil {
		t.Fatal(err)
	}
	request, err := svc.CreateDeparture(leader, DepartureInput{Reason: "个人发展", LastWorkingDate: time.Now().UTC().AddDate(0, 0, 7).Format("2006-01-02")})
	if err != nil {
		t.Fatal(err)
	}
	if request.DepartmentLeaderID != "people-admin" {
		t.Fatalf("leader departure approver = %q, want people-admin", request.DepartmentLeaderID)
	}
	administrator, err := svc.GetEmployee("people-admin")
	if err != nil {
		t.Fatal(err)
	}
	request, err = svc.ReviewDeparture(administrator, request.ID, "manager", ReviewInput{Approved: true}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReviewDeparture(leader, request.ID, "hr", ReviewInput{Approved: true}, true); !errors.Is(err, ErrForbidden) {
		t.Fatalf("self HR review error = %v, want forbidden", err)
	}
}
