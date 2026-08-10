package model

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

const (
	StatusEnabled  = "enabled"
	StatusDisabled = "disabled"
	RoleAdmin      = "admin"
	RoleEmployee   = "employee"

	EmploymentFullTime = "full_time"
	EmploymentPartTime = "part_time"
	EmploymentContract = "contract"
	EmploymentIntern   = "intern"

	ApprovalTypeDeparture = "departure"
	ApprovalTypeLeave     = "leave"
	ApprovalTypeTransfer  = "transfer"

	ApprovalPending   = "pending"
	ApprovalApproved  = "approved"
	ApprovalRejected  = "rejected"
	ApprovalCancelled = "cancelled"

	ApprovalStepWaiting  = "waiting"
	ApprovalStepPending  = "pending"
	ApprovalStepApproved = "approved"
	ApprovalStepRejected = "rejected"
	ApprovalStepSkipped  = "skipped"

	EmploymentEventHire      = "hire"
	EmploymentEventTransfer  = "transfer"
	EmploymentEventPromotion = "promotion"
	EmploymentEventDeparture = "departure"
	EmploymentEventEnable    = "enable"
	EmploymentEventDisable   = "disable"

	PositionSystemAdminID = "pos_system_admin"
	PositionGeneralID     = "pos_general_employee"

	DeparturePendingManager = "pending_manager"
	DeparturePendingHR      = "pending_hr"
	DepartureApproved       = "approved"
	DepartureRejected       = "rejected"
	DepartureCancelled      = "cancelled"
)

type Employee struct {
	ID                       uint       `json:"-" gorm:"primaryKey"`
	PublicID                 string     `json:"id" gorm:"size:40;uniqueIndex;not null"`
	EmployeeNo               uint       `json:"employeeNo" gorm:"-"`
	LegacyEmployeeNo         string     `json:"-" gorm:"column:employee_no;size:64;uniqueIndex;not null"`
	Username                 string     `json:"username" gorm:"size:64;uniqueIndex;not null"`
	DisplayName              string     `json:"displayName" gorm:"size:100;not null"`
	Email                    string     `json:"email" gorm:"size:255"`
	Phone                    string     `json:"phone" gorm:"size:32"`
	DepartmentID             string     `json:"departmentId" gorm:"size:40;index"`
	Department               string     `json:"department" gorm:"size:100"`
	PositionID               string     `json:"positionId" gorm:"size:40;not null;default:pos_general_employee;index"`
	Title                    string     `json:"title" gorm:"size:100"`
	EmploymentType           string     `json:"employmentType" gorm:"size:24;not null;default:full_time"`
	HireDate                 string     `json:"hireDate" gorm:"size:10"`
	ProbationEndDate         string     `json:"probationEndDate" gorm:"size:10"`
	WorkLocation             string     `json:"workLocation" gorm:"size:100"`
	EmergencyContactName     string     `json:"emergencyContactName" gorm:"size:100"`
	EmergencyContactPhone    string     `json:"emergencyContactPhone" gorm:"size:32"`
	EmergencyContactRelation string     `json:"emergencyContactRelation" gorm:"size:50"`
	Role                     string     `json:"role" gorm:"size:16;not null;default:employee"`
	Status                   string     `json:"status" gorm:"size:16;not null;default:enabled;index"`
	Permissions              []string   `json:"permissions" gorm:"-"`
	PasswordHash             string     `json:"-" gorm:"size:255"`
	MustChangePassword       bool       `json:"mustChangePassword" gorm:"not null;default:true"`
	PasswordChangedAt        *time.Time `json:"passwordChangedAt"`
	LastLoginAt              *time.Time `json:"lastLoginAt"`
	CreatedAt                time.Time  `json:"createdAt"`
	UpdatedAt                time.Time  `json:"updatedAt"`
}

