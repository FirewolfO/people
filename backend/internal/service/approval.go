package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"people/internal/model"

	"gorm.io/gorm"
)

type ApprovalInput struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data"`
}

type ApprovalFilter struct {
	Scope  string
	Type   string
	Status string
}

type ApprovalTypeDefinition struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Steps       []string `json:"steps"`
}

var approvalTypes = []ApprovalTypeDefinition{
	{Code: model.ApprovalTypeLeave, Name: "请假", Description: "年假、病假、事假等休假申请", Steps: []string{"部门负责人审批"}},
	{Code: model.ApprovalTypeTransfer, Name: "岗位异动", Description: "部门或职务调整申请", Steps: []string{"部门负责人审批", "HR 审批"}},
	{Code: model.ApprovalTypeDeparture, Name: "离职", Description: "员工离职与账号停用流程", Steps: []string{"部门负责人审批", "HR 审批"}},
}

func ApprovalTypes() []ApprovalTypeDefinition {
	return append([]ApprovalTypeDefinition(nil), approvalTypes...)
}

func (s *Service) CreateApproval(actor *model.Employee, input ApprovalInput) (*model.ApprovalRequest, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	if actor.Role == model.RoleAdmin || actor.Status != model.StatusEnabled {
		return nil, fmt.Errorf("%w: 管理员或停用账号不能发起员工审批", ErrForbidden)
	}
	input.Type = strings.ToLower(strings.TrimSpace(input.Type))
	var department model.Department
	if actor.DepartmentID == "" || s.store.DB.Where("id = ? AND status = ?", actor.DepartmentID, model.StatusEnabled).First(&department).Error != nil {
		return nil, fmt.Errorf("%w: 所在部门不存在或已停用", ErrInvalid)
	}
	approverID, err := s.departureApprover(actor, department)
	if err != nil {
		return nil, err
	}
	title, summary, normalized, steps, leave, err := s.prepareApproval(actor, department, approverID, input)
	if err != nil {
		return nil, err
	}
	var active int64
	if err := s.store.DB.Model(&model.ApprovalRequest{}).Where("applicant_id = ? AND type = ? AND status = ?", actor.ID, input.Type, model.ApprovalPending).Count(&active).Error; err != nil {
		return nil, err
	}
	if active > 0 {
		return nil, fmt.Errorf("%w: 已存在同类型的待审批申请", ErrConflict)
	}
	id, err := randomToken("apr_", 18)
	if err != nil {
		return nil, err
	}
	dataJSON, err := json.Marshal(normalized)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	request := model.ApprovalRequest{
		ID: id, Type: input.Type, Title: title, Summary: summary,
		ApplicantID: actor.ID, ApplicantPublicID: actor.PublicID, ApplicantName: actor.DisplayName, ApplicantNo: actor.EmployeeNo,
		DepartmentID: actor.DepartmentID, DepartmentName: actor.Department, DataJSON: string(dataJSON), Data: normalized,
		Status: model.ApprovalPending, CurrentStep: 1, TotalSteps: len(steps), SubmittedAt: now,
	}
	for index := range steps {
		steps[index].ApprovalID = id
		steps[index].Sequence = index + 1
		steps[index].Status = model.ApprovalStepWaiting
	}
	steps[0].Status = model.ApprovalStepPending
	err = s.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&request).Error; err != nil {
			return err
		}
		if err := tx.Create(&steps).Error; err != nil {
			return err
		}
		if leave != nil {
			leave.ID = "lev_" + strings.TrimPrefix(id, "apr_")
			leave.ApprovalID = id
			leave.Status = model.ApprovalPending
			return tx.Create(leave).Error
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.approvalByID(id, actor, false)
}

