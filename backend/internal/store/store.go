package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"people/internal/model"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Store struct {
	DB *gorm.DB
}

type OAuthClientSeed struct {
	ClientID      string
	Name          string
	ClientSecret  string
	RedirectURIs  []string
	AllowedScopes []string
}

func Open(dsn, permissionClientID, permissionClientSecret string, redirectURIs []string, additionalClients ...OAuthClientSeed) (*Store, error) {
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(
		&model.Department{}, &model.Employee{}, &model.Session{}, &model.OAuthClient{}, &model.OAuthCode{}, &model.OAuthToken{},
		&model.DepartureRequest{}, &model.ApprovalRequest{}, &model.ApprovalStep{}, &model.LeaveRecord{}, &model.LeaveBalance{},
		&model.EmploymentEvent{}, &model.EmployeeContract{}, &model.PerformanceGoal{}, &model.Notification{},
	); err != nil {
		return nil, err
	}
	if err := migrateLegacyDepartments(db); err != nil {
		return nil, err
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("admin"), 12)
	if err != nil {
		return nil, err
	}
	admin := model.Employee{
		PublicID: "people-admin", LegacyEmployeeNo: "people-admin", Username: "admin", DisplayName: "系统管理员",
		Role: model.RoleAdmin, Status: model.StatusEnabled, PasswordHash: string(passwordHash), MustChangePassword: false,
	}
	adminCreate := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "username"}}, DoNothing: true}).
		Select("PublicID", "LegacyEmployeeNo", "Username", "DisplayName", "Role", "Status", "PasswordHash", "MustChangePassword").
		Create(&admin)
	if adminCreate.Error != nil {
		return nil, adminCreate.Error
	}
	if adminCreate.RowsAffected == 1 {
		if err := db.Model(&model.Employee{}).Where("id = ?", admin.ID).Update("must_change_password", false).Error; err != nil {
			return nil, err
		}
	}
	if err := migrateLegacyDepartures(db); err != nil {
		return nil, err
	}
	if err := seedEmploymentEvents(db); err != nil {
		return nil, err
	}
	clients := append([]OAuthClientSeed{{
		ClientID: permissionClientID, Name: "权限系统", ClientSecret: permissionClientSecret,
		RedirectURIs: redirectURIs, AllowedScopes: []string{"openid", "profile", "employees.read"},
	}}, additionalClients...)
	for _, seed := range clients {
		client := model.OAuthClient{
			ClientID: seed.ClientID, Name: seed.Name, SecretHash: hash(seed.ClientSecret),
			RedirectURIs: strings.Join(seed.RedirectURIs, "\n"), AllowedScopes: strings.Join(seed.AllowedScopes, " "),
		}
		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "client_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "secret_hash", "redirect_uris", "allowed_scopes", "updated_at"}),
		}).Create(&client).Error; err != nil {
			return nil, err
		}
	}
	return &Store{DB: db}, nil
}

