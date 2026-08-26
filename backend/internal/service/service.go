package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"people/internal/model"
	"people/internal/store"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrInvalid      = errors.New("invalid argument")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
)

var (
	usernamePattern       = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.-]{2,63}$`)
	departmentCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)
)

type Service struct {
	store           *store.Store
	sessionTTL      time.Duration
	authorizer      Authorizer
	permissionMu    sync.Mutex
	permissionCache map[string]permissionCacheEntry
}

type permissionCacheEntry struct {
	permissions []string
	expiresAt   time.Time
}

const (
	PermissionEmployeeView      = "people.employee:view"
	PermissionEmployeeManage    = "people.employee:manage"
	PermissionEmployeeReset     = "people.employee:reset"
	PermissionEmployeeDisable   = "people.employee:disable"
	PermissionDepartmentManage  = "people.department:manage"
	PermissionDepartureView     = "people.departure:view"
	PermissionDepartureReview   = "people.departure:review"
	PermissionApprovalView      = "people.approval:view"
	PermissionApprovalReview    = "people.approval:review"
	PermissionContractView      = "people.contract:view"
	PermissionContractManage    = "people.contract:manage"
	PermissionPerformanceView   = "people.performance:view"
	PermissionPerformanceManage = "people.performance:manage"
	PermissionDashboardView     = "people.dashboard:view"
)

var elevatedPermissions = []string{
	PermissionEmployeeView, PermissionEmployeeManage, PermissionEmployeeReset, PermissionEmployeeDisable,
	PermissionDepartmentManage, PermissionDepartureView, PermissionDepartureReview, PermissionDashboardView,
	PermissionApprovalView, PermissionApprovalReview, PermissionContractView, PermissionContractManage,
	PermissionPerformanceView, PermissionPerformanceManage,
}

type Authorizer interface {
	Authorize(context.Context, string, []string) (map[string]bool, error)
}

type EmployeeInput struct {
	Username                 string `json:"username"`
	DisplayName              string `json:"displayName"`
	Email                    string `json:"email"`
	Phone                    string `json:"phone"`
	DepartmentID             string `json:"departmentId"`
	PositionID               string `json:"positionId"`
	EmploymentType           string `json:"employmentType"`
	HireDate                 string `json:"hireDate"`
	ProbationEndDate         string `json:"probationEndDate"`
	WorkLocation             string `json:"workLocation"`
	EmergencyContactName     string `json:"emergencyContactName"`
	EmergencyContactPhone    string `json:"emergencyContactPhone"`
	EmergencyContactRelation string `json:"emergencyContactRelation"`
	Role                     string `json:"role"`
	Status                   string `json:"status"`
}

type DepartmentInput struct {
	ParentID    string `json:"parentId"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	LeaderID    string `json:"leaderId"`
	Status      string `json:"status"`
}

type Page struct {
	Items    []model.Employee `json:"items"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"pageSize"`
}

type TokenResult struct {
	AccessToken string          `json:"access_token"`
	TokenType   string          `json:"token_type"`
	ExpiresIn   int64           `json:"expires_in"`
	Scope       string          `json:"scope"`
	User        *model.Employee `json:"user,omitempty"`
}

func New(store *store.Store, sessionTTL time.Duration, authorizers ...Authorizer) *Service {
	service := &Service{store: store, sessionTTL: sessionTTL, permissionCache: make(map[string]permissionCacheEntry)}
	if len(authorizers) > 0 {
		service.authorizer = authorizers[0]
	}
	return service
}