func (s *Service) prepareApproval(actor *model.Employee, department model.Department, approverID string, input ApprovalInput) (string, string, map[string]any, []model.ApprovalStep, *model.LeaveRecord, error) {
	managerStep := model.ApprovalStep{Name: "部门负责人审批", ApproverID: approverID}
	hrStep := model.ApprovalStep{Name: "HR 审批", PermissionCode: PermissionApprovalReview}
	switch input.Type {
	case model.ApprovalTypeDeparture:
		reason := dataString(input.Data, "reason")
		lastWorkingDate := dataString(input.Data, "lastWorkingDate")
		date, err := parseDate(lastWorkingDate)
		if reason == "" || len([]rune(reason)) > 1000 || err != nil || date.Before(todayUTC()) {
			return "", "", nil, nil, nil, fmt.Errorf("%w: 离职原因和有效的最后工作日不能为空", ErrInvalid)
		}
		data := map[string]any{"reason": reason, "lastWorkingDate": lastWorkingDate}
		return "离职申请", actor.DisplayName + " 计划于 " + lastWorkingDate + " 离职", data, []model.ApprovalStep{managerStep, hrStep}, nil, nil
	case model.ApprovalTypeLeave:
		leaveType := strings.ToLower(dataString(input.Data, "leaveType"))
		if !contains([]string{"annual", "sick", "personal", "other"}, leaveType) {
			return "", "", nil, nil, nil, fmt.Errorf("%w: 请假类型无效", ErrInvalid)
		}
		startDate, endDate := dataString(input.Data, "startDate"), dataString(input.Data, "endDate")
		start, startErr := parseDate(startDate)
		end, endErr := parseDate(endDate)
		if startErr != nil || endErr != nil || start.Before(todayUTC()) || end.Before(start) || start.Year() != end.Year() {
			return "", "", nil, nil, nil, fmt.Errorf("%w: 请假日期无效或不能跨年", ErrInvalid)
		}
		days := businessDays(start, end)
		if days <= 0 || days > 60 {
			return "", "", nil, nil, nil, fmt.Errorf("%w: 请假必须包含工作日且不能超过 60 天", ErrInvalid)
		}
		if leaveType == "annual" {
			balance, err := s.GetLeaveBalance(actor, start.Year())
			if err != nil {
				return "", "", nil, nil, nil, err
			}
			if days > balance.AnnualRemaining {
				return "", "", nil, nil, nil, fmt.Errorf("%w: 年假余额不足", ErrConflict)
			}
		}
		reason := dataString(input.Data, "reason")
		if len([]rune(reason)) > 1000 {
			return "", "", nil, nil, nil, fmt.Errorf("%w: 请假事由过长", ErrInvalid)
		}
		data := map[string]any{"leaveType": leaveType, "startDate": startDate, "endDate": endDate, "days": days, "reason": reason}
		leave := &model.LeaveRecord{EmployeeID: actor.ID, EmployeePublicID: actor.PublicID, EmployeeName: actor.DisplayName, DepartmentID: department.ID, DepartmentName: department.Name, LeaveType: leaveType, StartDate: startDate, EndDate: endDate, Days: days, Reason: reason}
		return "请假申请", fmt.Sprintf("%s 申请 %s 至 %s 请假 %.0f 天", actor.DisplayName, startDate, endDate, days), data, []model.ApprovalStep{managerStep}, leave, nil
	case model.ApprovalTypeTransfer:
		targetDepartmentID := dataString(input.Data, "targetDepartmentId")
		targetTitle := dataString(input.Data, "targetTitle")
		effectiveDate := dataString(input.Data, "effectiveDate")
		reason := dataString(input.Data, "reason")
		if len([]rune(reason)) > 1000 {
			return "", "", nil, nil, nil, fmt.Errorf("%w: 异动原因过长", ErrInvalid)
		}
		date, dateErr := parseDate(effectiveDate)
		var target model.Department
		if targetDepartmentID == "" || targetTitle == "" || len([]rune(targetTitle)) > 100 || dateErr != nil || date.Before(todayUTC()) || s.store.DB.Where("id = ? AND status = ?", targetDepartmentID, model.StatusEnabled).First(&target).Error != nil {
			return "", "", nil, nil, nil, fmt.Errorf("%w: 目标部门、职务或生效日期无效", ErrInvalid)
		}
		if target.ID == actor.DepartmentID && targetTitle == actor.Title {
			return "", "", nil, nil, nil, fmt.Errorf("%w: 部门和职务均未发生变化", ErrInvalid)
		}
		data := map[string]any{"targetDepartmentId": target.ID, "targetDepartmentName": target.Name, "targetTitle": targetTitle, "effectiveDate": effectiveDate, "reason": reason}
		return "岗位异动申请", fmt.Sprintf("%s 申请调整至 %s / %s", actor.DisplayName, target.Name, targetTitle), data, []model.ApprovalStep{managerStep, hrStep}, nil, nil
	default:
		return "", "", nil, nil, nil, fmt.Errorf("%w: 不支持的审批类型", ErrInvalid)
	}
}

