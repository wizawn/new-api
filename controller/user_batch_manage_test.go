package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type userBatchManageResult struct {
	Action        string         `json:"action"`
	UpdatedCount  int            `json:"updated_count"`
	FailedCount   int            `json:"failed_count"`
	FailedIDs     []int          `json:"failed_ids"`
	FailedReasons map[int]string `json:"failed_reasons"`
}

func newAdminUserContext(t *testing.T, method string, target string, body []byte, role int) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("role", role)
	ctx.Set("id", 999)
	ctx.Set("username", "test-admin")
	return ctx, recorder
}

func TestBatchManageUsersSupportsPartialSuccess(t *testing.T) {
	db := setupUserControllerTestDB(t)
	enabledUser := seedUser(t, db, "enabled-batch-user", common.UserStatusEnabled, common.RoleCommonUser, "default", false)
	rootUser := seedUser(t, db, "root-batch-user", common.UserStatusEnabled, common.RoleRootUser, "default", false)

	body, err := common.Marshal(BatchManageUsersRequest{
		Ids:    []int{enabledUser.Id, rootUser.Id, 99999},
		Action: "disable",
	})
	if err != nil {
		t.Fatalf("marshal batch manage request: %v", err)
	}

	ctx, recorder := newAdminUserContext(t, http.MethodPost, "/api/user/manage/batch", body, common.RoleAdminUser)
	BatchManageUsers(ctx)

	response := decodeUserAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var result userBatchManageResult
	if err := common.Unmarshal(response.Data, &result); err != nil {
		t.Fatalf("failed to decode batch result: %v", err)
	}

	if result.Action != "disable" {
		t.Fatalf("expected action disable, got %s", result.Action)
	}
	if result.UpdatedCount != 1 {
		t.Fatalf("expected 1 updated user, got %d", result.UpdatedCount)
	}
	if result.FailedCount != 2 {
		t.Fatalf("expected 2 failed users, got %d", result.FailedCount)
	}
	if len(result.FailedIDs) != 2 {
		t.Fatalf("expected 2 failed ids, got %d", len(result.FailedIDs))
	}
	if result.FailedReasons[rootUser.Id] == "" {
		t.Fatalf("expected failure reason for root user")
	}
	if result.FailedReasons[99999] == "" {
		t.Fatalf("expected failure reason for missing user")
	}

	var refreshedUser model.User
	if err := db.Where("id = ?", enabledUser.Id).First(&refreshedUser).Error; err != nil {
		t.Fatalf("failed to load refreshed user: %v", err)
	}
	if refreshedUser.Status != common.UserStatusDisabled {
		t.Fatalf("expected enabled user to be disabled, got %d", refreshedUser.Status)
	}
}