type ApprovalRequest struct {
	ID                string         `json:"id" gorm:"size:40;primaryKey"`
	Type              string         `json:"type" gorm:"size:32;not null;index"`
	Title             string         `json:"title" gorm:"size:160;not null"`
	Summary           string         `json:"summary" gorm:"size:500;not null"`
	ApplicantID       uint           `json:"-" gorm:"not null;index"`
	ApplicantPublicID string         `json:"applicantId" gorm:"size:40;not null;index"`
	ApplicantName     string         `json:"applicantName" gorm:"size:100;not null"`
	ApplicantNo       uint           `json:"applicantNo" gorm:"not null"`
	DepartmentID      string         `json:"departmentId" gorm:"size:40;index"`
	DepartmentName    string         `json:"departmentName" gorm:"size:100"`
	DataJSON          string         `json:"-" gorm:"column:data;type:text;not null"`
	Data              map[string]any `json:"data" gorm:"-"`
	Status            string         `json:"status" gorm:"size:24;not null;index"`
	CurrentStep       int            `json:"currentStep" gorm:"not null;default:1"`
	TotalSteps        int            `json:"totalSteps" gorm:"not null"`
	CurrentStepName   string         `json:"currentStepName" gorm:"-"`
	Steps             []ApprovalStep `json:"steps" gorm:"foreignKey:ApprovalID;references:ID;constraint:OnDelete:CASCADE"`
	CanReview         bool           `json:"canReview" gorm:"-"`
	CanCancel         bool           `json:"canCancel" gorm:"-"`
	SubmittedAt       time.Time      `json:"submittedAt" gorm:"not null;index"`
	CompletedAt       *time.Time     `json:"completedAt"`
	CreatedAt         time.Time      `json:"createdAt"`
	UpdatedAt         time.Time      `json:"updatedAt"`
}

func (request *ApprovalRequest) AfterFind(_ *gorm.DB) error {
	if request.DataJSON == "" {
		request.Data = map[string]any{}
		return nil
	}
	return json.Unmarshal([]byte(request.DataJSON), &request.Data)
}