func (s *Service) ReviewApproval(actor *model.Employee, id string, input ReviewInput, canReviewByPermission bool) (*model.ApprovalRequest, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	comment := strings.TrimSpace(input.Comment)
	if len([]rune(comment)) > 500 {
		return nil, fmt.Errorf("%w: 审批意见过长", ErrInvalid)
	}
	err := s.store.DB.Transaction(func(tx *gorm.DB) error {
		var request model.ApprovalRequest
		if err := tx.Where("id = ?", strings.TrimSpace(id)).First(&request).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if request.Status != model.ApprovalPending {
			return fmt.Errorf("%w: 当前审批已结束", ErrConflict)
		}
		var step model.ApprovalStep
		if err := tx.Where("approval_id = ? AND sequence = ? AND status = ?", request.ID, request.CurrentStep, model.ApprovalStepPending).First(&step).Error; err != nil {
			return fmt.Errorf("%w: 当前审批步骤不存在", ErrConflict)
		}
		allowed := step.ApproverID != "" && step.ApproverID == actor.PublicID
		if step.PermissionCode != "" && canReviewByPermission {
			allowed = true
		}
		if !allowed || request.ApplicantPublicID == actor.PublicID {
			return ErrForbidden
		}
		now := time.Now().UTC()
		stepStatus := model.ApprovalStepRejected
		if input.Approved {
			stepStatus = model.ApprovalStepApproved
		}
		updated := tx.Model(&model.ApprovalStep{}).Where("id = ? AND status = ?", step.ID, model.ApprovalStepPending).Updates(map[string]any{
			"status": stepStatus, "reviewer_id": actor.PublicID, "reviewer_name": actor.DisplayName, "comment": comment, "reviewed_at": now,
		})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return fmt.Errorf("%w: 审批已被其他人处理", ErrConflict)
		}
		if !input.Approved {
			if err := tx.Model(&request).Updates(map[string]any{"status": model.ApprovalRejected, "completed_at": now}).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.ApprovalStep{}).Where("approval_id = ? AND status = ?", request.ID, model.ApprovalStepWaiting).Update("status", model.ApprovalStepSkipped).Error; err != nil {
				return err
			}
			if request.Type == model.ApprovalTypeLeave {
				if err := tx.Model(&model.LeaveRecord{}).Where("approval_id = ?", request.ID).Update("status", model.ApprovalRejected).Error; err != nil {
					return err
				}
			}
			return createNotification(tx, request.ApplicantPublicID, "approval_result", request.Title+"已驳回", actor.DisplayName+"驳回了你的申请", "approval", request.ID)
		}
		if request.CurrentStep < request.TotalSteps {
			next := request.CurrentStep + 1
			if err := tx.Model(&model.ApprovalStep{}).Where("approval_id = ? AND sequence = ? AND status = ?", request.ID, next, model.ApprovalStepWaiting).Update("status", model.ApprovalStepPending).Error; err != nil {
				return err
			}
			return tx.Model(&request).Update("current_step", next).Error
		}
		if err := s.completeApproval(tx, &request, now); err != nil {
			return err
		}
		if err := tx.Model(&request).Updates(map[string]any{"status": model.ApprovalApproved, "completed_at": now}).Error; err != nil {
			return err
		}
		return createNotification(tx, request.ApplicantPublicID, "approval_result", request.Title+"已通过", "你的申请已完成全部审批", "approval", request.ID)
	})
	if err != nil {
		return nil, err
	}
	return s.approvalByID(id, actor, canReviewByPermission)
}