func (s *Service) loadPermissions(employee *model.Employee) {
	if employee == nil {
		return
	}
	employee.Permissions = []string{}
	if employee.Role == model.RoleAdmin {
		employee.Permissions = append(employee.Permissions, elevatedPermissions...)
		return
	}
	if s.authorizer == nil {
		return
	}
	now := time.Now().UTC()
	s.permissionMu.Lock()
	cached, exists := s.permissionCache[employee.Username]
	if exists && cached.expiresAt.After(now) {
		employee.Permissions = append(employee.Permissions, cached.permissions...)
		s.permissionMu.Unlock()
		return
	}
	s.permissionMu.Unlock()
	allowed, err := s.authorizer.Authorize(context.Background(), employee.Username, elevatedPermissions)
	if err != nil {
		return
	}
	for _, code := range elevatedPermissions {
		if allowed[code] {
			employee.Permissions = append(employee.Permissions, code)
		}
	}
	s.permissionMu.Lock()
	s.permissionCache[employee.Username] = permissionCacheEntry{permissions: append([]string(nil), employee.Permissions...), expiresAt: now.Add(15 * time.Second)}
	s.permissionMu.Unlock()
}

func (s *Service) HasPermission(employee *model.Employee, code string) bool {
	if employee == nil {
		return false
	}
	for _, current := range employee.Permissions {
		if current == code {
			return true
		}
	}
	return false
}

func (s *Service) Login(username, password string) (*model.Employee, string, error) {
	var employee model.Employee
	if err := s.store.DB.Where("LOWER(username) = ?", strings.ToLower(strings.TrimSpace(username))).First(&employee).Error; err != nil {
		return nil, "", ErrUnauthorized
	}
	if employee.Status != model.StatusEnabled {
		return nil, "", ErrUnauthorized
	}
	if !employee.MustChangePassword {
		if bcrypt.CompareHashAndPassword([]byte(employee.PasswordHash), []byte(password)) != nil {
			return nil, "", ErrUnauthorized
		}
	}
	token, err := randomToken("ps_", 32)
	if err != nil {
		return nil, "", err
	}
	now := time.Now().UTC()
	session := model.Session{EmployeeID: employee.ID, TokenHash: hash(token), ExpiresAt: now.Add(s.sessionTTL)}
	if err := s.store.DB.Create(&session).Error; err != nil {
		return nil, "", err
	}
	employee.LastLoginAt = &now
	if err := s.store.DB.Model(&employee).Update("last_login_at", now).Error; err != nil {
		return nil, "", err
	}
	s.loadPermissions(&employee)
	return &employee, token, nil
}

// AuthenticateOAuthAccount verifies an account for one authorization request
// without creating or replacing a People browser session.
func (s *Service) AuthenticateOAuthAccount(username, password string) (*model.Employee, error) {
	var employee model.Employee
	if err := s.store.DB.Where("LOWER(username) = ?", strings.ToLower(strings.TrimSpace(username))).First(&employee).Error; err != nil {
		return nil, ErrUnauthorized
	}
	if employee.Status != model.StatusEnabled || employee.MustChangePassword || employee.PasswordHash == "" {
		return nil, ErrUnauthorized
	}
	if bcrypt.CompareHashAndPassword([]byte(employee.PasswordHash), []byte(password)) != nil {
		return nil, ErrUnauthorized
	}
	return &employee, nil
}

func (s *Service) AuthenticateSession(token string) (*model.Employee, *model.Session, error) {
	if token == "" {
		return nil, nil, ErrUnauthorized
	}
	var session model.Session
	err := s.store.DB.Preload("Employee").Where("token_hash = ? AND revoked_at IS NULL AND expires_at > ?", hash(token), time.Now().UTC()).First(&session).Error
	if err != nil || session.Employee.Status != model.StatusEnabled {
		return nil, nil, ErrUnauthorized
	}
	s.loadPermissions(&session.Employee)
	return &session.Employee, &session, nil
}

func (s *Service) Logout(sessionID uint) error {
	now := time.Now().UTC()
	return s.store.DB.Model(&model.Session{}).Where("id = ?", sessionID).Update("revoked_at", now).Error
}

