package controller

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestBatchDeleteTeamAutomationAccountsByFilter(t *testing.T) {
	oldDB := model.DB
	defer func() {
		model.DB = oldDB
	}()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&model.AutomationAccount{}); err != nil {
		t.Fatalf("failed to migrate automation accounts: %v", err)
	}
	model.DB = db

	matched := &model.AutomationAccount{
		Name:   "failed-one",
		Email:  "failed-one@example.com",
		Status: constant.AutomationAccountStatusFailed,
	}
	if err := matched.Insert(); err != nil {
		t.Fatalf("insert matched account: %v", err)
	}
	kept := &model.AutomationAccount{
		Name:   "completed-one",
		Email:  "completed-one@example.com",
		Status: constant.AutomationAccountStatusCompleted,
	}
	if err := kept.Insert(); err != nil {
		t.Fatalf("insert kept account: %v", err)
	}

	setTeamAutomationOptions(t, map[string]string{
		"AutomationMode": "local",
	})

	body := bytes.NewBufferString(`{"status":"failed","search":"failed-one"}`)
	ctx, recorder := newTeamAutomationTestContext(http.MethodPost, "/api/team-automation/accounts/batch-delete", body)
	BatchDeleteTeamAutomationAccounts(ctx)

	response := decodeTeamAutomationAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}
	if _, err := model.GetAutomationAccountByID(matched.Id); err == nil {
		t.Fatal("expected matched account to be deleted")
	}
	if _, err := model.GetAutomationAccountByID(kept.Id); err != nil {
		t.Fatalf("expected kept account to remain: %v", err)
	}
}

func TestBatchDeleteTeamAutomationTeamsByFilter(t *testing.T) {
	oldDB := model.DB
	defer func() {
		model.DB = oldDB
	}()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&model.AutomationTeam{}); err != nil {
		t.Fatalf("failed to migrate automation teams: %v", err)
	}
	model.DB = db

	matched := &model.AutomationTeam{
		Email:       "disabled-team@example.com",
		AccessToken: "access-a",
		MaxMembers:  6,
		Status:      model.AutomationTeamStatusDisabled,
		Metadata:    "cleanup-batch",
	}
	if err := matched.Insert(); err != nil {
		t.Fatalf("insert matched team: %v", err)
	}
	kept := &model.AutomationTeam{
		Email:       "active-team@example.com",
		AccessToken: "access-b",
		MaxMembers:  6,
		Status:      model.AutomationTeamStatusActive,
	}
	if err := kept.Insert(); err != nil {
		t.Fatalf("insert kept team: %v", err)
	}

	setTeamAutomationOptions(t, map[string]string{
		"AutomationMode": "local",
	})

	body := bytes.NewBufferString(`{"status":"disabled","search":"cleanup-batch"}`)
	ctx, recorder := newTeamAutomationTestContext(http.MethodPost, "/api/team-automation/teams/batch-delete", body)
	BatchDeleteTeamAutomationTeams(ctx)

	response := decodeTeamAutomationAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}
	if _, err := model.GetAutomationTeamByID(matched.Id); err == nil {
		t.Fatal("expected matched team to be deleted")
	}
	if _, err := model.GetAutomationTeamByID(kept.Id); err != nil {
		t.Fatalf("expected kept team to remain: %v", err)
	}
}
