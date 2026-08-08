package model

import (
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

	DeparturePendingManager = "pending_manager"
	DeparturePendingHR      = "pending_hr"
	DepartureApproved       = "approved"
	DepartureRejected       = "rejected"
	DepartureCancelled      = "cancelled"
)

type Employee struct {
	ID                 uint       `json:"-" gorm:"primaryKey"`
	PublicID           string     `json:"id" gorm:"size:40;uniqueIndex;not null"`
	EmployeeNo         uint       `json:"employeeNo" gorm:"-"`
	LegacyEmployeeNo   string     `json:"-" gorm:"column:employee_no;size:64;uniqueIndex;not null"`
	Username           string     `json:"username" gorm:"size:64;uniqueIndex;not null"`
	DisplayName        string     `json:"displayName" gorm:"size:100;not null"`
	Email              string     `json:"email" gorm:"size:255"`
	Phone              string     `json:"phone" gorm:"size:32"`
	DepartmentID       string     `json:"departmentId" gorm:"size:40;index"`
	Department         string     `json:"department" gorm:"size:100"`
	Title              string     `json:"title" gorm:"size:100"`
	EmploymentType     string     `json:"employmentType" gorm:"size:24;not null;default:full_time"`
	HireDate           string     `json:"hireDate" gorm:"size:10"`
	ProbationEndDate   string     `json:"probationEndDate" gorm:"size:10"`
	WorkLocation       string     `json:"workLocation" gorm:"size:100"`
	Role               string     `json:"role" gorm:"size:16;not null;default:employee"`
	Status             string     `json:"status" gorm:"size:16;not null;default:enabled;index"`
	Permissions        []string   `json:"permissions" gorm:"-"`
	PasswordHash       string     `json:"-" gorm:"size:255"`
	MustChangePassword bool       `json:"mustChangePassword" gorm:"not null;default:true"`
	PasswordChangedAt  *time.Time `json:"passwordChangedAt"`
	LastLoginAt        *time.Time `json:"lastLoginAt"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
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
