package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"people/internal/model"

	"gorm.io/gorm"
)

type ProfileInput struct {
	Email                    string `json:"email"`
	Phone                    string `json:"phone"`
	EmergencyContactName     string `json:"emergencyContactName"`
	EmergencyContactPhone    string `json:"emergencyContactPhone"`
	EmergencyContactRelation string `json:"emergencyContactRelation"`
}

type ContractInput struct {
	Type         string `json:"type"`
	StartDate    string `json:"startDate"`
	EndDate      string `json:"endDate"`
	Status       string `json:"status"`
	DocumentName string `json:"documentName"`
	Note         string `json:"note"`
}

type GoalInput struct {
	Cycle          string `json:"cycle"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	DueDate        string `json:"dueDate"`
	Weight         int    `json:"weight"`
	Progress       int    `json:"progress"`
	Status         string `json:"status"`
	ManagerComment string `json:"managerComment"`
}

func (s *Service) UpdateMyProfile(actor *model.Employee, input ProfileInput) (*model.Employee, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Phone = strings.TrimSpace(input.Phone)
	input.EmergencyContactName = strings.TrimSpace(input.EmergencyContactName)
	input.EmergencyContactPhone = strings.TrimSpace(input.EmergencyContactPhone)
	input.EmergencyContactRelation = strings.TrimSpace(input.EmergencyContactRelation)
	if len(input.Email) > 255 || len(input.Phone) > 32 || len([]rune(input.EmergencyContactName)) > 100 || len(input.EmergencyContactPhone) > 32 || len([]rune(input.EmergencyContactRelation)) > 50 {
		return nil, fmt.Errorf("%w: 联系方式字段过长", ErrInvalid)
	}
	if input.Email != "" && (!strings.Contains(input.Email, "@") || strings.HasPrefix(input.Email, "@") || strings.HasSuffix(input.Email, "@")) {
		return nil, fmt.Errorf("%w: 邮箱格式无效", ErrInvalid)
	}
	if err := s.store.DB.Model(&model.Employee{}).Where("id = ?", actor.ID).Updates(map[string]any{
		"email": input.Email, "phone": input.Phone, "emergency_contact_name": input.EmergencyContactName,
		"emergency_contact_phone": input.EmergencyContactPhone, "emergency_contact_relation": input.EmergencyContactRelation,
	}).Error; err != nil {
		return nil, err
	}
	return s.GetEmployee(actor.PublicID)
}

func (s *Service) ListEmploymentEvents(employeePublicID string) ([]model.EmploymentEvent, error) {
	items := make([]model.EmploymentEvent, 0)
	return items, s.store.DB.Where("employee_public_id = ?", strings.TrimSpace(employeePublicID)).Order("effective_date DESC, created_at DESC").Find(&items).Error
}

func createEmploymentEvent(tx *gorm.DB, employee model.Employee, eventType, effectiveDate, fromDepartmentID, fromDepartment, toDepartmentID, toDepartment, fromTitle, toTitle, note, approvalID string) error {
	if effectiveDate == "" {
		effectiveDate = time.Now().UTC().Format("2006-01-02")
	}
	id, err := randomToken("evt_", 18)
	if err != nil {
		return err
	}
	event := model.EmploymentEvent{
		ID: id, EmployeeID: employee.ID, EmployeePublicID: employee.PublicID, Type: eventType, EffectiveDate: effectiveDate,
		FromDepartmentID: fromDepartmentID, FromDepartment: fromDepartment, ToDepartmentID: toDepartmentID, ToDepartment: toDepartment,
		FromTitle: fromTitle, ToTitle: toTitle, Note: strings.TrimSpace(note), ApprovalID: approvalID,
	}
	return tx.Create(&event).Error
}

func (s *Service) CreateContract(employeePublicID string, input ContractInput) (*model.EmployeeContract, error) {
	var employee model.Employee
	if err := s.store.DB.Where("public_id = ?", strings.TrimSpace(employeePublicID)).First(&employee).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	normalized, err := normalizeContract(input)
	if err != nil {
		return nil, err
	}
	if normalized.Status == "active" {
		var count int64
		if err := s.store.DB.Model(&model.EmployeeContract{}).Where("employee_id = ? AND status = ?", employee.ID, "active").Count(&count).Error; err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, fmt.Errorf("%w: 员工已存在生效中的合同", ErrConflict)
		}
	}
	id, err := randomToken("ctr_", 18)
	if err != nil {
		return nil, err
	}
	contract := model.EmployeeContract{ID: id, EmployeeID: employee.ID, EmployeePublicID: employee.PublicID, EmployeeName: employee.DisplayName, Type: normalized.Type, StartDate: normalized.StartDate, EndDate: normalized.EndDate, Status: normalized.Status, DocumentName: normalized.DocumentName, Note: normalized.Note}
	return &contract, s.store.DB.Create(&contract).Error
}

func (s *Service) UpdateContract(id string, input ContractInput) (*model.EmployeeContract, error) {
	var contract model.EmployeeContract
	if err := s.store.DB.Where("id = ?", strings.TrimSpace(id)).First(&contract).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	normalized, err := normalizeContract(input)
	if err != nil {
		return nil, err
	}
	if normalized.Status == "active" {
		var count int64
		if err := s.store.DB.Model(&model.EmployeeContract{}).Where("employee_id = ? AND status = ? AND id <> ?", contract.EmployeeID, "active", contract.ID).Count(&count).Error; err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, fmt.Errorf("%w: 员工已存在其他生效合同", ErrConflict)
		}
	}
	if err := s.store.DB.Model(&contract).Updates(map[string]any{"type": normalized.Type, "start_date": normalized.StartDate, "end_date": normalized.EndDate, "status": normalized.Status, "document_name": normalized.DocumentName, "note": normalized.Note}).Error; err != nil {
		return nil, err
	}
	return &contract, s.store.DB.Where("id = ?", contract.ID).First(&contract).Error
}

func (s *Service) DeleteContract(id string) error {
	result := s.store.DB.Where("id = ?", strings.TrimSpace(id)).Delete(&model.EmployeeContract{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) ListContracts(actor *model.Employee, canManage bool, employeePublicID string) ([]model.EmployeeContract, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	query := s.store.DB.Model(&model.EmployeeContract{})
	if canManage {
		if employeePublicID = strings.TrimSpace(employeePublicID); employeePublicID != "" {
			query = query.Where("employee_public_id = ?", employeePublicID)
		}
	} else {
		query = query.Where("employee_public_id = ?", actor.PublicID)
	}
	items := make([]model.EmployeeContract, 0)
	return items, query.Order("CASE WHEN status = 'active' THEN 0 ELSE 1 END, end_date ASC, created_at DESC").Find(&items).Error
}

func normalizeContract(input ContractInput) (ContractInput, error) {
	input.Type = strings.ToLower(strings.TrimSpace(input.Type))
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	input.StartDate = strings.TrimSpace(input.StartDate)
	input.EndDate = strings.TrimSpace(input.EndDate)
	input.DocumentName = strings.TrimSpace(input.DocumentName)
	input.Note = strings.TrimSpace(input.Note)
	if !contains([]string{"fixed_term", "open_ended", "internship", "service"}, input.Type) || !contains([]string{"active", "ended", "terminated"}, input.Status) {
		return input, fmt.Errorf("%w: 合同类型或状态无效", ErrInvalid)
	}
	start, err := parseDate(input.StartDate)
	if err != nil {
		return input, fmt.Errorf("%w: 合同开始日期无效", ErrInvalid)
	}
	if input.Type != "open_ended" || input.EndDate != "" {
		end, err := parseDate(input.EndDate)
		if err != nil || end.Before(start) {
			return input, fmt.Errorf("%w: 合同结束日期无效", ErrInvalid)
		}
	}
	if len([]rune(input.DocumentName)) > 255 || len([]rune(input.Note)) > 1000 {
		return input, fmt.Errorf("%w: 合同说明过长", ErrInvalid)
	}
	return input, nil
}

func (s *Service) CreateGoal(actor *model.Employee, input GoalInput) (*model.PerformanceGoal, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	normalized, err := normalizeGoal(input, true)
	if err != nil {
		return nil, err
	}
	id, err := randomToken("gol_", 18)
	if err != nil {
		return nil, err
	}
	goal := model.PerformanceGoal{ID: id, EmployeeID: actor.ID, EmployeePublicID: actor.PublicID, EmployeeName: actor.DisplayName, DepartmentID: actor.DepartmentID, Cycle: normalized.Cycle, Title: normalized.Title, Description: normalized.Description, DueDate: normalized.DueDate, Weight: normalized.Weight, Progress: normalized.Progress, Status: normalized.Status}
	if err := s.store.DB.Create(&goal).Error; err != nil {
		return nil, err
	}
	goal.CanEdit = true
	return &goal, nil
}

func (s *Service) UpdateGoal(actor *model.Employee, id string, input GoalInput, canManage bool) (*model.PerformanceGoal, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	var goal model.PerformanceGoal
	if err := s.store.DB.Where("id = ?", strings.TrimSpace(id)).First(&goal).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	isOwner := goal.EmployeePublicID == actor.PublicID
	isManager, err := s.isDepartmentLeader(actor.PublicID, goal.DepartmentID)
	if err != nil {
		return nil, err
	}
	if !isOwner && !isManager && !canManage {
		return nil, ErrForbidden
	}
	normalized, err := normalizeGoal(input, false)
	if err != nil {
		return nil, err
	}
	updates := map[string]any{}
	if isOwner || canManage {
		updates = map[string]any{"cycle": normalized.Cycle, "title": normalized.Title, "description": normalized.Description, "due_date": normalized.DueDate, "weight": normalized.Weight, "progress": normalized.Progress, "status": normalized.Status}
	}
	if !isOwner && (isManager || canManage) {
		updates["manager_comment"] = normalized.ManagerComment
	}
	if err := s.store.DB.Model(&goal).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.goalByID(goal.ID, actor, canManage)
}

func (s *Service) ListGoals(actor *model.Employee, canManage bool, cycle string) ([]model.PerformanceGoal, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	query := s.store.DB.Model(&model.PerformanceGoal{})
	if !canManage {
		var departments []string
		if err := s.store.DB.Model(&model.Department{}).Where("leader_id = ?", actor.PublicID).Pluck("id", &departments).Error; err != nil {
			return nil, err
		}
		if len(departments) > 0 {
			query = query.Where("employee_public_id = ? OR department_id IN ?", actor.PublicID, departments)
		} else {
			query = query.Where("employee_public_id = ?", actor.PublicID)
		}
	}
	if cycle = strings.TrimSpace(cycle); cycle != "" {
		query = query.Where("cycle = ?", cycle)
	}
	items := make([]model.PerformanceGoal, 0)
	if err := query.Order("due_date ASC, created_at DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	for index := range items {
		items[index].CanEdit = items[index].EmployeePublicID == actor.PublicID || canManage
		items[index].CanReview = items[index].EmployeePublicID != actor.PublicID && (canManage || s.isDepartmentLeaderFast(actor.PublicID, items[index].DepartmentID))
	}
	return items, nil
}

func (s *Service) goalByID(id string, actor *model.Employee, canManage bool) (*model.PerformanceGoal, error) {
	var goal model.PerformanceGoal
	if err := s.store.DB.Where("id = ?", id).First(&goal).Error; err != nil {
		return nil, err
	}
	goal.CanEdit = goal.EmployeePublicID == actor.PublicID || canManage
	goal.CanReview = goal.EmployeePublicID != actor.PublicID && (canManage || s.isDepartmentLeaderFast(actor.PublicID, goal.DepartmentID))
	return &goal, nil
}

func normalizeGoal(input GoalInput, creating bool) (GoalInput, error) {
	input.Cycle = strings.TrimSpace(input.Cycle)
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)
	input.DueDate = strings.TrimSpace(input.DueDate)
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	input.ManagerComment = strings.TrimSpace(input.ManagerComment)
	if creating && input.Status == "" {
		input.Status = "active"
	}
	if input.Cycle == "" || len([]rune(input.Cycle)) > 32 || input.Title == "" || len([]rune(input.Title)) > 160 || len([]rune(input.Description)) > 1000 || len([]rune(input.ManagerComment)) > 1000 {
		return input, fmt.Errorf("%w: 目标内容无效", ErrInvalid)
	}
	if _, err := parseDate(input.DueDate); err != nil || input.Weight < 1 || input.Weight > 100 || input.Progress < 0 || input.Progress > 100 || !contains([]string{"draft", "active", "completed", "cancelled"}, input.Status) {
		return input, fmt.Errorf("%w: 目标日期、权重、进度或状态无效", ErrInvalid)
	}
	if input.Progress == 100 {
		input.Status = "completed"
	}
	return input, nil
}

func (s *Service) isDepartmentLeader(publicID, departmentID string) (bool, error) {
	var count int64
	err := s.store.DB.Model(&model.Department{}).Where("id = ? AND leader_id = ?", departmentID, publicID).Count(&count).Error
	return count > 0, err
}

func (s *Service) isDepartmentLeaderFast(publicID, departmentID string) bool {
	value, err := s.isDepartmentLeader(publicID, departmentID)
	return err == nil && value
}