func migrateLegacyDepartures(db *gorm.DB) error {
	var departures []model.DepartureRequest
	if err := db.Find(&departures).Error; err != nil {
		return err
	}
	for _, departure := range departures {
		var count int64
		if err := db.Model(&model.ApprovalRequest{}).Where("id = ?", departure.ID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		data, err := json.Marshal(map[string]any{"reason": departure.Reason, "lastWorkingDate": departure.LastWorkingDate})
		if err != nil {
			return err
		}
		status, currentStep := model.ApprovalPending, 1
		var completedAt *time.Time
		switch departure.Status {
		case model.DeparturePendingHR:
			currentStep = 2
		case model.DepartureApproved:
			status, currentStep, completedAt = model.ApprovalApproved, 2, departure.HRReviewedAt
		case model.DepartureRejected:
			status, completedAt = model.ApprovalRejected, departure.ManagerReviewedAt
			if departure.HRReviewedAt != nil {
				currentStep, completedAt = 2, departure.HRReviewedAt
			}
		case model.DepartureCancelled:
			status, completedAt = model.ApprovalCancelled, &departure.UpdatedAt
		}
		approval := model.ApprovalRequest{
			ID: departure.ID, Type: model.ApprovalTypeDeparture, Title: "离职申请", Summary: departure.EmployeeName + " 的离职申请",
			ApplicantID: departure.EmployeeID, ApplicantPublicID: departure.EmployeePublicID, ApplicantName: departure.EmployeeName,
			ApplicantNo: departure.EmployeeNo, DepartmentID: departure.DepartmentID, DepartmentName: departure.DepartmentName,
			DataJSON: string(data), Status: status, CurrentStep: currentStep, TotalSteps: 2,
			SubmittedAt: departure.CreatedAt, CompletedAt: completedAt, CreatedAt: departure.CreatedAt, UpdatedAt: departure.UpdatedAt,
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&approval).Error; err != nil {
				return err
			}
			managerStatus, hrStatus := model.ApprovalStepPending, model.ApprovalStepWaiting
			if departure.Status == model.DeparturePendingHR || departure.Status == model.DepartureApproved || (departure.Status == model.DepartureRejected && departure.HRReviewedAt != nil) {
				managerStatus, hrStatus = model.ApprovalStepApproved, model.ApprovalStepPending
			}
			if departure.Status == model.DepartureApproved {
				hrStatus = model.ApprovalStepApproved
			} else if departure.Status == model.DepartureRejected {
				if departure.HRReviewedAt != nil {
					hrStatus = model.ApprovalStepRejected
				} else {
					managerStatus, hrStatus = model.ApprovalStepRejected, model.ApprovalStepSkipped
				}
			} else if departure.Status == model.DepartureCancelled {
				managerStatus, hrStatus = model.ApprovalStepSkipped, model.ApprovalStepSkipped
			}
			steps := []model.ApprovalStep{
				{ApprovalID: departure.ID, Sequence: 1, Name: "部门负责人审批", ApproverID: departure.DepartmentLeaderID, Status: managerStatus, ReviewerID: departure.ManagerReviewerID, ReviewerName: departure.ManagerReviewerName, Comment: departure.ManagerReviewComment, ReviewedAt: departure.ManagerReviewedAt},
				{ApprovalID: departure.ID, Sequence: 2, Name: "HR 审批", PermissionCode: "people.approval:review", Status: hrStatus, ReviewerID: departure.HRReviewerID, ReviewerName: departure.HRReviewerName, Comment: departure.HRReviewComment, ReviewedAt: departure.HRReviewedAt},
			}
			return tx.Create(&steps).Error
		}); err != nil {
			return err
		}
	}
	return nil
}

func seedEmploymentEvents(db *gorm.DB) error {
	var employees []model.Employee
	if err := db.Find(&employees).Error; err != nil {
		return err
	}
	for _, employee := range employees {
		id := "evt_seed_" + hash(employee.PublicID)[:20]
		effectiveDate := employee.HireDate
		if effectiveDate == "" {
			effectiveDate = employee.CreatedAt.UTC().Format("2006-01-02")
		}
		event := model.EmploymentEvent{
			ID: id, EmployeeID: employee.ID, EmployeePublicID: employee.PublicID, Type: model.EmploymentEventHire,
			EffectiveDate: effectiveDate, ToDepartmentID: employee.DepartmentID, ToDepartment: employee.Department,
			ToTitle: employee.Title, Note: "员工加入组织", CreatedAt: employee.CreatedAt,
		}
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&event).Error; err != nil {
			return err
		}
	}
	return nil
}

func migrateLegacyDepartments(db *gorm.DB) error {
	var names []string
	if err := db.Model(&model.Employee{}).
		Where("department <> '' AND (department_id = '' OR department_id IS NULL)").
		Distinct("department").Pluck("department", &names).Error; err != nil {
		return err
	}
	for _, rawName := range names {
		name := strings.TrimSpace(rawName)
		if name == "" {
			continue
		}
		suffix := hash(strings.ToLower(name))[:12]
		department := model.Department{
			ID: "dep_" + suffix, Code: "legacy_" + suffix, Name: name, Status: model.StatusEnabled,
		}
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&department).Error; err != nil {
			return err
		}
		if err := db.Where("name = ?", name).First(&department).Error; err != nil {
			return err
		}
		if err := db.Model(&model.Employee{}).
			Where("department = ? AND (department_id = '' OR department_id IS NULL)", rawName).
			Updates(map[string]any{"department_id": department.ID, "department": department.Name}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Close() error {
	db, err := s.DB.DB()
	if err != nil {
		return err
	}
	return db.Close()
}

func hash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
