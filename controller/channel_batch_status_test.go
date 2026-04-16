package controller

import (
	"bytes"
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

type channelBatchStatusAPIResponse struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message"`
	Data    channelBatchStatusData `json:"data"`
}

type channelBatchStatusData struct {
	UpdatedCount int               `json:"updated_count"`
	FailedCount  int               `json:"failed_count"`
	FailedIDs    []int             `json:"failed_ids"`
	Status       int               `json:"status"`
	FailedReason map[string]string `json:"failed_reasons"`
}

func setupChannelBatchStatusTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.MemoryCacheEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db

	if err := db.AutoMigrate(&model.Channel{}, &model.Ability{}); err != nil {
		t.Fatalf("failed to migrate channel tables: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func seedChannelForBatchStatus(t *testing.T, db *gorm.DB, status int, name string) *model.Channel {
	t.Helper()

	priority := int64(0)
	weight := uint(0)
	channel := &model.Channel{
		Type:         1,
		Key:          "test-key",
		Status:       status,
		Name:         name,
		Group:        "default",
		Models:       "gpt-4o-mini",
		Priority:     &priority,
		Weight:       &weight,
		CreatedTime:  common.GetTimestamp(),
		TestTime:     0,
		ResponseTime: 0,
	}
	if err := db.Create(channel).Error; err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}
	if err := channel.UpdateAbilities(nil); err != nil {
		t.Fatalf("failed to seed abilities: %v", err)
	}
	return channel
}

func TestBatchUpdateChannelStatus(t *testing.T) {
	db := setupChannelBatchStatusTestDB(t)
	enabled := seedChannelForBatchStatus(t, db, common.ChannelStatusEnabled, "enabled-channel")
	disabled := seedChannelForBatchStatus(t, db, common.ChannelStatusManuallyDisabled, "disabled-channel")

	body := []byte(fmt.Sprintf(`{"ids":[%d,%d],"status":%d}`, enabled.Id, disabled.Id, common.ChannelStatusManuallyDisabled))
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/channel/batch/status", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	BatchUpdateChannelStatus(ctx)

	var response channelBatchStatusAPIResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}
	if response.Data.UpdatedCount != 2 {
		t.Fatalf("expected 2 updated channels, got %d", response.Data.UpdatedCount)
	}
	if response.Data.FailedCount != 0 {
		t.Fatalf("expected 0 failed channels, got %d", response.Data.FailedCount)
	}

	var reloadedEnabled model.Channel
	if err := db.First(&reloadedEnabled, enabled.Id).Error; err != nil {
		t.Fatalf("failed to reload enabled channel: %v", err)
	}
	if reloadedEnabled.Status != common.ChannelStatusManuallyDisabled {
		t.Fatalf("expected channel status %d, got %d", common.ChannelStatusManuallyDisabled, reloadedEnabled.Status)
	}

	var reloadedDisabled model.Channel
	if err := db.First(&reloadedDisabled, disabled.Id).Error; err != nil {
		t.Fatalf("failed to reload disabled channel: %v", err)
	}
	if reloadedDisabled.Status != common.ChannelStatusManuallyDisabled {
		t.Fatalf("expected disabled channel to remain disabled, got %d", reloadedDisabled.Status)
	}
}
