package controller

import (
	"encoding/json"
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

type logInsightsResponse struct {
	Success bool              `json:"success"`
	Message string            `json:"message"`
	Data    model.LogInsights `json:"data"`
}

func setupLogExportTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.MemoryCacheEnabled = false

	db, err := gorm.Open(sqlite.Open("file:log_export_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db

	if err := db.AutoMigrate(&model.Channel{}, &model.Log{}); err != nil {
		t.Fatalf("failed to migrate log export tables: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestExportAllLogsWritesCSV(t *testing.T) {
	db := setupLogExportTestDB(t)
	channel := &model.Channel{
		Id:     12,
		Type:   1,
		Key:    "test-key",
		Status: common.ChannelStatusEnabled,
		Name:   "export-channel",
		Group:  "default",
		Models: "gpt-4o-mini",
	}
	if err := db.Create(channel).Error; err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}
	logItem := &model.Log{
		UserId:       7,
		CreatedAt:    1234567890,
		Type:         model.LogTypeConsume,
		Content:      "hello export",
		Username:     "tester",
		TokenName:    "token-a",
		ModelName:    "gpt-4o-mini",
		Quota:        42,
		ChannelId:    channel.Id,
		Group:        "default",
		RequestId:    "req-1",
		Ip:           "127.0.0.1",
		PromptTokens: 10,
	}
	if err := db.Create(logItem).Error; err != nil {
		t.Fatalf("failed to create log: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/log/export?type=2", nil)

	ExportAllLogs(ctx)

	if got := recorder.Header().Get("Content-Disposition"); !strings.Contains(got, "usage_logs.csv") {
		t.Fatalf("expected csv disposition header, got %q", got)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "username,channel_id,channel_name,time,type") {
		t.Fatalf("expected admin csv header, got %q", body)
	}
	if !strings.Contains(body, "tester,12,export-channel") {
		t.Fatalf("expected exported admin row, got %q", body)
	}
}

func TestGetLogsInsightsReturnsAggregates(t *testing.T) {
	db := setupLogExportTestDB(t)
	channel := &model.Channel{
		Id:     13,
		Type:   1,
		Key:    "test-key",
		Status: common.ChannelStatusEnabled,
		Name:   "insight-channel",
		Group:  "default",
		Models: "gpt-4o-mini",
	}
	if err := db.Create(channel).Error; err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}
	items := []*model.Log{
		{
			UserId:    7,
			CreatedAt: 1234567891,
			Type:      model.LogTypeError,
			Content:   "rate limited",
			Username:  "tester",
			TokenName: "token-a",
			ModelName: "gpt-4o-mini",
			ChannelId: channel.Id,
			Group:     "default",
		},
		{
			UserId:    7,
			CreatedAt: 1234567892,
			Type:      model.LogTypeError,
			Content:   "rate limited",
			Username:  "tester",
			TokenName: "token-a",
			ModelName: "gpt-4o-mini",
			ChannelId: channel.Id,
			Group:     "default",
		},
	}
	for _, item := range items {
		if err := db.Create(item).Error; err != nil {
			t.Fatalf("failed to create log: %v", err)
		}
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/log/insights?type=5&limit=5", nil)

	GetLogsInsights(ctx)

	var response logInsightsResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode insights response: %v", err)
	}
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}
	if len(response.Data.Models) == 0 || response.Data.Models[0].Label != "gpt-4o-mini" {
		t.Fatalf("expected model hotspot, got %#v", response.Data.Models)
	}
	if len(response.Data.Contents) == 0 || response.Data.Contents[0].Label != "rate limited" {
		t.Fatalf("expected content hotspot, got %#v", response.Data.Contents)
	}
}
