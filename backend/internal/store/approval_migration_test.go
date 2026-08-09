package store

import (
	"path/filepath"
	"testing"
	"time"

	"people/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestOpenMigratesLegacyDepartureIntoApprovalWorkflow(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "people.db")
	legacyDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := legacyDB.AutoMigrate(&model.Department{}, &model.Employee{}, &model.DepartureRequest{}); err != nil {
		t.Fatal(err)
	}
	department := model.Department{ID: "dep_legacy", Code: "legacy", Name: "历史部门", LeaderID: "pep_leader", Status: model.StatusEnabled}
	if err := legacyDB.Create(&department).Error; err != nil {
		t.Fatal(err)
	}
	employee := model.Employee{
		PublicID: "pep_legacy_departure", LegacyEmployeeNo: "legacy-departure", Username: "legacydeparture",
		DisplayName: "历史员工", DepartmentID: department.ID, Department: department.Name,
		Role: model.RoleEmployee, Status: model.StatusEnabled, EmploymentType: "full_time",
	}
	if err := legacyDB.Create(&employee).Error; err != nil {
		t.Fatal(err)
	}
	reviewedAt := time.Now().UTC().Add(-time.Hour)
	departure := model.DepartureRequest{
		ID: "dpr_legacy", EmployeeID: employee.ID, EmployeePublicID: employee.PublicID, EmployeeName: employee.DisplayName,
		EmployeeNo: employee.ID, DepartmentID: department.ID, DepartmentName: department.Name, DepartmentLeaderID: department.LeaderID,
		Reason: "个人发展", LastWorkingDate: "2026-08-31", Status: model.DeparturePendingHR,
		ManagerReviewerID: department.LeaderID, ManagerReviewerName: "历史负责人", ManagerReviewComment: "同意", ManagerReviewedAt: &reviewedAt,
	}
	if err := legacyDB.Create(&departure).Error; err != nil {
		t.Fatal(err)
	}
	legacySQL, err := legacyDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := legacySQL.Close(); err != nil {
		t.Fatal(err)
	}

	database, err := Open(dsn, "permission-ui", "secret", []string{"http://localhost/callback"})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	var approval model.ApprovalRequest
	if err := database.DB.Preload("Steps").Where("id = ?", departure.ID).First(&approval).Error; err != nil {
		t.Fatal(err)
	}
	if approval.Type != model.ApprovalTypeDeparture || approval.Status != model.ApprovalPending || approval.CurrentStep != 2 || approval.Data["reason"] != departure.Reason {
		t.Fatalf("migrated approval = %#v", approval)
	}
	if len(approval.Steps) != 2 || approval.Steps[0].Status != model.ApprovalStepApproved || approval.Steps[1].Status != model.ApprovalStepPending || approval.Steps[1].PermissionCode != "people.approval:review" {
		t.Fatalf("migrated steps = %#v", approval.Steps)
	}

	if err := migrateLegacyDepartures(database.DB); err != nil {
		t.Fatal(err)
	}
	var approvals, steps int64
	if err := database.DB.Model(&model.ApprovalRequest{}).Where("id = ?", departure.ID).Count(&approvals).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Model(&model.ApprovalStep{}).Where("approval_id = ?", departure.ID).Count(&steps).Error; err != nil {
		t.Fatal(err)
	}
	if approvals != 1 || steps != 2 {
		t.Fatalf("idempotent migration counts = approvals %d, steps %d", approvals, steps)
	}
}
