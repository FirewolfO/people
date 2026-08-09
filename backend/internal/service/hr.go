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
	TotalEmployees             int64          `json:"totalEmployees"`
	EnabledEmployees           int64          `json:"enabledEmployees"`
	DisabledEmployees          int64          `json:"disabledEmployees"`
	Departments                int64          `json:"departments"`
	PendingDepartures          int64          `json:"pendingDepartures"`
	PendingApprovals           int64          `json:"pendingApprovals"`
	ProbationEmployees         int64          `json:"probationEmployees"`
	RecentHires                int64          `json:"recentHires"`
	EmployeesOnLeave           int64          `json:"employeesOnLeave"`
	ContractsExpiring          int64          `json:"contractsExpiring"`
	ActiveGoals                int64          `json:"activeGoals"`
	OverdueGoals               int64          `json:"overdueGoals"`
	DepartmentDistribution     []MetricBucket `json:"departmentDistribution"`
	EmploymentTypeDistribution []MetricBucket `json:"employmentTypeDistribution"`
	ApprovalDistribution       []MetricBucket `json:"approvalDistribution"`
}

type MetricBucket struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
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
		eventType, note := model.EmploymentEventDisable, "HR 停用员工账号"
		if enabled {
			eventType, note = model.EmploymentEventEnable, "HR 恢复员工账号"
		}
		if err := createEmploymentEvent(tx, employee, eventType, now.Format("2006-01-02"), employee.DepartmentID, employee.Department, employee.DepartmentID, employee.Department, employee.Title, employee.Title, note, ""); err != nil {
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
	result := HRDashboard{
		DepartmentDistribution:     []MetricBucket{},
		EmploymentTypeDistribution: []MetricBucket{},
		ApprovalDistribution:       []MetricBucket{},
	}
	today := time.Now().UTC().Format("2006-01-02")
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
		{&model.ApprovalRequest{}, "type = ? AND status = ?", []any{model.ApprovalTypeDeparture, model.ApprovalPending}, &result.PendingDepartures},
		{&model.ApprovalRequest{}, "status = ?", []any{model.ApprovalPending}, &result.PendingApprovals},
		{&model.Employee{}, "status = ? AND probation_end_date >= ?", []any{model.StatusEnabled, today}, &result.ProbationEmployees},
		{&model.Employee{}, "hire_date >= ?", []any{time.Now().UTC().AddDate(0, 0, -30).Format("2006-01-02")}, &result.RecentHires},
		{&model.LeaveRecord{}, "status = ? AND start_date <= ? AND end_date >= ?", []any{model.ApprovalApproved, today, today}, &result.EmployeesOnLeave},
		{&model.EmployeeContract{}, "status = ? AND end_date <> '' AND end_date BETWEEN ? AND ?", []any{"active", today, time.Now().UTC().AddDate(0, 2, 0).Format("2006-01-02")}, &result.ContractsExpiring},
		{&model.PerformanceGoal{}, "status = ?", []any{"active"}, &result.ActiveGoals},
		{&model.PerformanceGoal{}, "status = ? AND progress < 100 AND due_date < ?", []any{"active", today}, &result.OverdueGoals},
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
	groupQueries := []struct {
		query *gorm.DB
		value *[]MetricBucket
	}{
		{s.store.DB.Model(&model.Employee{}).Select("COALESCE(NULLIF(department, ''), '未分配') AS name, COUNT(*) AS count").Where("status = ?", model.StatusEnabled).Group("department").Order("count DESC"), &result.DepartmentDistribution},
		{s.store.DB.Model(&model.Employee{}).Select("employment_type AS name, COUNT(*) AS count").Where("status = ?", model.StatusEnabled).Group("employment_type").Order("count DESC"), &result.EmploymentTypeDistribution},
		{s.store.DB.Model(&model.ApprovalRequest{}).Select("type AS name, COUNT(*) AS count").Where("status = ?", model.ApprovalPending).Group("type").Order("count DESC"), &result.ApprovalDistribution},
	}
	for _, item := range groupQueries {
		if err := item.query.Scan(item.value).Error; err != nil {
			return HRDashboard{}, err
		}
	}
	return result, nil
}

