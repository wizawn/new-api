package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type chatMockAPIResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func TestAdminBatchDeleteChatMockAccountsByFilter(t *testing.T) {
	oldDB := model.DB
	oldEnabled := setting.ChatMockEnabled
	defer func() {
		model.DB = oldDB
		setting.ChatMockEnabled = oldEnabled
	}()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err = db.AutoMigrate(
		&model.ChatMockAccount{},
		&model.ChatMockAccountSnapshot{},
		&model.ChatMockRouteLog{},
	); err != nil {
		t.Fatalf("migrate chatmock tables: %v", err)
	}
	model.DB = db
	setting.ChatMockEnabled = true

	deleted := &model.ChatMockAccount{
		Name:            "disabled-a",
		Account:         "disabled-a@example.com",
		ApiKey:          "sk-disabled",
		BaseURL:         "https://chatgpt.com",
		Status:          constant.ChatMockAccountStatusDisabled,
		SupportedModels: "[]",
	}
	if err = deleted.Insert(); err != nil {
		t.Fatalf("insert deleted account: %v", err)
	}
	kept := &model.ChatMockAccount{
		Name:            "enabled-a",
		Account:         "enabled-a@example.com",
		ApiKey:          "sk-enabled",
		BaseURL:         "https://chatgpt.com",
		Status:          constant.ChatMockAccountStatusEnabled,
		SupportedModels: "[]",
	}
	if err = kept.Insert(); err != nil {
		t.Fatalf("insert kept account: %v", err)
	}
	if err = (&model.ChatMockAccountSnapshot{
		AccountID:    deleted.Id,
		SnapshotType: "health",
		Status:       "error",
		Summary:      "stale",
	}).Insert(); err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}
	if err = (&model.ChatMockRouteLog{
		AccountID:       deleted.Id,
		AccountName:     deleted.Name,
		Model:           "gpt-5.4",
		Strategy:        "weighted",
		SelectionReason: "request_error",
	}).Insert(); err != nil {
		t.Fatalf("insert route log: %v", err)
	}

	body := bytes.NewBufferString(`{"filters":{"status":"disabled"}}`)
	ctx, recorder := newChatMockTestContext(http.MethodPost, "/api/admin/chatmock/accounts/batch-delete", body)
	AdminBatchDeleteChatMockAccounts(ctx)

	response := decodeChatMockAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var result map[string]any
	if err = json.Unmarshal(response.Data, &result); err != nil {
		t.Fatalf("decode response data: %v", err)
	}
	if got := int(result["deleted_count"].(float64)); got != 1 {
		t.Fatalf("unexpected deleted_count: %d", got)
	}
	if got := int(result["deleted_snapshot_count"].(float64)); got != 1 {
		t.Fatalf("unexpected deleted_snapshot_count: %d", got)
	}
	if got := int(result["deleted_route_log_count"].(float64)); got != 1 {
		t.Fatalf("unexpected deleted_route_log_count: %d", got)
	}

	if _, err = model.GetChatMockAccountByID(deleted.Id); err == nil {
		t.Fatal("expected disabled account to be deleted")
	}
	if _, err = model.GetChatMockAccountByID(kept.Id); err != nil {
		t.Fatalf("expected enabled account to remain: %v", err)
	}
}

func TestAdminBatchSetChatMockAccountStatus(t *testing.T) {
	oldDB := model.DB
	oldEnabled := setting.ChatMockEnabled
	defer func() {
		model.DB = oldDB
		setting.ChatMockEnabled = oldEnabled
	}()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err = db.AutoMigrate(&model.ChatMockAccount{}); err != nil {
		t.Fatalf("migrate chatmock account: %v", err)
	}
	model.DB = db
	setting.ChatMockEnabled = true

	first := &model.ChatMockAccount{
		Name:            "first",
		Account:         "first@example.com",
		ApiKey:          "sk-first",
		BaseURL:         "https://chatgpt.com",
		Status:          constant.ChatMockAccountStatusDisabled,
		SupportedModels: "[]",
	}
	if err = first.Insert(); err != nil {
		t.Fatalf("insert first account: %v", err)
	}
	second := &model.ChatMockAccount{
		Name:            "second",
		Account:         "second@example.com",
		ApiKey:          "sk-second",
		BaseURL:         "https://chatgpt.com",
		Status:          constant.ChatMockAccountStatusDisabled,
		SupportedModels: "[]",
	}
	if err = second.Insert(); err != nil {
		t.Fatalf("insert second account: %v", err)
	}

	rawBody, err := json.Marshal(map[string]any{
		"ids":           []int{first.Id, second.Id},
		"target_status": "enabled",
	})
	if err != nil {
		t.Fatalf("marshal batch status body: %v", err)
	}
	body := bytes.NewBuffer(rawBody)
	ctx, recorder := newChatMockTestContext(http.MethodPost, "/api/admin/chatmock/accounts/batch-status", body)
	AdminBatchSetChatMockAccountStatus(ctx)

	response := decodeChatMockAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var result map[string]any
	if err = json.Unmarshal(response.Data, &result); err != nil {
		t.Fatalf("decode response data: %v", err)
	}
	if got := int(result["updated_count"].(float64)); got != 2 {
		t.Fatalf("unexpected updated_count: %d", got)
	}

	updatedFirst, err := model.GetChatMockAccountByID(first.Id)
	if err != nil {
		t.Fatalf("load first account: %v", err)
	}
	if updatedFirst.Status != constant.ChatMockAccountStatusEnabled {
		t.Fatalf("unexpected first status: %q", updatedFirst.Status)
	}
	updatedSecond, err := model.GetChatMockAccountByID(second.Id)
	if err != nil {
		t.Fatalf("load second account: %v", err)
	}
	if updatedSecond.Status != constant.ChatMockAccountStatusEnabled {
		t.Fatalf("unexpected second status: %q", updatedSecond.Status)
	}
}

func newChatMockTestContext(method string, target string, body *bytes.Buffer) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, body)
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx, recorder
}

func decodeChatMockAPIResponse(t *testing.T, recorder *httptest.ResponseRecorder) chatMockAPIResponse {
	t.Helper()
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d, body=%s", recorder.Code, recorder.Body.String())
	}
	response := chatMockAPIResponse{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode api response: %v", err)
	}
	return response
}
