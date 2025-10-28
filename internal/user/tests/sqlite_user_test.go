package tests

import (
	"context"
	"log"
	"testing"
	"time"

	"simple-securities/internal/user/application/enum"
	"simple-securities/internal/user/domain/model"
	"simple-securities/internal/user/infras/repo"
	"simple-securities/pkg/db/sqlite"

	"github.com/stretchr/testify/assert"
)

func setupTestRepo(t *testing.T) (*repo.UserRepo, func()) {
	ctx := context.Background()

	db, err := sqlite.NewSQLiteClient()
	if err != nil {
		t.Fatalf("Failed to connect SQLite: %v", err)
	}

	db.AutoMigrate([]string{
		"migrations/000002_init_userdb.up.sql",
	})

	cleanup := func() {
		// Drop the table or close connection
		db.Close(ctx)
	}

	userRepo := repo.NewUserRepo(db.DB)
	return userRepo.(*repo.UserRepo), cleanup
}

func TestUserRepo_CRUD(t *testing.T) {
	ctx := context.Background()
	userRepo, cleanup := setupTestRepo(t)
	defer cleanup()

	now := time.Now()

	// 1️⃣ Create user
	newUser, err := model.NewUser("test_user@example.com", "abc123")
	assert.NoError(t, err)
	saved, err := userRepo.Save(ctx, newUser)
	assert.NoError(t, err)
	assert.NotZero(t, saved.ID)
	assert.Equal(t, enum.StatusPending, saved.Status)

	// 2️⃣ Find by Email
	foundByEmail, err := userRepo.FindByEmail(ctx, saved.Email)
	assert.NoError(t, err)
	assert.Equal(t, saved.Email, foundByEmail.Email)

	// 3️⃣ Find by ID
	foundById, err := userRepo.FindById(ctx, saved.ID)
	assert.NoError(t, err)
	assert.Equal(t, saved.Uuid, foundById.Uuid)

	// 4️⃣ Find by UUID
	foundByUuid, err := userRepo.FindByUuid(ctx, saved.Uuid)
	assert.NoError(t, err)
	assert.Equal(t, saved.Email, foundByUuid.Email)

	// 5️⃣ Update user
	saved.Email = "updated_" + saved.Email
	saved.Status = enum.StatusActive
	saved.LastLoginAt = &now
	saved.UpdatedAt = now
	saved.UpdatedBy = 0

	updated, err := userRepo.Save(ctx, saved)
	assert.NoError(t, err)
	assert.Equal(t, enum.StatusActive, updated.Status)

	// 6️⃣ FindByIdIn
	list, err := userRepo.FindByIdIn(ctx, []uint64{saved.ID})
	assert.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, updated.Email, list[0].Email)

	// 7️⃣ SaveAll (insert + update)
	user1, _ := model.NewUser("bulk1@example.com", "pw1")
	user2, _ := model.NewUser("bulk2@example.com", "pw2")
	users := []*model.User{user1, user2}

	inserted, err := userRepo.SaveAll(ctx, users)
	assert.NoError(t, err)
	assert.Len(t, inserted, 2)

	inserted[0].Email = "updated_bulk1@example.com"
	inserted[1].Email = "updated_bulk2@example.com"
	inserted[0].UpdatedAt = time.Now()
	inserted[1].UpdatedAt = time.Now()

	updatedBulk, err := userRepo.SaveAll(ctx, inserted)
	assert.NoError(t, err)
	assert.Equal(t, "updated_bulk1@example.com", updatedBulk[0].Email)
	assert.Equal(t, "updated_bulk2@example.com", updatedBulk[1].Email)

	// 8️⃣ Verify updated bulk
	for _, u := range updatedBulk {
		found, err := userRepo.FindByEmail(ctx, u.Email)
		assert.NoError(t, err)
		assert.NotNil(t, found)
		assert.Equal(t, u.Email, found.Email)
	}

	log.Println("✅ All UserRepo methods passed successfully")
}