func (s *Service) completeApproval(tx *gorm.DB, request *model.ApprovalRequest, now time.Time) error {
	var data map[string]any
	if err := json.Unmarshal([]byte(request.DataJSON), &data); err != nil {
		return err
	}
	var employee model.Employee
	if err := tx.Where("id = ?", request.ApplicantID).First(&employee).Error; err != nil {
		return err
	}
	switch request.Type {
	case model.ApprovalTypeDeparture:
		var leaderCount int64
		if err := tx.Model(&model.Department{}).Where("leader_id = ?", employee.PublicID).Count(&leaderCount).Error; err != nil {
			return err
		}
		if leaderCount > 0 {
			return fmt.Errorf("%w: 请先为员工负责的部门更换负责人", ErrConflict)
		}
		if err := tx.Model(&employee).Update("status", model.StatusDisabled).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Session{}).Where("employee_id = ? AND revoked_at IS NULL", employee.ID).Update("revoked_at", now).Error; err != nil {
			return err
		}
		if err := tx.Where("employee_id = ?", employee.ID).Delete(&model.OAuthToken{}).Error; err != nil {
			return err
		}
		return createEmploymentEvent(tx, employee, model.EmploymentEventDeparture, dataString(data, "lastWorkingDate"), employee.DepartmentID, employee.Department, "", "", employee.Title, "", dataString(data, "reason"), request.ID)
	case model.ApprovalTypeLeave:
		var leave model.LeaveRecord
		if err := tx.Where("approval_id = ?", request.ID).First(&leave).Error; err != nil {
			return err
		}
		if err := tx.Model(&leave).Update("status", model.ApprovalApproved).Error; err != nil {
			return err
		}
		year, _ := time.Parse("2006-01-02", leave.StartDate)
		balance := model.LeaveBalance{EmployeeID: employee.ID, EmployeePublicID: employee.PublicID, Year: year.Year(), AnnualEntitlement: 10}
		if err := tx.Where("employee_id = ? AND year = ?", employee.ID, year.Year()).FirstOrCreate(&balance).Error; err != nil {
			return err
		}
		column := "personal_used"
		if leave.LeaveType == "annual" {
			column = "annual_used"
		} else if leave.LeaveType == "sick" {
			column = "sick_used"
		}
		return tx.Model(&balance).UpdateColumn(column, gorm.Expr(column+" + ?", leave.Days)).Error
	case model.ApprovalTypeTransfer:
		targetID, targetTitle := dataString(data, "targetDepartmentId"), dataString(data, "targetTitle")
		var target model.Department
		if err := tx.Where("id = ? AND status = ?", targetID, model.StatusEnabled).First(&target).Error; err != nil {
			return fmt.Errorf("%w: 目标部门已不可用", ErrConflict)
		}
		if employee.DepartmentID != target.ID {
			var leaderCount int64
			if err := tx.Model(&model.Department{}).Where("leader_id = ?", employee.PublicID).Count(&leaderCount).Error; err != nil {
				return err
			}
			if leaderCount > 0 {
				return fmt.Errorf("%w: 请先为员工负责的部门更换负责人", ErrConflict)
			}
		}
		eventType := model.EmploymentEventTransfer
		if employee.DepartmentID == target.ID {
			eventType = model.EmploymentEventPromotion
		}
		if err := createEmploymentEvent(tx, employee, eventType, dataString(data, "effectiveDate"), employee.DepartmentID, employee.Department, target.ID, target.Name, employee.Title, targetTitle, dataString(data, "reason"), request.ID); err != nil {
			return err
		}
		return tx.Model(&employee).Updates(map[string]any{"department_id": target.ID, "department": target.Name, "title": targetTitle}).Error
	default:
		return fmt.Errorf("%w: 不支持的审批类型", ErrInvalid)
	}
}

