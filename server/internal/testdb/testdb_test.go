package testdb_test

import (
	"testing"

	"charity-chest/internal/model"
	"charity-chest/internal/testdb"
)

func TestOpen_FreshDBHasMigratedSchema(t *testing.T) {
	db := testdb.Open(t)

	// Insert + read back to prove the migrated schema is usable.
	user := model.User{Email: "a@example.com", Name: "A"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	if user.ID == 0 {
		t.Fatal("expected non-zero ID")
	}

	var got model.User
	if err := db.First(&got, user.ID).Error; err != nil {
		t.Fatalf("first: %v", err)
	}
	if got.Email != "a@example.com" {
		t.Errorf("email = %q, want a@example.com", got.Email)
	}
}

func TestOpen_TwoCalls_AreIsolated(t *testing.T) {
	db1 := testdb.Open(t)
	db2 := testdb.Open(t)

	if err := db1.Create(&model.User{Email: "iso@example.com", Name: "Iso"}).Error; err != nil {
		t.Fatalf("create on db1: %v", err)
	}

	var count int64
	if err := db2.Model(&model.User{}).Where("email = ?", "iso@example.com").Count(&count).Error; err != nil {
		t.Fatalf("count on db2: %v", err)
	}
	if count != 0 {
		t.Errorf("db2 saw %d rows; want 0 (databases must be isolated)", count)
	}
}
