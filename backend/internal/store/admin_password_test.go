package store

import (
	"path/filepath"
	"testing"

	"people/internal/model"

	"golang.org/x/crypto/bcrypt"
)

func TestOpenSeedsAndMigratesDefaultAdministratorPassword(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "people.db")
	database, err := Open(dsn, "permission-ui", "permission-secret", []string{"http://localhost/callback"})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	var administrator model.Employee
	if err := database.DB.Where("username = ?", "admin").First(&administrator).Error; err != nil {
		t.Fatalf("find administrator: %v", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(administrator.PasswordHash), []byte("admin123!")) != nil {
		t.Fatal("new administrator does not use the expected default password")
	}

	legacyHash, err := bcrypt.GenerateFromPassword([]byte("admin"), 12)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Model(&administrator).Update("password_hash", string(legacyHash)).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	database, err = Open(dsn, "permission-ui", "permission-secret", []string{"http://localhost/callback"})
	if err != nil {
		t.Fatalf("reopen legacy database: %v", err)
	}
	defer database.Close()
	if err := database.DB.Where("username = ?", "admin").First(&administrator).Error; err != nil {
		t.Fatal(err)
	}
	if bcrypt.CompareHashAndPassword([]byte(administrator.PasswordHash), []byte("admin123!")) != nil {
		t.Fatal("legacy administrator password was not migrated")
	}

	customHash, err := bcrypt.GenerateFromPassword([]byte("custom-password"), 12)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Model(&administrator).Update("password_hash", string(customHash)).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	database, err = Open(dsn, "permission-ui", "permission-secret", []string{"http://localhost/callback"})
	if err != nil {
		t.Fatalf("reopen database with custom password: %v", err)
	}
	defer database.Close()
	if err := database.DB.Where("username = ?", "admin").First(&administrator).Error; err != nil {
		t.Fatal(err)
	}
	if bcrypt.CompareHashAndPassword([]byte(administrator.PasswordHash), []byte("custom-password")) != nil {
		t.Fatal("custom administrator password was overwritten")
	}
}
