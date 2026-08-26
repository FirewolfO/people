package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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
		&model.Department{}, &model.Position{}, &model.DepartmentPosition{}, &model.Employee{},
		&model.Session{}, &model.OAuthClient{}, &model.OAuthCode{}, &model.OAuthToken{},
		&model.DepartureRequest{}, &model.ApprovalRequest{}, &model.ApprovalStep{}, &model.LeaveRecord{}, &model.LeaveBalance{},
		&model.EmploymentEvent{}, &model.EmployeeContract{}, &model.PerformanceGoal{}, &model.Notification{},
	); err != nil {
		return nil, err
	}
	if err := migrateLegacyDepartments(db); err != nil {
		return nil, err
	}
	if err := seedPositions(db); err != nil {
		return nil, err
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("admin123!"), 12)
	if err != nil {
		return nil, err
	}
	admin := model.Employee{
		PublicID: "people-admin", LegacyEmployeeNo: "people-admin", Username: "admin", DisplayName: "系统管理员",
		PositionID: model.PositionSystemAdminID, Title: "系统管理员", Role: model.RoleAdmin, Status: model.StatusEnabled,
		PasswordHash: string(passwordHash), MustChangePassword: false,
	}
	adminCreate := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "username"}}, DoNothing: true}).
		Select("PublicID", "LegacyEmployeeNo", "Username", "DisplayName", "PositionID", "Title", "Role", "Status", "PasswordHash", "MustChangePassword").
		Create(&admin)
	if adminCreate.Error != nil {
		return nil, adminCreate.Error
	}
	if adminCreate.RowsAffected == 1 {
		if err := db.Model(&model.Employee{}).Where("id = ?", admin.ID).Update("must_change_password", false).Error; err != nil {
			return nil, err
		}
	} else {
		var existing model.Employee
		if err := db.Where("username = ?", "admin").First(&existing).Error; err != nil {
			return nil, err
		}
		if bcrypt.CompareHashAndPassword([]byte(existing.PasswordHash), []byte("admin")) == nil {
			if err := db.Transaction(func(tx *gorm.DB) error {
				updated := tx.Model(&model.Employee{}).
					Where("id = ? AND password_hash = ?", existing.ID, existing.PasswordHash).
					Update("password_hash", string(passwordHash))
				if updated.Error != nil {
					return updated.Error
				}
				if updated.RowsAffected == 0 {
					return errors.New("administrator password changed during migration")
				}
				return tx.Where("employee_id = ?", existing.ID).Delete(&model.Session{}).Error
			}); err != nil {
				return nil, err
			}
		}
	}
	if err := migrateLegacyPositions(db); err != nil {
		return nil, err
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

var commonPositions = []model.Position{
	{ID: model.PositionSystemAdminID, Code: "system_admin", Name: "系统管理员", Description: "People 内置系统管理员岗位", Status: model.StatusEnabled, Builtin: true},
	{ID: model.PositionGeneralID, Code: "general_employee", Name: "通用员工", Description: "旧数据迁移与待细分岗位的兜底岗位", Status: model.StatusEnabled, Builtin: true},
	{ID: "pos_ceo", Code: "ceo", Name: "首席执行官", Description: "负责公司整体战略与经营管理", Status: model.StatusEnabled},
	{ID: "pos_cto", Code: "cto", Name: "首席技术官", Description: "负责技术战略、架构与研发组织", Status: model.StatusEnabled},
	{ID: "pos_engineering_manager", Code: "engineering_manager", Name: "研发经理", Description: "负责研发团队交付与人员管理", Status: model.StatusEnabled},
	{ID: "pos_software_architect", Code: "software_architect", Name: "软件架构师", Description: "负责系统架构设计与技术治理", Status: model.StatusEnabled},
	{ID: "pos_backend_engineer", Code: "backend_engineer", Name: "后端开发工程师", Description: "负责服务端系统设计与开发", Status: model.StatusEnabled},
	{ID: "pos_frontend_engineer", Code: "frontend_engineer", Name: "前端开发工程师", Description: "负责 Web 前端应用设计与开发", Status: model.StatusEnabled},
	{ID: "pos_fullstack_engineer", Code: "fullstack_engineer", Name: "全栈开发工程师", Description: "负责前后端一体化应用开发", Status: model.StatusEnabled},
	{ID: "pos_mobile_engineer", Code: "mobile_engineer", Name: "移动端开发工程师", Description: "负责 iOS、Android 或跨端应用开发", Status: model.StatusEnabled},
	{ID: "pos_qa_engineer", Code: "qa_engineer", Name: "测试开发工程师", Description: "负责质量保障、自动化测试与效能建设", Status: model.StatusEnabled},
	{ID: "pos_devops_engineer", Code: "devops_engineer", Name: "DevOps 工程师", Description: "负责持续交付、基础设施与工程效率", Status: model.StatusEnabled},
	{ID: "pos_sre_engineer", Code: "sre_engineer", Name: "SRE 工程师", Description: "负责系统可靠性、可观测性与应急响应", Status: model.StatusEnabled},
	{ID: "pos_security_engineer", Code: "security_engineer", Name: "信息安全工程师", Description: "负责应用、基础设施和数据安全", Status: model.StatusEnabled},
	{ID: "pos_data_engineer", Code: "data_engineer", Name: "数据工程师", Description: "负责数据平台、数据链路与数据治理", Status: model.StatusEnabled},
	{ID: "pos_data_analyst", Code: "data_analyst", Name: "数据分析师", Description: "负责业务分析、指标体系与数据洞察", Status: model.StatusEnabled},
	{ID: "pos_ai_engineer", Code: "ai_engineer", Name: "AI 算法工程师", Description: "负责机器学习与人工智能应用研发", Status: model.StatusEnabled},
	{ID: "pos_dba", Code: "dba", Name: "数据库管理员", Description: "负责数据库可用性、性能与安全", Status: model.StatusEnabled},
	{ID: "pos_product_manager", Code: "product_manager", Name: "产品经理", Description: "负责产品规划、需求与生命周期管理", Status: model.StatusEnabled},
	{ID: "pos_project_manager", Code: "project_manager", Name: "项目经理", Description: "负责项目计划、协作与交付管理", Status: model.StatusEnabled},
	{ID: "pos_ux_designer", Code: "ux_designer", Name: "UI/UX 设计师", Description: "负责用户体验与界面设计", Status: model.StatusEnabled},
	{ID: "pos_it_support", Code: "it_support", Name: "IT 支持工程师", Description: "负责办公 IT、终端与员工技术支持", Status: model.StatusEnabled},
	{ID: "pos_network_engineer", Code: "network_engineer", Name: "网络工程师", Description: "负责企业网络建设、运维与安全", Status: model.StatusEnabled},
	{ID: "pos_hrbp", Code: "hrbp", Name: "人力资源业务伙伴", Description: "负责组织与人才相关人力资源工作", Status: model.StatusEnabled},
	{ID: "pos_finance", Code: "finance", Name: "财务专员", Description: "负责财务核算、预算与合规", Status: model.StatusEnabled},
	{ID: "pos_sales", Code: "sales", Name: "销售经理", Description: "负责客户拓展与销售目标", Status: model.StatusEnabled},
	{ID: "pos_customer_success", Code: "customer_success", Name: "客户成功经理", Description: "负责客户交付、采用与续约", Status: model.StatusEnabled},
	{ID: "pos_operations", Code: "operations", Name: "运营专员", Description: "负责产品或业务运营", Status: model.StatusEnabled},
}

func seedPositions(db *gorm.DB) error {
	for index := range commonPositions {
		position := commonPositions[index]
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&position).Error; err != nil {
			return err
		}
	}
	return nil
}

