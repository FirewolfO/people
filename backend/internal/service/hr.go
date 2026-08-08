package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"people/internal/model"

	"gorm.io/gorm"
)

type DepartureInput struct {
	Reason          string `json:"reason"`
	LastWorkingDate string `json:"lastWorkingDate"`
}

type ReviewInput struct {
	Approved bool   `json:"approved"`
	Comment  string `json:"comment"`
}

type NotificationSummary struct {
	Unread       int64 `json:"unread"`
	PendingTasks int64 `json:"pendingTasks"`
	Total        int64 `json:"total"`
}

type HRDashboard struct {
	TotalEmployees     int64 `json:"totalEmployees"`
	EnabledEmployees   int64 `json:"enabledEmployees"`
	DisabledEmployees  int64 `json:"disabledEmployees"`
	Departments        int64 `json:"departments"`
	PendingDepartures  int64 `json:"pendingDepartures"`
	ProbationEmployees int64 `json:"probationEmployees"`
	RecentHires        int64 `json:"recentHires"`
}

func (s *Service) ResetEmployeePassword(publicID string) error {
	var employee model.Employee
	if err := s.store.DB.Where("public_id = ?", strings.TrimSpace(publicID)).First(&employee).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if employee.Username == "admin" {
		return fmt.Errorf("%w: 不能重置内置管理员密码", ErrInvalid)
	}
	now := time.Now().UTC()
	return s.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&employee).Updates(map[string]any{"password_hash": "", "must_change_password": true, "password_changed_at": nil}).Error; err != nil {
			return err
		}
		return tx.Model(&model.Session{}).Where("employee_id = ? AND revoked_at IS NULL", employee.ID).Update("revoked_at", now).Error
	})
}

func (s *Service) SetEmployeeEnabled(publicID string, enabled bool) (*model.Employee, error) {
	var employee model.Employee
	if err := s.store.DB.Where("public_id = ?", strings.TrimSpace(publicID)).First(&employee).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	if employee.Username == "admin" && !enabled {
		return nil, fmt.Errorf("%w: 不能停用内置管理员", ErrInvalid)
	}
	if !enabled {
		var leaderCount int64
		if err := s.store.DB.Model(&model.Department{}).Where("leader_id = ?", employee.PublicID).Count(&leaderCount).Error; err != nil {
			return nil, err
		}
		if leaderCount > 0 {
			return nil, fmt.Errorf("%w: 请先为其负责的部门更换负责人", ErrConflict)
		}
	}
	status := model.StatusDisabled
	if enabled {
		status = model.StatusEnabled
	}
	now := time.Now().UTC()
	err := s.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&employee).Update("status", status).Error; err != nil {
			return err
		}
		if enabled {
			return nil
		}
		return tx.Model(&model.Session{}).Where("employee_id = ? AND revoked_at IS NULL", employee.ID).Update("revoked_at", now).Error
	})
	if err != nil {
		return nil, err
	}
	return s.GetEmployee(employee.PublicID)
}

func (s *Service) Dashboard() (HRDashboard, error) {
	result := HRDashboard{}
	queries := []struct {
		model any
		where string
		args  []any
		value *int64
	}{
		{&model.Employee{}, "", nil, &result.TotalEmployees},
		{&model.Employee{}, "status = ?", []any{model.StatusEnabled}, &result.EnabledEmployees},
		{&model.Employee{}, "status = ?", []any{model.StatusDisabled}, &result.DisabledEmployees},
		{&model.Department{}, "status = ?", []any{model.StatusEnabled}, &result.Departments},
		{&model.DepartureRequest{}, "status IN ?", []any{[]string{model.DeparturePendingManager, model.DeparturePendingHR}}, &result.PendingDepartures},
		{&model.Employee{}, "status = ? AND probation_end_date >= ?", []any{model.StatusEnabled, time.Now().UTC().Format("2006-01-02")}, &result.ProbationEmployees},
		{&model.Employee{}, "hire_date >= ?", []any{time.Now().UTC().AddDate(0, 0, -30).Format("2006-01-02")}, &result.RecentHires},
	}
	for _, item := range queries {
		query := s.store.DB.Model(item.model)
		if item.where != "" {
			query = query.Where(item.where, item.args...)
		}
		if err := query.Count(item.value).Error; err != nil {
			return HRDashboard{}, err
		}
	}
	return result, nil
}

