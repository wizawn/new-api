package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type userAPIResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type userPageResponse struct {
	Items []userResponseItem `json:"items"`
}

type userResponseItem struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Status   int    `json:"status"`
	Role     int    `json:"role"`
}

func setupUserControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db

	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("failed to migrate user table: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func seedUser(t *testing.T, db *gorm.DB, username string, status int, role int, group string, deleted bool) *model.User {
	t.Helper()

	user := &model.User{
		Username:    username,
		Password:    "password123",
		DisplayName: username,
		Role:        role,
		Status:      status,
		Email:       username + "@example.com",
		Group:       group,
		AffCode:     "aff-" + username,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	if deleted {
		if err := db.Delete(user).Error; err != nil {
			t.Fatalf("failed to soft delete user: %v", err)
		}
	}
	return user
}

func newUserContext(t *testing.T, target string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, target, nil)
	return ctx, recorder
}

func decodeUserAPIResponse(t *testing.T, recorder *httptest.ResponseRecorder) userAPIResponse {
	t.Helper()

	var response userAPIResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode api response: %v", err)
	}
	return response
}

func TestGetAllUsersSupportsStatusFilter(t *testing.T) {
	db := setupUserControllerTestDB(t)
	seedUser(t, db, "enabled-user", common.UserStatusEnabled, common.RoleCommonUser, "default", false)
	disabled := seedUser(t, db, "disabled-user", common.UserStatusDisabled, common.RoleCommonUser, "default", false)
	seedUser(t, db, "deleted-user", common.UserStatusEnabled, common.RoleCommonUser, "default", true)

	ctx, recorder := newUserContext(t, "/api/user/?p=1&page_size=10&status=disabled")
	GetAllUsers(ctx)

	response := decodeUserAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var page userPageResponse
	if err := common.Unmarshal(response.Data, &page); err != nil {
		t.Fatalf("failed to decode user page response: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected exactly one disabled user, got %d", len(page.Items))
	}
	if page.Items[0].ID != disabled.Id {
		t.Fatalf("expected disabled user id %d, got %d", disabled.Id, page.Items[0].ID)
	}
}

func TestSearchUsersSupportsRoleFilter(t *testing.T) {
	db := setupUserControllerTestDB(t)
	seedUser(t, db, "match-common", common.UserStatusEnabled, common.RoleCommonUser, "default", false)
	admin := seedUser(t, db, "match-admin", common.UserStatusEnabled, common.RoleAdminUser, "default", false)
	seedUser(t, db, "other-root", common.UserStatusEnabled, common.RoleRootUser, "default", false)

	ctx, recorder := newUserContext(t, "/api/user/search?keyword=match&role=admin&p=1&page_size=10")
	SearchUsers(ctx)

	response := decodeUserAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var page userPageResponse
	if err := common.Unmarshal(response.Data, &page); err != nil {
		t.Fatalf("failed to decode user search response: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected exactly one admin user, got %d", len(page.Items))
	}
	if page.Items[0].ID != admin.Id {
		t.Fatalf("expected admin user id %d, got %d", admin.Id, page.Items[0].ID)
	}
	if page.Items[0].Role != common.RoleAdminUser {
		t.Fatalf("expected admin role %d, got %d", common.RoleAdminUser, page.Items[0].Role)
	}
}