func (s *Service) ChangePassword(employee *model.Employee, sessionID uint, currentPassword, newPassword string) error {
	if len(newPassword) < 8 || len(newPassword) > 72 {
		return fmt.Errorf("%w: 新密码长度必须为 8 到 72 个字符", ErrInvalid)
	}
	if !employee.MustChangePassword && bcrypt.CompareHashAndPassword([]byte(employee.PasswordHash), []byte(currentPassword)) != nil {
		return fmt.Errorf("%w: 当前密码错误", ErrInvalid)
	}
	hashValue, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	return s.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Employee{}).Where("id = ?", employee.ID).Updates(map[string]any{
			"password_hash": string(hashValue), "must_change_password": false, "password_changed_at": now,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&model.Session{}).Where("employee_id = ? AND id <> ? AND revoked_at IS NULL", employee.ID, sessionID).
			Update("revoked_at", now).Error
	})
}

func (s *Service) ListEmployees(search string, page, pageSize int) (Page, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	query := s.store.DB.Model(&model.Employee{})
	if value := strings.TrimSpace(search); value != "" {
		like := "%" + value + "%"
		if employeeNo, err := strconv.ParseUint(value, 10, 64); err == nil {
			query = query.Where("id = ? OR username LIKE ? OR display_name LIKE ? OR email LIKE ? OR department LIKE ? OR title LIKE ?", employeeNo, like, like, like, like, like)
		} else {
			query = query.Where("username LIKE ? OR display_name LIKE ? OR email LIKE ? OR department LIKE ? OR title LIKE ?", like, like, like, like, like)
		}
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return Page{}, err
	}
	var items []model.Employee
	if err := query.Order("created_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return Page{}, err
	}
	return Page{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *Service) GetEmployee(publicID string) (*model.Employee, error) {
	var employee model.Employee
	if err := s.store.DB.Where("public_id = ?", strings.TrimSpace(publicID)).First(&employee).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	return &employee, nil
}

func (s *Service) CreateEmployee(input EmployeeInput) (*model.Employee, error) {
	input.Role = model.RoleEmployee
	input.Status = model.StatusEnabled
	normalized, err := normalizeEmployee(input)
	if err != nil {
		return nil, err
	}
	department, err := s.resolveDepartment(normalized.Role, normalized.DepartmentID)
	if err != nil {
		return nil, err
	}
	position, err := s.resolvePosition(normalized.Role, department, normalized.PositionID)
	if err != nil {
		return nil, err
	}
	var count int64
	if err := s.store.DB.Model(&model.Employee{}).Where("LOWER(username) = ?", strings.ToLower(normalized.Username)).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, fmt.Errorf("%w: 用户名已存在", ErrConflict)
	}
	publicID, err := randomToken("pep_", 18)
	if err != nil {
		return nil, err
	}
	employee := model.Employee{
		PublicID: publicID, LegacyEmployeeNo: publicID, Username: normalized.Username,
		DisplayName: normalized.DisplayName, Email: normalized.Email, Phone: normalized.Phone,
		PositionID: position.ID, Title: position.Name, EmploymentType: normalized.EmploymentType, HireDate: normalized.HireDate,
		ProbationEndDate: normalized.ProbationEndDate, WorkLocation: normalized.WorkLocation, Role: normalized.Role,
		EmergencyContactName: normalized.EmergencyContactName, EmergencyContactPhone: normalized.EmergencyContactPhone,
		EmergencyContactRelation: normalized.EmergencyContactRelation,
		Status:                   normalized.Status, MustChangePassword: true,
	}
	if department != nil {
		employee.DepartmentID = department.ID
		employee.Department = department.Name
	}
	if err := s.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&employee).Error; err != nil {
			return err
		}
		effectiveDate := employee.HireDate
		if effectiveDate == "" {
			effectiveDate = time.Now().UTC().Format("2006-01-02")
		}
		return createEmploymentEvent(tx, employee, model.EmploymentEventHire, effectiveDate, "", "", employee.DepartmentID, employee.Department, "", employee.Title, "员工加入组织", "")
	}); err != nil {
		return nil, err
	}
	return &employee, nil
}

