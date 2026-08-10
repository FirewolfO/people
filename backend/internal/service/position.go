package service

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"people/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var positionCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

type PositionInput struct {
	Code          string   `json:"code"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	DepartmentIDs []string `json:"departmentIds"`
	Status        string   `json:"status"`
}

func (s *Service) ListPositions(search, departmentID string) ([]model.Position, error) {
	query := s.store.DB.Model(&model.Position{}).Distinct("positions.*")
	if value := strings.TrimSpace(search); value != "" {
		like := "%" + value + "%"
		query = query.Where("positions.code LIKE ? OR positions.name LIKE ?", like, like)
	}
	if departmentID = strings.TrimSpace(departmentID); departmentID != "" {
		query = query.Joins("JOIN department_positions ON department_positions.position_id = positions.id").
			Where("department_positions.department_id = ?", departmentID)
	}
	positions := make([]model.Position, 0)
	if err := query.Order("CASE WHEN positions.status = 'enabled' THEN 0 ELSE 1 END, positions.name ASC").Find(&positions).Error; err != nil {
		return nil, err
	}
	for index := range positions {
		if err := s.decoratePosition(&positions[index]); err != nil {
			return nil, err
		}
	}
	return positions, nil
}

func (s *Service) CreatePosition(input PositionInput) (*model.Position, error) {
	normalized, err := normalizePosition(input)
	if err != nil {
		return nil, err
	}
	departmentIDs, err := s.validatePositionDepartments(normalized.DepartmentIDs)
	if err != nil {
		return nil, err
	}
	if exists, err := s.positionExists(normalized.Code, normalized.Name, ""); err != nil {
		return nil, err
	} else if exists {
		return nil, fmt.Errorf("%w: 岗位编码或名称已存在", ErrConflict)
	}
	id, err := randomToken("pos_", 18)
	if err != nil {
		return nil, err
	}
	position := model.Position{ID: id, Code: normalized.Code, Name: normalized.Name, Description: normalized.Description, Status: normalized.Status}
	if err := s.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&position).Error; err != nil {
			return err
		}
		return replacePositionDepartments(tx, position.ID, departmentIDs)
	}); err != nil {
		return nil, err
	}
	if err := s.decoratePosition(&position); err != nil {
		return nil, err
	}
	return &position, nil
}

func (s *Service) UpdatePosition(id string, input PositionInput) (*model.Position, error) {
	var position model.Position
	if err := s.store.DB.Where("id = ?", strings.TrimSpace(id)).First(&position).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("%w: 岗位不存在", ErrNotFound)
	} else if err != nil {
		return nil, err
	}
	normalized, err := normalizePosition(input)
	if err != nil {
		return nil, err
	}
	if position.Builtin && (normalized.Code != position.Code || normalized.Name != position.Name || normalized.Status != model.StatusEnabled) {
		return nil, fmt.Errorf("%w: 内置岗位的编码、名称和启用状态不能修改", ErrInvalid)
	}
	departmentIDs, err := s.validatePositionDepartments(normalized.DepartmentIDs)
	if err != nil {
		return nil, err
	}
	if exists, err := s.positionExists(normalized.Code, normalized.Name, position.ID); err != nil {
		return nil, err
	} else if exists {
		return nil, fmt.Errorf("%w: 岗位编码或名称已存在", ErrConflict)
	}
	var employeeCount int64
	if err := s.store.DB.Model(&model.Employee{}).Where("position_id = ?", position.ID).Count(&employeeCount).Error; err != nil {
		return nil, err
	}
	if normalized.Status == model.StatusDisabled && employeeCount > 0 {
		return nil, fmt.Errorf("%w: 岗位仍有关联员工，不能停用", ErrConflict)
	}
	if err := s.ensurePositionDepartmentsCoverEmployees(position.ID, departmentIDs); err != nil {
		return nil, err
	}
	oldName := position.Name
	err = s.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&position).Updates(map[string]any{
			"code": normalized.Code, "name": normalized.Name, "description": normalized.Description, "status": normalized.Status,
		}).Error; err != nil {
			return err
		}
		if oldName != normalized.Name {
			if err := tx.Model(&model.Employee{}).Where("position_id = ?", position.ID).Update("title", normalized.Name).Error; err != nil {
				return err
			}
		}
		return replacePositionDepartments(tx, position.ID, departmentIDs)
	})
	if err != nil {
		return nil, err
	}
	if err := s.store.DB.Where("id = ?", position.ID).First(&position).Error; err != nil {
		return nil, err
	}
	if err := s.decoratePosition(&position); err != nil {
		return nil, err
	}
	return &position, nil
}

func (s *Service) DeletePosition(id string) error {
	var position model.Position
	if err := s.store.DB.Where("id = ?", strings.TrimSpace(id)).First(&position).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("%w: 岗位不存在", ErrNotFound)
	} else if err != nil {
		return err
	}
	if position.Builtin {
		return fmt.Errorf("%w: 内置岗位不能删除", ErrInvalid)
	}
	var count int64
	if err := s.store.DB.Model(&model.Employee{}).Where("position_id = ?", position.ID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("%w: 岗位仍有关联员工，不能删除", ErrConflict)
	}
	return s.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("position_id = ?", position.ID).Delete(&model.DepartmentPosition{}).Error; err != nil {
			return err
		}
		return tx.Delete(&position).Error
	})
}

func (s *Service) resolvePosition(role string, department *model.Department, positionID string) (*model.Position, error) {
	positionID = strings.TrimSpace(positionID)
	if positionID == "" {
		return nil, fmt.Errorf("%w: 必须选择岗位", ErrInvalid)
	}
	var position model.Position
	if err := s.store.DB.Where("id = ? AND status = ?", positionID, model.StatusEnabled).First(&position).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: 岗位不存在或已停用", ErrInvalid)
		}
		return nil, err
	}
	if department == nil {
		if role != model.RoleAdmin {
			return nil, fmt.Errorf("%w: 非管理员员工必须选择部门", ErrInvalid)
		}
		return &position, nil
	}
	var count int64
	if err := s.store.DB.Model(&model.DepartmentPosition{}).Where("department_id = ? AND position_id = ?", department.ID, position.ID).Count(&count).Error; err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, fmt.Errorf("%w: 所选岗位不属于该部门", ErrInvalid)
	}
	return &position, nil
}

func (s *Service) decoratePosition(position *model.Position) error {
	type departmentRow struct {
		ID   string
		Name string
	}
	rows := make([]departmentRow, 0)
	if err := s.store.DB.Model(&model.Department{}).
		Select("departments.id, departments.name").
		Joins("JOIN department_positions ON department_positions.department_id = departments.id").
		Where("department_positions.position_id = ?", position.ID).
		Order("departments.name ASC").Scan(&rows).Error; err != nil {
		return err
	}
	position.DepartmentIDs = make([]string, 0, len(rows))
	position.DepartmentNames = make([]string, 0, len(rows))
	for _, row := range rows {
		position.DepartmentIDs = append(position.DepartmentIDs, row.ID)
		position.DepartmentNames = append(position.DepartmentNames, row.Name)
	}
	return s.store.DB.Model(&model.Employee{}).Where("position_id = ?", position.ID).Count(&position.EmployeeCount).Error
}

func (s *Service) validatePositionDepartments(ids []string) ([]string, error) {
	result := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, rawID := range ids {
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		if len(id) > 40 {
			return nil, fmt.Errorf("%w: 部门标识格式无效", ErrInvalid)
		}
		if _, exists := seen[id]; exists {
			continue
		}
		var count int64
		if err := s.store.DB.Model(&model.Department{}).Where("id = ?", id).Count(&count).Error; err != nil {
			return nil, err
		}
		if count == 0 {
			return nil, fmt.Errorf("%w: 关联部门不存在", ErrInvalid)
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result, nil
}

func (s *Service) ensurePositionDepartmentsCoverEmployees(positionID string, departmentIDs []string) error {
	allowed := make(map[string]struct{}, len(departmentIDs))
	for _, id := range departmentIDs {
		allowed[id] = struct{}{}
	}
	var used []string
	if err := s.store.DB.Model(&model.Employee{}).Where("position_id = ? AND department_id <> ''", positionID).Distinct("department_id").Pluck("department_id", &used).Error; err != nil {
		return err
	}
	for _, id := range used {
		if _, exists := allowed[id]; !exists {
			return fmt.Errorf("%w: 仍有员工在待移除的关联部门中使用该岗位", ErrConflict)
		}
	}
	return nil
}

func (s *Service) positionExists(code, name, excludeID string) (bool, error) {
	query := s.store.DB.Model(&model.Position{}).Where("LOWER(code) = ? OR LOWER(name) = ?", strings.ToLower(code), strings.ToLower(name))
	if excludeID != "" {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	err := query.Count(&count).Error
	return count > 0, err
}

func replacePositionDepartments(tx *gorm.DB, positionID string, departmentIDs []string) error {
	if err := tx.Where("position_id = ?", positionID).Delete(&model.DepartmentPosition{}).Error; err != nil {
		return err
	}
	for _, departmentID := range departmentIDs {
		relation := model.DepartmentPosition{DepartmentID: departmentID, PositionID: positionID}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&relation).Error; err != nil {
			return err
		}
	}
	return nil
}

func normalizePosition(input PositionInput) (PositionInput, error) {
	input.Code = strings.ToLower(strings.TrimSpace(input.Code))
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	if !positionCodePattern.MatchString(input.Code) {
		return input, fmt.Errorf("%w: 岗位编码须以字母开头，且只能包含小写字母、数字、下划线或连字符", ErrInvalid)
	}
	if input.Name == "" || len([]rune(input.Name)) > 100 || len([]rune(input.Description)) > 500 {
		return input, fmt.Errorf("%w: 岗位名称或描述格式无效", ErrInvalid)
	}
	if input.Status == "" {
		input.Status = model.StatusEnabled
	}
	if input.Status != model.StatusEnabled && input.Status != model.StatusDisabled {
		return input, fmt.Errorf("%w: 岗位状态无效", ErrInvalid)
	}
	return input, nil
}
