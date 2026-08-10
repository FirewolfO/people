package service

import (
	"errors"
	"testing"
	"time"

	"people/internal/model"
)

func approvalFixture(t *testing.T) (*Service, *model.Department, *model.Employee, *model.Employee, *model.Employee) {
	t.Helper()
	svc := newTestService(t)
	department, err := svc.CreateDepartment(DepartmentInput{Code: "delivery", Name: "交付部", Status: model.StatusEnabled})
	if err != nil {
		t.Fatal(err)
	}
	positionID := bindTestPosition(t, svc, department.ID, "pos_backend_engineer")
	leader, err := svc.CreateEmployee(EmployeeInput{Username: "deliverylead", DisplayName: "交付负责人", DepartmentID: department.ID, PositionID: positionID})
	if err != nil {
		t.Fatal(err)
	}
	worker, err := svc.CreateEmployee(EmployeeInput{Username: "deliveryuser", DisplayName: "交付员工", DepartmentID: department.ID, PositionID: positionID, HireDate: "2025-01-02"})
	if err != nil {
		t.Fatal(err)
	}
	hr, err := svc.CreateEmployee(EmployeeInput{Username: "hrpartner", DisplayName: "HRBP", DepartmentID: department.ID, PositionID: positionID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateDepartment(department.ID, DepartmentInput{Code: department.Code, Name: department.Name, LeaderID: leader.PublicID, Status: model.StatusEnabled}); err != nil {
		t.Fatal(err)
	}
	return svc, department, leader, worker, hr
}

func TestDepartureIsGenericTwoStepApproval(t *testing.T) {
	svc, _, leader, worker, hr := approvalFixture(t)
	lastDay := time.Now().UTC().AddDate(0, 0, 14).Format("2006-01-02")
	request, err := svc.CreateApproval(worker, ApprovalInput{Type: model.ApprovalTypeDeparture, Data: map[string]any{
		"reason": "个人发展", "lastWorkingDate": lastDay,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if request.Type != model.ApprovalTypeDeparture || request.Status != model.ApprovalPending || len(request.Steps) != 2 {
		t.Fatalf("departure approval = %#v", request)
	}
	if request.Steps[0].ApproverID != leader.PublicID || request.Steps[0].Status != model.ApprovalStepPending || request.Steps[1].PermissionCode != PermissionApprovalReview {
		t.Fatalf("departure steps = %#v", request.Steps)
	}
	request, err = svc.ReviewApproval(leader, request.ID, ReviewInput{Approved: true, Comment: "已完成交接安排"}, false)
	if err != nil || request.CurrentStep != 2 || request.Steps[1].Status != model.ApprovalStepPending {
		t.Fatalf("manager review = %#v, %v", request, err)
	}
	if _, err := svc.ReviewApproval(worker, request.ID, ReviewInput{Approved: true}, true); !errors.Is(err, ErrForbidden) {
		t.Fatalf("self HR review error = %v, want forbidden", err)
	}
	request, err = svc.ReviewApproval(hr, request.ID, ReviewInput{Approved: true, Comment: "离职手续完成"}, true)
	if err != nil || request.Status != model.ApprovalApproved {
		t.Fatalf("HR review = %#v, %v", request, err)
	}
	disabled, err := svc.GetEmployee(worker.PublicID)
	if err != nil || disabled.Status != model.StatusDisabled {
		t.Fatalf("employee after departure = %#v, %v", disabled, err)
	}
	events, err := svc.ListEmploymentEvents(worker.PublicID)
	if err != nil || len(events) < 2 || events[0].Type != model.EmploymentEventDeparture {
		t.Fatalf("employment events = %#v, %v", events, err)
	}
}

func TestLeaveApprovalCalculatesWorkingDaysAndBalance(t *testing.T) {
	svc, _, leader, worker, _ := approvalFixture(t)
	start := nextWeekday(time.Now().UTC().AddDate(0, 0, 2))
	end := start.AddDate(0, 0, 2)
	request, err := svc.CreateApproval(worker, ApprovalInput{Type: model.ApprovalTypeLeave, Data: map[string]any{
		"leaveType": "annual", "startDate": start.Format("2006-01-02"), "endDate": end.Format("2006-01-02"), "reason": "家庭安排",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if request.Data["days"].(float64) < 1 || len(request.Steps) != 1 {
		t.Fatalf("leave approval = %#v", request)
	}
	pendingBalance, err := svc.GetLeaveBalance(worker, start.Year())
	if err != nil || pendingBalance.AnnualPending <= 0 || pendingBalance.AnnualRemaining >= pendingBalance.AnnualEntitlement {
		t.Fatalf("pending leave balance = %#v, %v", pendingBalance, err)
	}
	request, err = svc.ReviewApproval(leader, request.ID, ReviewInput{Approved: true}, false)
	if err != nil || request.Status != model.ApprovalApproved {
		t.Fatalf("leave review = %#v, %v", request, err)
	}
	balance, err := svc.GetLeaveBalance(worker, time.Now().UTC().Year())
	if err != nil || balance.AnnualUsed <= 0 || balance.AnnualRemaining >= balance.AnnualEntitlement {
		t.Fatalf("leave balance = %#v, %v", balance, err)
	}
	calendar, err := svc.ListLeaveCalendar(worker, false, start.Format("2006-01"))
	if err != nil || len(calendar) != 1 || calendar[0].Status != model.ApprovalApproved {
		t.Fatalf("leave calendar = %#v, %v", calendar, err)
	}
}

func TestTransferApprovalUpdatesMasterDataAndHistory(t *testing.T) {
	svc, source, leader, worker, hr := approvalFixture(t)
	target, err := svc.CreateDepartment(DepartmentInput{Code: "platform", Name: "平台部", Status: model.StatusEnabled})
	if err != nil {
		t.Fatal(err)
	}
	targetPositionID := bindTestPosition(t, svc, target.ID, "pos_software_architect")
	request, err := svc.CreateApproval(worker, ApprovalInput{Type: model.ApprovalTypeTransfer, Data: map[string]any{
		"targetDepartmentId": target.ID, "targetPositionId": targetPositionID, "effectiveDate": time.Now().UTC().AddDate(0, 0, 7).Format("2006-01-02"), "reason": "内部发展",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReviewApproval(leader, request.ID, ReviewInput{Approved: true}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReviewApproval(hr, request.ID, ReviewInput{Approved: true}, true); err != nil {
		t.Fatal(err)
	}
	updated, err := svc.GetEmployee(worker.PublicID)
	if err != nil || updated.DepartmentID != target.ID || updated.PositionID != targetPositionID || updated.Title != "软件架构师" {
		t.Fatalf("employee after transfer = %#v, %v", updated, err)
	}
	if updated.PublicID != worker.PublicID || updated.EmployeeNo != worker.EmployeeNo {
		t.Fatalf("stable identity changed: before=%#v after=%#v", worker, updated)
	}
	events, err := svc.ListEmploymentEvents(worker.PublicID)
	if err != nil || len(events) < 2 || events[0].Type != model.EmploymentEventTransfer || events[0].FromDepartmentID != source.ID || events[0].ToDepartmentID != target.ID {
		t.Fatalf("transfer events = %#v, %v", events, err)
	}
}

func TestContractsGoalsAndSelfServiceFormUsefulEmployeeRecord(t *testing.T) {
	svc, _, leader, worker, _ := approvalFixture(t)
	profile, err := svc.UpdateMyProfile(worker, ProfileInput{Email: "worker@example.com", Phone: "13800138000", EmergencyContactName: "家属", EmergencyContactPhone: "13900139000", EmergencyContactRelation: "配偶"})
	if err != nil || profile.EmergencyContactName != "家属" {
		t.Fatalf("profile = %#v, %v", profile, err)
	}
	contract, err := svc.CreateContract(worker.PublicID, ContractInput{Type: "fixed_term", StartDate: "2025-01-02", EndDate: time.Now().UTC().AddDate(0, 1, 0).Format("2006-01-02"), Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	contracts, err := svc.ListContracts(worker, false, "")
	if err != nil || len(contracts) != 1 || contracts[0].ID != contract.ID {
		t.Fatalf("own contracts = %#v, %v", contracts, err)
	}
	goal, err := svc.CreateGoal(worker, GoalInput{Cycle: "2026-H2", Title: "提升交付质量", Description: "降低线上缺陷", DueDate: "2026-12-31", Weight: 40})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateGoal(worker, goal.ID, GoalInput{Cycle: goal.Cycle, Title: goal.Title, Description: goal.Description, DueDate: goal.DueDate, Weight: goal.Weight, Progress: 60, Status: "active"}, false); err != nil {
		t.Fatal(err)
	}
	goals, err := svc.ListGoals(leader, false, "")
	if err != nil || len(goals) != 1 || !goals[0].CanReview {
		t.Fatalf("leader goals = %#v, %v", goals, err)
	}
}

func TestNewPeopleCollectionsAreEmptyArrays(t *testing.T) {
	svc := newTestService(t)
	dashboard, err := svc.Dashboard()
	if err != nil {
		t.Fatal(err)
	}
	if dashboard.DepartmentDistribution == nil || dashboard.EmploymentTypeDistribution == nil || dashboard.ApprovalDistribution == nil {
		t.Fatalf("dashboard distributions must be empty arrays: %#v", dashboard)
	}
	admin, err := svc.GetEmployee("people-admin")
	if err != nil {
		t.Fatal(err)
	}
	approvals, err := svc.ListApprovals(admin, ApprovalFilter{Scope: "all"}, true, true)
	if err != nil || approvals == nil {
		t.Fatalf("approvals = %#v, %v", approvals, err)
	}
	contracts, err := svc.ListContracts(admin, true, "")
	if err != nil || contracts == nil {
		t.Fatalf("contracts = %#v, %v", contracts, err)
	}
	goals, err := svc.ListGoals(admin, true, "")
	if err != nil || goals == nil {
		t.Fatalf("goals = %#v, %v", goals, err)
	}
}

func nextWeekday(value time.Time) time.Time {
	for value.Weekday() == time.Saturday || value.Weekday() == time.Sunday {
		value = value.AddDate(0, 0, 1)
	}
	return value
}
