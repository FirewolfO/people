package store

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

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
	if err := db.AutoMigrate(&model.Employee{}, &model.Session{}, &model.OAuthClient{}, &model.OAuthCode{}, &model.OAuthToken{}); err != nil {
		return nil, err
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("admin"), 12)
	if err != nil {
		return nil, err
	}
	admin := model.Employee{
		PublicID: "people-admin", EmployeeNo: "ADMIN", Username: "admin", DisplayName: "系统管理员",
		Role: model.RoleAdmin, Status: model.StatusEnabled, PasswordHash: string(passwordHash), MustChangePassword: false,
	}
	adminCreate := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "username"}}, DoNothing: true}).
		Select("PublicID", "EmployeeNo", "Username", "DisplayName", "Role", "Status", "PasswordHash", "MustChangePassword").
		Create(&admin)
	if adminCreate.Error != nil {
		return nil, adminCreate.Error
	}
	if adminCreate.RowsAffected == 1 {
		if err := db.Model(&model.Employee{}).Where("id = ?", admin.ID).Update("must_change_password", false).Error; err != nil {
			return nil, err
		}
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
