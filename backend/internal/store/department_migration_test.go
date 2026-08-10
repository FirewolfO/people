package store

import (
	"path/filepath"
	"testing"
	"time"

	"people/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type legacyEmployee struct {
	ID                 uint `gorm:"primaryKey"`
	PublicID           string
	EmployeeNo         string
	Username           string
	DisplayName        string
	Email              string
	Phone              string
	Department         string
	Title              string
	Role               string
	Status             string
	PasswordHash       string
	MustChangePassword bool
	PasswordChangedAt  *time.Time
	LastLoginAt        *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (legacyEmployee) TableName() string { return "employees" }

func TestOpenMigratesLegacyDepartmentText(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "people.db")
	legacyDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := legacyDB.AutoMigrate(&legacyEmployee{}); err != nil {
		t.Fatal(err)
	}
	if err := legacyDB.Create(&legacyEmployee{
		PublicID: "pep_legacy", EmployeeNo: "E001", Username: "legacy", DisplayName: "Legacy",
		Department: " 研发部 ", Title: "后端开发工程师", Role: model.RoleEmployee, Status: model.StatusEnabled,
	}).Error; err != nil {
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

	var employee model.Employee
	if err := database.DB.Where("public_id = ?", "pep_legacy").First(&employee).Error; err != nil {
		t.Fatal(err)
	}
	if employee.DepartmentID == "" || employee.Department != "研发部" || employee.PositionID != "pos_backend_engineer" || employee.Title != "后端开发工程师" {
		t.Fatalf("migrated employee = %#v", employee)
	}
	var department model.Department
	if err := database.DB.Where("id = ?", employee.DepartmentID).First(&department).Error; err != nil {
		t.Fatal(err)
	}
	if department.Name != "研发部" || department.Status != model.StatusEnabled {
		t.Fatalf("migrated department = %#v", department)
	}
	var relation model.DepartmentPosition
	if err := database.DB.Where("department_id = ? AND position_id = ?", department.ID, employee.PositionID).First(&relation).Error; err != nil {
		t.Fatal(err)
	}
	var positions int64
	if err := database.DB.Model(&model.Position{}).Count(&positions).Error; err != nil {
		t.Fatal(err)
	}
	if positions < 20 {
		t.Fatalf("seeded positions = %d, want common IT position catalog", positions)
	}
	var relations int64
	if err := database.DB.Model(&model.DepartmentPosition{}).Count(&relations).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateLegacyPositions(database.DB); err != nil {
		t.Fatal(err)
	}
	var positionsAfter, relationsAfter int64
	if err := database.DB.Model(&model.Position{}).Count(&positionsAfter).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Model(&model.DepartmentPosition{}).Count(&relationsAfter).Error; err != nil {
		t.Fatal(err)
	}
	if positionsAfter != positions || relationsAfter != relations {
		t.Fatalf("idempotent position migration counts = positions %d/%d, relations %d/%d", positions, positionsAfter, relations, relationsAfter)
	}
}