func (s *Service) UpdateEmployee(publicID string, input EmployeeInput) (*model.Employee, error) {
	var employee model.Employee
	if err := s.store.DB.Where("public_id = ?", publicID).First(&employee).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	if employee.Username == "admin" {
		input.Username = "admin"
		input.DepartmentID = employee.DepartmentID
		input.PositionID = model.PositionSystemAdminID
	}
	input.Role = employee.Role
	input.Status = employee.Status
	normalized, err := normalizeEmployee(input)
	if err != nil {
		return nil, err
	}
	department, err := s.resolveDepartment(normalized.Role, normalized.DepartmentID)
	if err != nil {
		return nil, err
	}
	position, err := s.resolvePosition(normalized.Role, department, normalized.PositionID)
	if err != nil {
		return nil, err
	}
	if employee.DepartmentID != normalized.DepartmentID {
		var leaderCount int64
		if err := s.store.DB.Model(&model.Department{}).Where("leader_id = ?", employee.PublicID).Count(&leaderCount).Error; err != nil {
			return nil, err
		}
		if leaderCount > 0 {
			return nil, fmt.Errorf("%w: 请先为其负责的部门更换负责人", ErrConflict)
		}
	}
	var count int64
	if err := s.store.DB.Model(&model.Employee{}).Where("id <> ? AND LOWER(username) = ?", employee.ID, strings.ToLower(normalized.Username)).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, fmt.Errorf("%w: 用户名已存在", ErrConflict)
	}
	departmentID, departmentName := "", ""
	if department != nil {
		departmentID, departmentName = department.ID, department.Name
	}
	departmentChanged, positionChanged := employee.DepartmentID != departmentID, employee.PositionID != position.ID
	if err := s.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&employee).Updates(map[string]any{
			"username": normalized.Username, "display_name": normalized.DisplayName, "email": normalized.Email, "phone": normalized.Phone,
			"department_id": departmentID, "department": departmentName, "position_id": position.ID, "title": position.Name,
			"employment_type": normalized.EmploymentType, "hire_date": normalized.HireDate,
			"probation_end_date": normalized.ProbationEndDate, "work_location": normalized.WorkLocation,
			"emergency_contact_name": normalized.EmergencyContactName, "emergency_contact_phone": normalized.EmergencyContactPhone,
			"emergency_contact_relation": normalized.EmergencyContactRelation, "role": normalized.Role, "status": normalized.Status,
		}).Error; err != nil {
			return err
		}
		if !departmentChanged && !positionChanged {
			return nil
		}
		eventType := model.EmploymentEventPromotion
		if departmentChanged {
			eventType = model.EmploymentEventTransfer
		}
		return createEmploymentEvent(tx, employee, eventType, time.Now().UTC().Format("2006-01-02"), employee.DepartmentID, employee.Department, departmentID, departmentName, employee.Title, position.Name, "HR 直接调整员工任职信息", "")
	}); err != nil {
		return nil, err
	}
	return &employee, s.store.DB.Where("id = ?", employee.ID).First(&employee).Error
}

func (s *Service) ListDepartments(search string) ([]model.Department, error) {
	query := s.store.DB.Model(&model.Department{})
	if value := strings.TrimSpace(search); value != "" {
		like := "%" + value + "%"
		query = query.Where("code LIKE ? OR name LIKE ?", like, like)
	}
	var departments []model.Department
	if err := query.Order("CASE WHEN status = 'enabled' THEN 0 ELSE 1 END, name ASC").Find(&departments).Error; err != nil {
		return nil, err
	}
	for index := range departments {
		if err := s.store.DB.Model(&model.Employee{}).Where("department_id = ?", departments[index].ID).
			Count(&departments[index].EmployeeCount).Error; err != nil {
			return nil, err
		}
		if departments[index].LeaderID != "" {
			var leader model.Employee
			if err := s.store.DB.Select("display_name").Where("public_id = ?", departments[index].LeaderID).First(&leader).Error; err == nil {
				departments[index].LeaderName = leader.DisplayName
			}
		}
	}
	return departments, nil
}