func (s *Service) ListApprovals(actor *model.Employee, filter ApprovalFilter, canViewAll, canReviewByPermission bool) ([]model.ApprovalRequest, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	query := s.store.DB.Model(&model.ApprovalRequest{}).Distinct("approval_requests.*")
	switch strings.ToLower(strings.TrimSpace(filter.Scope)) {
	case "mine":
		query = query.Where("approval_requests.applicant_public_id = ?", actor.PublicID)
	case "pending":
		query = query.Joins("JOIN approval_steps ON approval_steps.approval_id = approval_requests.id").Where("approval_steps.status = ?", model.ApprovalStepPending)
		if canReviewByPermission {
			query = query.Where("approval_steps.approver_id = ? OR (approval_steps.permission_code = ? AND approval_requests.applicant_public_id <> ?)", actor.PublicID, PermissionApprovalReview, actor.PublicID)
		} else {
			query = query.Where("approval_steps.approver_id = ?", actor.PublicID)
		}
	case "all":
		if !canViewAll {
			return nil, ErrForbidden
		}
	default:
		query = query.Joins("LEFT JOIN approval_steps ON approval_steps.approval_id = approval_requests.id")
		if canViewAll {
			// HR and administrators can see the full workflow register.
		} else if canReviewByPermission {
			query = query.Where("approval_requests.applicant_public_id = ? OR approval_steps.approver_id = ? OR approval_steps.permission_code = ?", actor.PublicID, actor.PublicID, PermissionApprovalReview)
		} else {
			query = query.Where("approval_requests.applicant_public_id = ? OR approval_steps.approver_id = ?", actor.PublicID, actor.PublicID)
		}
	}
	if value := strings.TrimSpace(filter.Type); value != "" {
		query = query.Where("approval_requests.type = ?", value)
	}
	if value := strings.TrimSpace(filter.Status); value != "" {
		query = query.Where("approval_requests.status = ?", value)
	}
	items := make([]model.ApprovalRequest, 0)
	if err := query.Preload("Steps", func(db *gorm.DB) *gorm.DB { return db.Order("sequence ASC") }).Order("approval_requests.created_at DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	for index := range items {
		s.decorateApproval(&items[index], actor, canReviewByPermission)
	}
	return items, nil
}

func (s *Service) approvalByID(id string, actor *model.Employee, canReviewByPermission bool) (*model.ApprovalRequest, error) {
	var request model.ApprovalRequest
	if err := s.store.DB.Preload("Steps", func(db *gorm.DB) *gorm.DB { return db.Order("sequence ASC") }).Where("id = ?", strings.TrimSpace(id)).First(&request).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	s.decorateApproval(&request, actor, canReviewByPermission)
	return &request, nil
}

func (s *Service) GetApproval(actor *model.Employee, id string, canViewAll, canReviewByPermission bool) (*model.ApprovalRequest, error) {
	request, err := s.approvalByID(id, actor, canReviewByPermission)
	if err != nil {
		return nil, err
	}
	if canViewAll || request.ApplicantPublicID == actor.PublicID {
		return request, nil
	}
	for _, step := range request.Steps {
		if step.ApproverID == actor.PublicID || step.ReviewerID == actor.PublicID || (canReviewByPermission && step.PermissionCode == PermissionApprovalReview) {
			return request, nil
		}
	}
	return nil, ErrForbidden
}

func (s *Service) decorateApproval(request *model.ApprovalRequest, actor *model.Employee, canReviewByPermission bool) {
	request.CanCancel = actor != nil && request.ApplicantPublicID == actor.PublicID && request.Status == model.ApprovalPending && request.CurrentStep == 1
	for _, step := range request.Steps {
		if step.Sequence != request.CurrentStep {
			continue
		}
		request.CurrentStepName = step.Name
		request.CanReview = actor != nil && request.ApplicantPublicID != actor.PublicID && step.Status == model.ApprovalStepPending && ((step.ApproverID != "" && step.ApproverID == actor.PublicID) || (step.PermissionCode != "" && canReviewByPermission))
	}
}