func migrateLegacyPositions(db *gorm.DB) error {
	var employees []model.Employee
	if err := db.Find(&employees).Error; err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		for _, employee := range employees {
			var position model.Position
			title := strings.TrimSpace(employee.Title)
			switch {
			case employee.Username == "admin":
				if err := tx.Where("id = ?", model.PositionSystemAdminID).First(&position).Error; err != nil {
					return err
				}
			case title == "":
				if err := tx.Where("id = ?", model.PositionGeneralID).First(&position).Error; err != nil {
					return err
				}
			default:
				err := tx.Where("LOWER(name) = ?", strings.ToLower(title)).First(&position).Error
				if errors.Is(err, gorm.ErrRecordNotFound) {
					suffix := hash(strings.ToLower(title))[:16]
					position = model.Position{ID: "pos_legacy_" + suffix, Code: "legacy_" + suffix, Name: title, Description: "由旧员工职务自动迁移", Status: model.StatusEnabled}
					if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&position).Error; err != nil {
						return err
					}
					if err := tx.Where("LOWER(name) = ?", strings.ToLower(title)).First(&position).Error; err != nil {
						return err
					}
				} else if err != nil {
					return err
				}
			}
			if employee.DepartmentID != "" {
				relation := model.DepartmentPosition{DepartmentID: employee.DepartmentID, PositionID: position.ID}
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&relation).Error; err != nil {
					return err
				}
			}
			if err := tx.Model(&model.Employee{}).Where("id = ?", employee.ID).Updates(map[string]any{"position_id": position.ID, "title": position.Name}).Error; err != nil {
				return err
			}
		}
		return nil
	})
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