func (s *Service) CreateDepartment(input DepartmentInput) (*model.Department, error) {
	normalized, err := normalizeDepartment(input)
	if err != nil {
		return nil, err
	}
	if exists, err := s.departmentExists(normalized.Code, normalized.Name, ""); err != nil {
		return nil, err
	} else if exists {
		return nil, fmt.Errorf("%w: 部门编码或名称已存在", ErrConflict)
	}
	if err := s.validateDepartmentParent("", normalized.ParentID); err != nil {
		return nil, err
	}
	id, err := randomToken("dep_", 18)
	if err != nil {
		return nil, err
	}
	if err := s.validateDepartmentLeader(id, normalized.LeaderID); err != nil {
		return nil, err
	}
	department := model.Department{ID: id, ParentID: normalized.ParentID, Code: normalized.Code, Name: normalized.Name, Description: normalized.Description, LeaderID: normalized.LeaderID, Status: normalized.Status}
	if err := s.store.DB.Create(&department).Error; err != nil {
		return nil, err
	}
	return &department, nil
}

func (s *Service) UpdateDepartment(id string, input DepartmentInput) (*model.Department, error) {
	var department model.Department
	if err := s.store.DB.Where("id = ?", id).First(&department).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("%w: 部门不存在", ErrNotFound)
	} else if err != nil {
		return nil, err
	}
	normalized, err := normalizeDepartment(input)
	if err != nil {
		return nil, err
	}
	if exists, err := s.departmentExists(normalized.Code, normalized.Name, department.ID); err != nil {
		return nil, err
	} else if exists {
		return nil, fmt.Errorf("%w: 部门编码或名称已存在", ErrConflict)
	}
	if err := s.validateDepartmentParent(department.ID, normalized.ParentID); err != nil {
		return nil, err
	}
	if err := s.validateDepartmentLeader(department.ID, normalized.LeaderID); err != nil {
		return nil, err
	}
	err = s.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&department).Updates(map[string]any{
			"parent_id": normalized.ParentID, "code": normalized.Code, "name": normalized.Name,
			"description": normalized.Description, "leader_id": normalized.LeaderID, "status": normalized.Status,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&model.Employee{}).Where("department_id = ?", department.ID).Update("department", normalized.Name).Error
	})
	if err != nil {
		return nil, err
	}
	if err := s.store.DB.Where("id = ?", department.ID).First(&department).Error; err != nil {
		return nil, err
	}
	if err := s.store.DB.Model(&model.Employee{}).Where("department_id = ?", department.ID).Count(&department.EmployeeCount).Error; err != nil {
		return nil, err
	}
	return &department, nil
}

func (s *Service) validateDepartmentLeader(departmentID, leaderID string) error {
	leaderID = strings.TrimSpace(leaderID)
	if leaderID == "" {
		return nil
	}
	var leader model.Employee
	if err := s.store.DB.Where("public_id = ? AND department_id = ? AND status = ?", leaderID, departmentID, model.StatusEnabled).First(&leader).Error; err != nil {
		return fmt.Errorf("%w: 部门负责人必须是本部门的在职员工", ErrInvalid)
	}
	return nil
}

func (s *Service) DeleteDepartment(id string) error {
	var department model.Department
	if err := s.store.DB.Where("id = ?", id).First(&department).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("%w: 部门不存在", ErrNotFound)
	} else if err != nil {
		return err
	}
	var childCount int64
	if err := s.store.DB.Model(&model.Department{}).Where("parent_id = ?", id).Count(&childCount).Error; err != nil {
		return err
	}
	if childCount > 0 {
		return fmt.Errorf("%w: 部门仍有下级部门，不能删除", ErrConflict)
	}
	var count int64
	if err := s.store.DB.Model(&model.Employee{}).Where("department_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("%w: 部门仍有关联员工，不能删除", ErrConflict)
	}
	return s.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("department_id = ?", department.ID).Delete(&model.DepartmentPosition{}).Error; err != nil {
			return err
		}
		return tx.Delete(&department).Error
	})
}

