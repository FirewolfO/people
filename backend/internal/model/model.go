package model

import "time"

const (
	StatusEnabled  = "enabled"
	StatusDisabled = "disabled"
	RoleAdmin      = "admin"
	RoleEmployee   = "employee"
)

type Employee struct {
	ID                 uint       `json:"-" gorm:"primaryKey"`
	PublicID           string     `json:"id" gorm:"size:40;uniqueIndex;not null"`
	EmployeeNo         string     `json:"employeeNo" gorm:"size:32;uniqueIndex;not null"`
	Username           string     `json:"username" gorm:"size:64;uniqueIndex;not null"`
	DisplayName        string     `json:"displayName" gorm:"size:100;not null"`
	Email              string     `json:"email" gorm:"size:255"`
	Phone              string     `json:"phone" gorm:"size:32"`
	DepartmentID       string     `json:"departmentId" gorm:"size:40;index"`
	Department         string     `json:"department" gorm:"size:100"`
	Title              string     `json:"title" gorm:"size:100"`
	Role               string     `json:"role" gorm:"size:16;not null;default:employee"`
	Status             string     `json:"status" gorm:"size:16;not null;default:enabled;index"`
	PasswordHash       string     `json:"-" gorm:"size:255"`
	MustChangePassword bool       `json:"mustChangePassword" gorm:"not null;default:true"`
	PasswordChangedAt  *time.Time `json:"passwordChangedAt"`
	LastLoginAt        *time.Time `json:"lastLoginAt"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

type Department struct {
	ID            string    `json:"id" gorm:"size:40;primaryKey"`
	ParentID      string    `json:"parentId" gorm:"size:40;index"`
	Code          string    `json:"code" gorm:"size:32;uniqueIndex;not null"`
	Name          string    `json:"name" gorm:"size:100;uniqueIndex;not null"`
	Description   string    `json:"description" gorm:"size:500"`
	Status        string    `json:"status" gorm:"size:16;not null;default:enabled;index"`
	EmployeeCount int64     `json:"employeeCount" gorm:"-"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
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