type ApprovalStep struct {
	ID             uint       `json:"id" gorm:"primaryKey"`
	ApprovalID     string     `json:"-" gorm:"size:40;not null;uniqueIndex:idx_approval_step"`
	Sequence       int        `json:"sequence" gorm:"not null;uniqueIndex:idx_approval_step"`
	Name           string     `json:"name" gorm:"size:100;not null"`
	ApproverID     string     `json:"approverId" gorm:"size:40;index"`
	PermissionCode string     `json:"permissionCode" gorm:"size:100;index"`
	Status         string     `json:"status" gorm:"size:24;not null;index"`
	ReviewerID     string     `json:"reviewerId" gorm:"size:40"`
	ReviewerName   string     `json:"reviewerName" gorm:"size:100"`
	Comment        string     `json:"comment" gorm:"size:500"`
	ReviewedAt     *time.Time `json:"reviewedAt"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

type LeaveRecord struct {
	ID               string    `json:"id" gorm:"size:40;primaryKey"`
	ApprovalID       string    `json:"approvalId" gorm:"size:40;uniqueIndex;not null"`
	EmployeeID       uint      `json:"-" gorm:"not null;index"`
	EmployeePublicID string    `json:"employeeId" gorm:"size:40;not null;index"`
	EmployeeName     string    `json:"employeeName" gorm:"size:100;not null"`
	DepartmentID     string    `json:"departmentId" gorm:"size:40;index"`
	DepartmentName   string    `json:"departmentName" gorm:"size:100"`
	LeaveType        string    `json:"leaveType" gorm:"size:24;not null;index"`
	StartDate        string    `json:"startDate" gorm:"size:10;not null;index"`
	EndDate          string    `json:"endDate" gorm:"size:10;not null;index"`
	Days             float64   `json:"days" gorm:"not null"`
	Reason           string    `json:"reason" gorm:"size:1000"`
	Status           string    `json:"status" gorm:"size:24;not null;index"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type LeaveBalance struct {
	ID                uint      `json:"-" gorm:"primaryKey"`
	EmployeeID        uint      `json:"-" gorm:"not null;uniqueIndex:idx_leave_balance"`
	EmployeePublicID  string    `json:"employeeId" gorm:"size:40;not null;index"`
	Year              int       `json:"year" gorm:"not null;uniqueIndex:idx_leave_balance"`
	AnnualEntitlement float64   `json:"annualEntitlement" gorm:"not null;default:10"`
	AnnualUsed        float64   `json:"annualUsed" gorm:"not null;default:0"`
	AnnualPending     float64   `json:"annualPending" gorm:"-"`
	AnnualRemaining   float64   `json:"annualRemaining" gorm:"-"`
	SickUsed          float64   `json:"sickUsed" gorm:"not null;default:0"`
	PersonalUsed      float64   `json:"personalUsed" gorm:"not null;default:0"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type EmploymentEvent struct {
	ID               string    `json:"id" gorm:"size:40;primaryKey"`
	EmployeeID       uint      `json:"-" gorm:"not null;index"`
	EmployeePublicID string    `json:"employeeId" gorm:"size:40;not null;index"`
	Type             string    `json:"type" gorm:"size:32;not null;index"`
	EffectiveDate    string    `json:"effectiveDate" gorm:"size:10;not null;index"`
	FromDepartmentID string    `json:"fromDepartmentId" gorm:"size:40"`
	FromDepartment   string    `json:"fromDepartment" gorm:"size:100"`
	ToDepartmentID   string    `json:"toDepartmentId" gorm:"size:40"`
	ToDepartment     string    `json:"toDepartment" gorm:"size:100"`
	FromTitle        string    `json:"fromTitle" gorm:"size:100"`
	ToTitle          string    `json:"toTitle" gorm:"size:100"`
	Note             string    `json:"note" gorm:"size:500"`
	ApprovalID       string    `json:"approvalId" gorm:"size:40;index"`
	CreatedAt        time.Time `json:"createdAt"`
}

type EmployeeContract struct {
	ID               string    `json:"id" gorm:"size:40;primaryKey"`
	EmployeeID       uint      `json:"-" gorm:"not null;index"`
	EmployeePublicID string    `json:"employeeId" gorm:"size:40;not null;index"`
	EmployeeName     string    `json:"employeeName" gorm:"size:100;not null"`
	Type             string    `json:"type" gorm:"size:32;not null"`
	StartDate        string    `json:"startDate" gorm:"size:10;not null;index"`
	EndDate          string    `json:"endDate" gorm:"size:10;index"`
	Status           string    `json:"status" gorm:"size:24;not null;index"`
	DocumentName     string    `json:"documentName" gorm:"size:255"`
	Note             string    `json:"note" gorm:"size:1000"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type PerformanceGoal struct {
	ID               string    `json:"id" gorm:"size:40;primaryKey"`
	EmployeeID       uint      `json:"-" gorm:"not null;index"`
	EmployeePublicID string    `json:"employeeId" gorm:"size:40;not null;index"`
	EmployeeName     string    `json:"employeeName" gorm:"size:100;not null"`
	DepartmentID     string    `json:"departmentId" gorm:"size:40;index"`
	Cycle            string    `json:"cycle" gorm:"size:32;not null;index"`
	Title            string    `json:"title" gorm:"size:160;not null"`
	Description      string    `json:"description" gorm:"size:1000"`
	DueDate          string    `json:"dueDate" gorm:"size:10;not null;index"`
	Weight           int       `json:"weight" gorm:"not null"`
	Progress         int       `json:"progress" gorm:"not null;default:0"`
	Status           string    `json:"status" gorm:"size:24;not null;index"`
	ManagerComment   string    `json:"managerComment" gorm:"size:1000"`
	CanEdit          bool      `json:"canEdit" gorm:"-"`
	CanReview        bool      `json:"canReview" gorm:"-"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

func (employee *Employee) AfterFind(_ *gorm.DB) error {
	employee.EmployeeNo = employee.ID
	return nil
}

func (employee *Employee) AfterCreate(_ *gorm.DB) error {
	employee.EmployeeNo = employee.ID
	return nil
}

type Department struct {
	ID            string    `json:"id" gorm:"size:40;primaryKey"`
	ParentID      string    `json:"parentId" gorm:"size:40;index"`
	Code          string    `json:"code" gorm:"size:32;uniqueIndex;not null"`
	Name          string    `json:"name" gorm:"size:100;uniqueIndex;not null"`
	Description   string    `json:"description" gorm:"size:500"`
	LeaderID      string    `json:"leaderId" gorm:"size:40;index"`
	LeaderName    string    `json:"leaderName" gorm:"-"`
	Status        string    `json:"status" gorm:"size:16;not null;default:enabled;index"`
	EmployeeCount int64     `json:"employeeCount" gorm:"-"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type Position struct {
	ID              string    `json:"id" gorm:"size:40;primaryKey"`
	Code            string    `json:"code" gorm:"size:32;uniqueIndex;not null"`
	Name            string    `json:"name" gorm:"size:100;uniqueIndex;not null"`
	Description     string    `json:"description" gorm:"size:500"`
	Status          string    `json:"status" gorm:"size:16;not null;default:enabled;index"`
	Builtin         bool      `json:"builtin" gorm:"not null;default:false"`
	DepartmentIDs   []string  `json:"departmentIds" gorm:"-"`
	DepartmentNames []string  `json:"departmentNames" gorm:"-"`
	EmployeeCount   int64     `json:"employeeCount" gorm:"-"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type DepartmentPosition struct {
	DepartmentID string    `json:"departmentId" gorm:"size:40;primaryKey"`
	PositionID   string    `json:"positionId" gorm:"size:40;primaryKey;index"`
	CreatedAt    time.Time `json:"createdAt"`
}

type DepartureRequest struct {
	ID                   string     `json:"id" gorm:"size:40;primaryKey"`
	EmployeeID           uint       `json:"-" gorm:"not null;index"`
	EmployeePublicID     string     `json:"employeeId" gorm:"size:40;not null;index"`
	EmployeeName         string     `json:"employeeName" gorm:"size:100;not null"`
	EmployeeNo           uint       `json:"employeeNo" gorm:"not null"`
	DepartmentID         string     `json:"departmentId" gorm:"size:40;not null;index"`
	DepartmentName       string     `json:"departmentName" gorm:"size:100;not null"`
	DepartmentLeaderID   string     `json:"departmentLeaderId" gorm:"size:40;not null;index"`
	Reason               string     `json:"reason" gorm:"size:1000;not null"`
	LastWorkingDate      string     `json:"lastWorkingDate" gorm:"size:10;not null"`
	Status               string     `json:"status" gorm:"size:24;not null;index"`
	ManagerReviewerID    string     `json:"managerReviewerId" gorm:"size:40"`
	ManagerReviewerName  string     `json:"managerReviewerName" gorm:"size:100"`
	ManagerReviewComment string     `json:"managerReviewComment" gorm:"size:500"`
	ManagerReviewedAt    *time.Time `json:"managerReviewedAt"`
	HRReviewerID         string     `json:"hrReviewerId" gorm:"size:40"`
	HRReviewerName       string     `json:"hrReviewerName" gorm:"size:100"`
	HRReviewComment      string     `json:"hrReviewComment" gorm:"size:500"`
	HRReviewedAt         *time.Time `json:"hrReviewedAt"`
	CanManagerReview     bool       `json:"canManagerReview" gorm:"-"`
	CanHRReview          bool       `json:"canHrReview" gorm:"-"`
	CanCancel            bool       `json:"canCancel" gorm:"-"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
}

type Notification struct {
	ID           string     `json:"id" gorm:"size:40;primaryKey"`
	RecipientID  string     `json:"recipientId" gorm:"size:40;not null;index"`
	Type         string     `json:"type" gorm:"size:40;not null;index"`
	Title        string     `json:"title" gorm:"size:160;not null"`
	Content      string     `json:"content" gorm:"size:500"`
	ResourceType string     `json:"resourceType" gorm:"size:40;not null"`
	ResourceID   string     `json:"resourceId" gorm:"size:40;not null;index"`
	ReadAt       *time.Time `json:"readAt" gorm:"index"`
	CreatedAt    time.Time  `json:"createdAt"`
}

type Session struct {
	ID         uint      `gorm:"primaryKey"`
	EmployeeID uint      `gorm:"not null;index"`
	Employee   Employee  `gorm:"constraint:OnDelete:CASCADE"`
	TokenHash  string    `gorm:"size:64;uniqueIndex;not null"`
	ExpiresAt  time.Time `gorm:"not null;index"`
	RevokedAt  *time.Time
	CreatedAt  time.Time
}

type OAuthClient struct {
	ID            uint   `gorm:"primaryKey"`
	ClientID      string `gorm:"size:64;uniqueIndex;not null"`
	Name          string `gorm:"size:100;not null"`
	SecretHash    string `gorm:"size:64;not null"`
	RedirectURIs  string `gorm:"type:text;not null"`
	AllowedScopes string `gorm:"type:text;not null"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type OAuthCode struct {
	ID          uint      `gorm:"primaryKey"`
	CodeHash    string    `gorm:"size:64;uniqueIndex;not null"`
	ClientID    string    `gorm:"size:64;not null;index"`
	EmployeeID  uint      `gorm:"not null;index"`
	RedirectURI string    `gorm:"size:500;not null"`
	ExpiresAt   time.Time `gorm:"not null;index"`
	UsedAt      *time.Time
	CreatedAt   time.Time
}

type OAuthToken struct {
	ID         uint      `gorm:"primaryKey"`
	TokenHash  string    `gorm:"size:64;uniqueIndex;not null"`
	ClientID   string    `gorm:"size:64;not null;index"`
	EmployeeID uint      `gorm:"not null;default:0;index"`
	Scope      string    `gorm:"type:text;not null"`
	ExpiresAt  time.Time `gorm:"not null;index"`
	CreatedAt  time.Time
}