func (s *Service) validateDepartmentParent(departmentID, parentID string) error {
	parentID = strings.TrimSpace(parentID)
	if parentID == "" {
		return nil
	}
	if parentID == departmentID {
		return fmt.Errorf("%w: 部门不能作为自己的上级", ErrInvalid)
	}
	visited := map[string]struct{}{departmentID: {}}
	currentID := parentID
	for currentID != "" {
		if _, exists := visited[currentID]; exists {
			return fmt.Errorf("%w: 部门层级不能形成循环", ErrInvalid)
		}
		visited[currentID] = struct{}{}
		var current model.Department
		if err := s.store.DB.Select("id", "parent_id").Where("id = ?", currentID).First(&current).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: 上级部门不存在", ErrInvalid)
		} else if err != nil {
			return err
		}
		currentID = current.ParentID
	}
	return nil
}

func (s *Service) resolveDepartment(role, id string) (*model.Department, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		if role == model.RoleEmployee {
			return nil, fmt.Errorf("%w: 非管理员员工必须选择部门", ErrInvalid)
		}
		return nil, nil
	}
	var department model.Department
	if err := s.store.DB.Where("id = ? AND status = ?", id, model.StatusEnabled).First(&department).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: 部门不存在或已停用", ErrInvalid)
		}
		return nil, err
	}
	return &department, nil
}