func (s *Service) CreateDeparture(employee *model.Employee, input DepartureInput) (*model.DepartureRequest, error) {
	request, err := s.CreateApproval(employee, ApprovalInput{Type: model.ApprovalTypeDeparture, Data: map[string]any{"reason": input.Reason, "lastWorkingDate": input.LastWorkingDate}})
	if err != nil {
		return nil, err
	}
	return approvalToDeparture(request), nil
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
	requests, err := s.ListApprovals(actor, ApprovalFilter{Type: model.ApprovalTypeDeparture}, canHR, canHR)
	if err != nil {
		return nil, err
	}
	items := make([]model.DepartureRequest, 0, len(requests))
	for index := range requests {
		items = append(items, *approvalToDeparture(&requests[index]))
	}
	return items, nil
}

func (s *Service) ReviewDeparture(actor *model.Employee, id, stage string, input ReviewInput, canHR bool) (*model.DepartureRequest, error) {
	request, err := s.approvalByID(id, actor, canHR)
	if err != nil {
		return nil, err
	}
	if request.Type != model.ApprovalTypeDeparture || (stage == "manager" && request.CurrentStep != 1) || (stage == "hr" && request.CurrentStep != 2) || (stage != "manager" && stage != "hr") {
		return nil, fmt.Errorf("%w: 审批阶段无效", ErrConflict)
	}
	request, err = s.ReviewApproval(actor, id, input, canHR)
	if err != nil {
		return nil, err
	}
	return approvalToDeparture(request), nil
}

func (s *Service) CancelDeparture(actor *model.Employee, id string) error {
	return s.CancelApproval(actor, id)
}

func approvalToDeparture(request *model.ApprovalRequest) *model.DepartureRequest {
	status := model.DeparturePendingManager
	if request.Status == model.ApprovalApproved {
		status = model.DepartureApproved
	} else if request.Status == model.ApprovalRejected {
		status = model.DepartureRejected
	} else if request.Status == model.ApprovalCancelled {
		status = model.DepartureCancelled
	} else if request.CurrentStep > 1 {
		status = model.DeparturePendingHR
	}
	result := &model.DepartureRequest{
		ID: request.ID, EmployeeID: request.ApplicantID, EmployeePublicID: request.ApplicantPublicID,
		EmployeeName: request.ApplicantName, EmployeeNo: request.ApplicantNo, DepartmentID: request.DepartmentID,
		DepartmentName: request.DepartmentName, Reason: dataString(request.Data, "reason"), LastWorkingDate: dataString(request.Data, "lastWorkingDate"),
		Status: status, CanManagerReview: request.CanReview && request.CurrentStep == 1,
		CanHRReview: request.CanReview && request.CurrentStep == 2, CanCancel: request.CanCancel,
		CreatedAt: request.CreatedAt, UpdatedAt: request.UpdatedAt,
	}
	for _, step := range request.Steps {
		if step.Sequence == 1 {
			result.DepartmentLeaderID = step.ApproverID
			result.ManagerReviewerID, result.ManagerReviewerName = step.ReviewerID, step.ReviewerName
			result.ManagerReviewComment, result.ManagerReviewedAt = step.Comment, step.ReviewedAt
		} else if step.Sequence == 2 {
			result.HRReviewerID, result.HRReviewerName = step.ReviewerID, step.ReviewerName
			result.HRReviewComment, result.HRReviewedAt = step.Comment, step.ReviewedAt
		}
	}
	return result
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
	pending := s.store.DB.Model(&model.ApprovalStep{}).
		Joins("JOIN approval_requests ON approval_requests.id = approval_steps.approval_id").
		Where("approval_steps.status = ? AND approval_requests.status = ?", model.ApprovalStepPending, model.ApprovalPending)
	if canHR {
		pending = pending.Where("approval_steps.approver_id = ? OR (approval_steps.permission_code = ? AND approval_requests.applicant_public_id <> ?)", actor.PublicID, PermissionApprovalReview, actor.PublicID)
	} else {
		pending = pending.Where("approval_steps.approver_id = ?", actor.PublicID)
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
