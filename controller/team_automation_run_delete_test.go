package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestBatchDeleteTeamAutomationRuns(t *testing.T) {
	oldDB := model.DB
	defer func() {
		model.DB = oldDB
	}()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&model.AutomationExecution{}, &model.AutomationExecutionLog{}, &model.AutomationArtifact{}); err != nil {
		t.Fatalf("failed to migrate automation tables: %v", err)
	}
	model.DB = db

	artifactRoot := t.TempDir()
	artifactPath := filepath.Join(artifactRoot, "exec-finished.json")
	if err := os.WriteFile(artifactPath, []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatalf("failed to seed artifact file: %v", err)
	}

	finished := &model.AutomationExecution{
		ExecutionID: "exec-finished",
		Module:      constant.AutomationModuleTeamRedeem,
		Status:      constant.AutomationExecutionStatusCompleted,
	}
	if err := finished.Insert(); err != nil {
		t.Fatalf("insert finished execution: %v", err)
	}
	running := &model.AutomationExecution{
		ExecutionID: "exec-running",
		Module:      constant.AutomationModuleBrowserLoginCapture,
		Status:      constant.AutomationExecutionStatusRunning,
	}
	if err := running.Insert(); err != nil {
		t.Fatalf("insert running execution: %v", err)
	}
	if err := (&model.AutomationExecutionLog{
		ExecutionID: finished.ExecutionID,
		Message:     "proxy retry scheduled",
	}).Insert(); err != nil {
		t.Fatalf("insert finished execution log: %v", err)
	}
	if err := (&model.AutomationArtifact{
		ExecutionID:  finished.ExecutionID,
		ArtifactType: "storage_state",
		Path:         artifactPath,
	}).Insert(); err != nil {
		t.Fatalf("insert finished artifact: %v", err)
	}

	setTeamAutomationOptions(t, map[string]string{
		"AutomationMode":         "local",
		"AutomationArtifactRoot": artifactRoot,
	})

	payload := map[string]any{
		"execution_ids":         []string{"exec-finished", "exec-running", "exec-missing"},
		"delete_artifact_files": true,
	}
	raw, _ := json.Marshal(payload)
	ctx, recorder := newTeamAutomationTestContext(http.MethodPost, "/api/team-automation/run/batch-delete", bytes.NewReader(raw))
	BatchDeleteTeamAutomationRuns(ctx)

	response := decodeTeamAutomationAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var result map[string]any
	if err := json.Unmarshal(response.Data, &result); err != nil {
		t.Fatalf("failed to decode delete result: %v", err)
	}
	if got := int(result["deleted_count"].(float64)); got != 1 {
		t.Fatalf("unexpected deleted_count: %d", got)
	}
	if got := int(result["skipped_active_count"].(float64)); got != 1 {
		t.Fatalf("unexpected skipped_active_count: %d", got)
	}
	if got := int(result["skipped_missing_count"].(float64)); got != 1 {
		t.Fatalf("unexpected skipped_missing_count: %d", got)
	}
	if got := int(result["deleted_artifact_file_count"].(float64)); got != 1 {
		t.Fatalf("unexpected deleted_artifact_file_count: %d", got)
	}

	if _, err := model.GetAutomationExecutionByExecutionID(finished.ExecutionID); err == nil {
		t.Fatal("expected finished execution to be deleted")
	}
	if _, err := model.GetAutomationExecutionByExecutionID(running.ExecutionID); err != nil {
		t.Fatalf("expected running execution to remain: %v", err)
	}

	var logCount int64
	if err := model.DB.Model(&model.AutomationExecutionLog{}).Where("execution_id = ?", finished.ExecutionID).Count(&logCount).Error; err != nil {
		t.Fatalf("count execution logs: %v", err)
	}
	if logCount != 0 {
		t.Fatalf("expected logs to be deleted, got %d", logCount)
	}

	var artifactCount int64
	if err := model.DB.Model(&model.AutomationArtifact{}).Where("execution_id = ?", finished.ExecutionID).Count(&artifactCount).Error; err != nil {
		t.Fatalf("count execution artifacts: %v", err)
	}
	if artifactCount != 0 {
		t.Fatalf("expected artifacts to be deleted, got %d", artifactCount)
	}

	if _, err := os.Stat(artifactPath); !os.IsNotExist(err) {
		t.Fatalf("expected artifact file to be removed, stat err=%v", err)
	}
}