func (s *Service) CancelApproval(actor *model.Employee, id string) error {
	if actor == nil {
		return ErrUnauthorized
	}
	now := time.Now().UTC()
	return s.store.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.ApprovalRequest{}).Where("id = ? AND applicant_id = ? AND status = ? AND current_step = 1", id, actor.ID, model.ApprovalPending).Updates(map[string]any{"status": model.ApprovalCancelled, "completed_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("%w: 当前申请不能撤回", ErrConflict)
		}
		if err := tx.Model(&model.ApprovalStep{}).Where("approval_id = ? AND status IN ?", id, []string{model.ApprovalStepPending, model.ApprovalStepWaiting}).Update("status", model.ApprovalStepSkipped).Error; err != nil {
			return err
		}
		return tx.Model(&model.LeaveRecord{}).Where("approval_id = ?", id).Update("status", model.ApprovalCancelled).Error
	})
}

func (s *Service) GetLeaveBalance(actor *model.Employee, year int) (*model.LeaveBalance, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	if year < 2000 || year > 2200 {
		year = time.Now().UTC().Year()
	}
	balance := model.LeaveBalance{EmployeeID: actor.ID, EmployeePublicID: actor.PublicID, Year: year, AnnualEntitlement: 10}
	if err := s.store.DB.Where("employee_id = ? AND year = ?", actor.ID, year).FirstOrCreate(&balance).Error; err != nil {
		return nil, err
	}
	start, end := fmt.Sprintf("%04d-01-01", year), fmt.Sprintf("%04d-12-31", year)
	row := s.store.DB.Model(&model.LeaveRecord{}).Where("employee_id = ? AND leave_type = ? AND status = ? AND start_date BETWEEN ? AND ?", actor.ID, "annual", model.ApprovalPending, start, end).Select("COALESCE(SUM(days), 0)").Row()
	if err := row.Scan(&balance.AnnualPending); err != nil {
		return nil, err
	}
	balance.AnnualRemaining = math.Max(0, balance.AnnualEntitlement-balance.AnnualUsed-balance.AnnualPending)
	return &balance, nil
}

func (s *Service) ListLeaveCalendar(actor *model.Employee, canViewAll bool, month string) ([]model.LeaveRecord, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	query := s.store.DB.Model(&model.LeaveRecord{})
	if month = strings.TrimSpace(month); month != "" {
		if _, err := time.Parse("2006-01", month); err != nil {
			return nil, fmt.Errorf("%w: 月份格式无效", ErrInvalid)
		}
		query = query.Where("start_date <= ? AND end_date >= ?", month+"-31", month+"-01")
	}
	if !canViewAll {
		var leaderCount int64
		if err := s.store.DB.Model(&model.Department{}).Where("id = ? AND leader_id = ?", actor.DepartmentID, actor.PublicID).Count(&leaderCount).Error; err != nil {
			return nil, err
		}
		if leaderCount > 0 {
			query = query.Where("employee_public_id = ? OR department_id = ?", actor.PublicID, actor.DepartmentID)
		} else {
			query = query.Where("employee_public_id = ?", actor.PublicID)
		}
	}
	items := make([]model.LeaveRecord, 0)
	return items, query.Order("start_date ASC, employee_name ASC").Find(&items).Error
}

func businessDays(start, end time.Time) float64 {
	var days float64
	for current := start; !current.After(end); current = current.AddDate(0, 0, 1) {
		if current.Weekday() != time.Saturday && current.Weekday() != time.Sunday {
			days++
		}
	}
	return days
}

func parseDate(value string) (time.Time, error) {
	return time.Parse("2006-01-02", strings.TrimSpace(value))
}

func todayUTC() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

func dataString(data map[string]any, key string) string {
	value, _ := data[key].(string)
	return strings.TrimSpace(value)
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