func (s *Service) departmentExists(code, name, excludeID string) (bool, error) {
	query := s.store.DB.Model(&model.Department{}).Where("(LOWER(code) = ? OR LOWER(name) = ?)", strings.ToLower(code), strings.ToLower(name))
	if excludeID != "" {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	err := query.Count(&count).Error
	return count > 0, err
}

func (s *Service) DeleteEmployee(publicID string, actorID uint) error {
	var employee model.Employee
	if err := s.store.DB.Where("public_id = ?", publicID).First(&employee).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if employee.ID == actorID || employee.Username == "admin" {
		return fmt.Errorf("%w: 不能删除当前用户或内置管理员", ErrInvalid)
	}
	var leaderCount int64
	if err := s.store.DB.Model(&model.Department{}).Where("leader_id = ?", employee.PublicID).Count(&leaderCount).Error; err != nil {
		return err
	}
	if leaderCount > 0 {
		return fmt.Errorf("%w: 请先为其负责的部门更换负责人", ErrConflict)
	}
	return s.store.DB.Delete(&employee).Error
}

func (s *Service) Authorize(employee *model.Employee, clientID, redirectURI, state string) (string, error) {
	if employee.MustChangePassword {
		return "", fmt.Errorf("%w: 请先设置登录密码", ErrForbidden)
	}
	client, err := s.oauthClientByID(clientID)
	if err != nil || !containsLine(client.RedirectURIs, redirectURI) {
		return "", fmt.Errorf("%w: OAuth 客户端或回调地址无效", ErrInvalid)
	}
	code, err := randomToken("poc_", 32)
	if err != nil {
		return "", err
	}
	record := model.OAuthCode{
		CodeHash: hash(code), ClientID: clientID, EmployeeID: employee.ID,
		RedirectURI: redirectURI, ExpiresAt: time.Now().UTC().Add(5 * time.Minute),
	}
	if err := s.store.DB.Create(&record).Error; err != nil {
		return "", err
	}
	target, _ := url.Parse(redirectURI)
	query := target.Query()
	query.Set("code", code)
	query.Set("state", state)
	target.RawQuery = query.Encode()
	return target.String(), nil
}

func (s *Service) ExchangeCode(clientID, clientSecret, code, redirectURI string) (TokenResult, error) {
	if _, err := s.authenticateOAuthClient(clientID, clientSecret); err != nil {
		return TokenResult{}, err
	}
	var result TokenResult
	err := s.store.DB.Transaction(func(tx *gorm.DB) error {
		var record model.OAuthCode
		if err := tx.Where("code_hash = ? AND client_id = ? AND redirect_uri = ? AND used_at IS NULL AND expires_at > ?",
			hash(code), clientID, redirectURI, time.Now().UTC()).First(&record).Error; err != nil {
			return ErrUnauthorized
		}
		var employee model.Employee
		if err := tx.First(&employee, record.EmployeeID).Error; err != nil || employee.Status != model.StatusEnabled {
			return ErrUnauthorized
		}
		consumed := tx.Model(&model.OAuthCode{}).Where("id = ? AND used_at IS NULL", record.ID).Update("used_at", time.Now().UTC())
		if consumed.Error != nil {
			return consumed.Error
		}
		if consumed.RowsAffected != 1 {
			return ErrUnauthorized
		}
		var err error
		result, err = s.issueOAuthTokenWithDB(tx, clientID, employee.ID, "openid profile", &employee)
		return err
	})
	return result, err
}

func (s *Service) ClientCredentials(clientID, clientSecret, scope string) (TokenResult, error) {
	client, err := s.authenticateOAuthClient(clientID, clientSecret)
	if err != nil {
		return TokenResult{}, err
	}
	if scope == "" {
		scope = "employees.read"
	}
	for _, requested := range strings.Fields(scope) {
		if !containsField(client.AllowedScopes, requested) {
			return TokenResult{}, fmt.Errorf("%w: OAuth scope 无效", ErrInvalid)
		}
	}
	return s.issueOAuthToken(clientID, 0, scope, nil)
}

func (s *Service) OAuthIdentity(token string) (*model.Employee, string, error) {
	var record model.OAuthToken
	if err := s.store.DB.Where("token_hash = ? AND expires_at > ?", hash(token), time.Now().UTC()).First(&record).Error; err != nil {
		return nil, "", ErrUnauthorized
	}
	if record.EmployeeID == 0 {
		return nil, record.Scope, nil
	}
	var employee model.Employee
	if err := s.store.DB.First(&employee, record.EmployeeID).Error; err != nil || employee.Status != model.StatusEnabled {
		return nil, "", ErrUnauthorized
	}
	return &employee, record.Scope, nil
}

func (s *Service) issueOAuthToken(clientID string, employeeID uint, scope string, employee *model.Employee) (TokenResult, error) {
	return s.issueOAuthTokenWithDB(s.store.DB, clientID, employeeID, scope, employee)
}

func (s *Service) issueOAuthTokenWithDB(db *gorm.DB, clientID string, employeeID uint, scope string, employee *model.Employee) (TokenResult, error) {
	token, err := randomToken("pat_", 32)
	if err != nil {
		return TokenResult{}, err
	}
	ttl := time.Hour
	if employeeID == 0 {
		ttl = 10 * time.Minute
	}
	if err := db.Create(&model.OAuthToken{
		TokenHash: hash(token), ClientID: clientID, EmployeeID: employeeID, Scope: scope, ExpiresAt: time.Now().UTC().Add(ttl),
	}).Error; err != nil {
		return TokenResult{}, err
	}
	return TokenResult{AccessToken: token, TokenType: "Bearer", ExpiresIn: int64(ttl.Seconds()), Scope: scope, User: employee}, nil
}

func (s *Service) oauthClientByID(clientID string) (*model.OAuthClient, error) {
	var client model.OAuthClient
	if err := s.store.DB.Where("client_id = ?", clientID).First(&client).Error; err != nil {
		return nil, ErrUnauthorized
	}
	return &client, nil
}

func (s *Service) authenticateOAuthClient(clientID, clientSecret string) (*model.OAuthClient, error) {
	client, err := s.oauthClientByID(clientID)
	if err != nil || clientSecret == "" || subtle.ConstantTimeCompare([]byte(client.SecretHash), []byte(hash(clientSecret))) != 1 {
		return nil, ErrUnauthorized
	}
	return client, nil
}

func normalizeEmployee(input EmployeeInput) (EmployeeInput, error) {
	input.Username = strings.TrimSpace(input.Username)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Phone = strings.TrimSpace(input.Phone)
	input.DepartmentID = strings.TrimSpace(input.DepartmentID)
	input.PositionID = strings.TrimSpace(input.PositionID)
	input.EmploymentType = strings.ToLower(strings.TrimSpace(input.EmploymentType))
	input.HireDate = strings.TrimSpace(input.HireDate)
	input.ProbationEndDate = strings.TrimSpace(input.ProbationEndDate)
	input.WorkLocation = strings.TrimSpace(input.WorkLocation)
	input.EmergencyContactName = strings.TrimSpace(input.EmergencyContactName)
	input.EmergencyContactPhone = strings.TrimSpace(input.EmergencyContactPhone)
	input.EmergencyContactRelation = strings.TrimSpace(input.EmergencyContactRelation)
	input.Role = strings.ToLower(strings.TrimSpace(input.Role))
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	if !usernamePattern.MatchString(input.Username) || input.DisplayName == "" || len(input.DisplayName) > 100 {
		return input, fmt.Errorf("%w: 用户名或姓名格式无效", ErrInvalid)
	}
	if input.Role == "" {
		input.Role = model.RoleEmployee
	}
	if input.Role != model.RoleAdmin && input.Role != model.RoleEmployee {
		return input, fmt.Errorf("%w: 角色无效", ErrInvalid)
	}
	if input.Status == "" {
		input.Status = model.StatusEnabled
	}
	if input.Status != model.StatusEnabled && input.Status != model.StatusDisabled {
		return input, fmt.Errorf("%w: 状态无效", ErrInvalid)
	}
	if input.EmploymentType == "" {
		input.EmploymentType = model.EmploymentFullTime
	}
	switch input.EmploymentType {
	case model.EmploymentFullTime, model.EmploymentPartTime, model.EmploymentContract, model.EmploymentIntern:
	default:
		return input, fmt.Errorf("%w: 用工类型无效", ErrInvalid)
	}
	for _, date := range []string{input.HireDate, input.ProbationEndDate} {
		if date != "" {
			if _, err := time.Parse("2006-01-02", date); err != nil {
				return input, fmt.Errorf("%w: 日期格式必须为 YYYY-MM-DD", ErrInvalid)
			}
		}
	}
	if input.HireDate != "" && input.ProbationEndDate != "" && input.ProbationEndDate < input.HireDate {
		return input, fmt.Errorf("%w: 试用期结束日期不能早于入职日期", ErrInvalid)
	}
	if input.PositionID == "" {
		return input, fmt.Errorf("%w: 必须选择岗位", ErrInvalid)
	}
	if len(input.Email) > 255 || len(input.Phone) > 32 || len(input.DepartmentID) > 40 || len(input.PositionID) > 40 || len(input.WorkLocation) > 100 || len([]rune(input.EmergencyContactName)) > 100 || len(input.EmergencyContactPhone) > 32 || len([]rune(input.EmergencyContactRelation)) > 50 {
		return input, fmt.Errorf("%w: 员工资料过长", ErrInvalid)
	}
	return input, nil
}

func normalizeDepartment(input DepartmentInput) (DepartmentInput, error) {
	input.ParentID = strings.TrimSpace(input.ParentID)
	input.Code = strings.ToLower(strings.TrimSpace(input.Code))
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.LeaderID = strings.TrimSpace(input.LeaderID)
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	if !departmentCodePattern.MatchString(input.Code) {
		return input, fmt.Errorf("%w: 部门编码须以字母开头，且只能包含小写字母、数字、下划线或连字符", ErrInvalid)
	}
	if input.Name == "" || len(input.Name) > 100 || len(input.ParentID) > 40 || len(input.Description) > 500 || len(input.LeaderID) > 40 {
		return input, fmt.Errorf("%w: 部门名称或描述格式无效", ErrInvalid)
	}
	if input.Status == "" {
		input.Status = model.StatusEnabled
	}
	if input.Status != model.StatusEnabled && input.Status != model.StatusDisabled {
		return input, fmt.Errorf("%w: 部门状态无效", ErrInvalid)
	}
	return input, nil
}

func randomToken(prefix string, size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(value), nil
}

func hash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func containsLine(lines, value string) bool {
	for _, item := range strings.Split(lines, "\n") {
		if item == value {
			return true
		}
	}
	return false
}

func containsField(fields, value string) bool {
	for _, item := range strings.Fields(fields) {
		if item == value {
			return true
		}
	}
	return false
}