func (s *Service) CreateDeparture(employee *model.Employee, input DepartureInput) (*model.DepartureRequest, error) {
	if employee == nil || employee.Role == model.RoleAdmin || employee.Status != model.StatusEnabled {
		return nil, fmt.Errorf("%w: 管理员或停用账号不能发起离职", ErrForbidden)
	}
	reason := strings.TrimSpace(input.Reason)
	lastWorkingDate := strings.TrimSpace(input.LastWorkingDate)
	date, err := time.Parse("2006-01-02", lastWorkingDate)
	if reason == "" || len(reason) > 1000 || err != nil {
		return nil, fmt.Errorf("%w: 离职原因和最后工作日不能为空", ErrInvalid)
	}
	if date.Before(time.Now().UTC().Truncate(24 * time.Hour)) {
		return nil, fmt.Errorf("%w: 最后工作日不能早于今天", ErrInvalid)
	}
	var department model.Department
	if employee.DepartmentID == "" || s.store.DB.Where("id = ? AND status = ?", employee.DepartmentID, model.StatusEnabled).First(&department).Error != nil {
		return nil, fmt.Errorf("%w: 所在部门不存在或已停用", ErrInvalid)
	}
	approverID, err := s.departureApprover(employee, department)
	if err != nil {
		return nil, err
	}
	var active int64
	if err := s.store.DB.Model(&model.DepartureRequest{}).Where("employee_id = ? AND status IN ?", employee.ID, []string{model.DeparturePendingManager, model.DeparturePendingHR}).Count(&active).Error; err != nil {
		return nil, err
	}
	if active > 0 {
		return nil, fmt.Errorf("%w: 已存在待审批的离职申请", ErrConflict)
	}
	id, err := randomToken("dpr_", 18)
	if err != nil {
		return nil, err
	}
	request := model.DepartureRequest{
		ID: id, EmployeeID: employee.ID, EmployeePublicID: employee.PublicID, EmployeeName: employee.DisplayName,
		EmployeeNo: employee.EmployeeNo, DepartmentID: department.ID, DepartmentName: department.Name,
		DepartmentLeaderID: approverID, Reason: reason, LastWorkingDate: lastWorkingDate,
		Status: model.DeparturePendingManager,
	}
	return &request, s.store.DB.Create(&request).Error
}

func (s *Service) departureApprover(employee *model.Employee, department model.Department) (string, error) {
	current := department
	for {
		if current.LeaderID != "" && current.LeaderID != employee.PublicID {
			var leader model.Employee
			if err := s.store.DB.Where("public_id = ? AND status = ?", current.LeaderID, model.StatusEnabled).First(&leader).Error; err == nil {
				return leader.PublicID, nil
			}
		}
		if current.ParentID == "" {
			break
		}
		if err := s.store.DB.Where("id = ? AND status = ?", current.ParentID, model.StatusEnabled).First(&current).Error; err != nil {
			break
		}
	}
	var administrator model.Employee
	if err := s.store.DB.Where("role = ? AND status = ?", model.RoleAdmin, model.StatusEnabled).Order("id ASC").First(&administrator).Error; err != nil {
		return "", fmt.Errorf("%w: 无可用的直属审批负责人", ErrConflict)
	}
	return administrator.PublicID, nil
}

func (s *Service) ListDepartures(actor *model.Employee, canHR bool) ([]model.DepartureRequest, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	query := s.store.DB.Model(&model.DepartureRequest{})
	if !canHR {
		query = query.Where("employee_public_id = ? OR department_leader_id = ?", actor.PublicID, actor.PublicID)
	}
	var items []model.DepartureRequest
	if err := query.Order("created_at DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	for index := range items {
		items[index].CanManagerReview = items[index].Status == model.DeparturePendingManager && items[index].DepartmentLeaderID == actor.PublicID
		items[index].CanHRReview = items[index].Status == model.DeparturePendingHR && canHR
		items[index].CanCancel = items[index].EmployeePublicID == actor.PublicID && items[index].Status == model.DeparturePendingManager
	}
	return items, nil
}

func (s *Service) ReviewDeparture(actor *model.Employee, id, stage string, input ReviewInput, canHR bool) (*model.DepartureRequest, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	comment := strings.TrimSpace(input.Comment)
	if len(comment) > 500 {
		return nil, fmt.Errorf("%w: 审批意见过长", ErrInvalid)
	}
	var result model.DepartureRequest
	err := s.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", strings.TrimSpace(id)).First(&result).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		now := time.Now().UTC()
		switch stage {
		case "manager":
			if result.Status != model.DeparturePendingManager {
				return fmt.Errorf("%w: 当前状态不能进行负责人审批", ErrConflict)
			}
			if result.DepartmentLeaderID != actor.PublicID {
				return ErrForbidden
			}
			status := model.DepartureRejected
			if input.Approved {
				status = model.DeparturePendingHR
			}
			if err := tx.Model(&result).Updates(map[string]any{
				"status": status, "manager_reviewer_id": actor.PublicID, "manager_reviewer_name": actor.DisplayName,
				"manager_review_comment": comment, "manager_reviewed_at": now,
			}).Error; err != nil {
				return err
			}
			if !input.Approved {
				return createNotification(tx, result.EmployeePublicID, "departure_result", "离职申请已驳回", "部门负责人驳回了你的离职申请", "departure", result.ID)
			}
		case "hr":
			if !canHR {
				return ErrForbidden
			}
			if result.EmployeePublicID == actor.PublicID {
				return fmt.Errorf("%w: 不能审批自己的离职申请", ErrForbidden)
			}
			if result.Status != model.DeparturePendingHR {
				return fmt.Errorf("%w: 当前状态不能进行 HR 审批", ErrConflict)
			}
			status := model.DepartureRejected
			if input.Approved {
				status = model.DepartureApproved
			}
			if err := tx.Model(&result).Updates(map[string]any{
				"status": status, "hr_reviewer_id": actor.PublicID, "hr_reviewer_name": actor.DisplayName,
				"hr_review_comment": comment, "hr_reviewed_at": now,
			}).Error; err != nil {
				return err
			}
			if input.Approved {
				if err := tx.Model(&model.Employee{}).Where("id = ?", result.EmployeeID).Update("status", model.StatusDisabled).Error; err != nil {
					return err
				}
				if err := tx.Model(&model.Session{}).Where("employee_id = ? AND revoked_at IS NULL", result.EmployeeID).Update("revoked_at", now).Error; err != nil {
					return err
				}
				if err := tx.Where("employee_id = ?", result.EmployeeID).Delete(&model.OAuthToken{}).Error; err != nil {
					return err
				}
			}
			title, content := "离职申请已驳回", "HR 驳回了你的离职申请"
			if input.Approved {
				title, content = "离职申请已通过", "离职申请审批完成，账号已停止使用"
			}
			return createNotification(tx, result.EmployeePublicID, "departure_result", title, content, "departure", result.ID)
		default:
			return fmt.Errorf("%w: 审批阶段无效", ErrInvalid)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := s.store.DB.Where("id = ?", result.ID).First(&result).Error; err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *Service) CancelDeparture(actor *model.Employee, id string) error {
	if actor == nil {
		return ErrUnauthorized
	}
	result := s.store.DB.Model(&model.DepartureRequest{}).Where("id = ? AND employee_id = ? AND status = ?", id, actor.ID, model.DeparturePendingManager).Update("status", model.DepartureCancelled)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("%w: 当前申请不能撤回", ErrConflict)
	}
	return nil
}

func (s *Service) ListNotifications(recipientID string, unreadOnly bool) ([]model.Notification, error) {
	query := s.store.DB.Where("recipient_id = ?", recipientID)
	if unreadOnly {
		query = query.Where("read_at IS NULL")
	}
	var items []model.Notification
	return items, query.Order("created_at DESC").Limit(100).Find(&items).Error
}

func (s *Service) NotificationSummary(actor *model.Employee, canHR bool) (NotificationSummary, error) {
	if actor == nil {
		return NotificationSummary{}, ErrUnauthorized
	}
	result := NotificationSummary{}
	if err := s.store.DB.Model(&model.Notification{}).Where("recipient_id = ? AND read_at IS NULL", actor.PublicID).Count(&result.Unread).Error; err != nil {
		return result, err
	}
	pending := s.store.DB.Model(&model.DepartureRequest{})
	if canHR {
		pending = pending.Where("status = ? OR (status = ? AND department_leader_id = ?)", model.DeparturePendingHR, model.DeparturePendingManager, actor.PublicID)
	} else {
		pending = pending.Where("status = ? AND department_leader_id = ?", model.DeparturePendingManager, actor.PublicID)
	}
	if err := pending.Count(&result.PendingTasks).Error; err != nil {
		return result, err
	}
	result.Total = result.Unread + result.PendingTasks
	return result, nil
}

func (s *Service) MarkNotificationRead(recipientID, id string) error {
	now := time.Now().UTC()
	result := s.store.DB.Model(&model.Notification{}).Where("id = ? AND recipient_id = ?", id, recipientID).Update("read_at", now)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) MarkAllNotificationsRead(recipientID string) error {
	return s.store.DB.Model(&model.Notification{}).Where("recipient_id = ? AND read_at IS NULL", recipientID).Update("read_at", time.Now().UTC()).Error
}

func createNotification(tx *gorm.DB, recipientID, kind, title, content, resourceType, resourceID string) error {
	id, err := randomToken("ntf_", 18)
	if err != nil {
		return err
	}
	return tx.Create(&model.Notification{ID: id, RecipientID: recipientID, Type: kind, Title: title, Content: content, ResourceType: resourceType, ResourceID: resourceID}).Error
}
