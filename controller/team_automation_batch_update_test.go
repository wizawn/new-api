package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestBatchUpdateTeamAutomationAccounts(t *testing.T) {
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

	account := &model.AutomationAccount{
		Name:       "demo",
		Email:      "demo@example.com",
		Status:     "pending",
		RedeemSite: "helloteam.store",
	}
	if err := account.Insert(); err != nil {
		t.Fatalf("failed to insert account: %v", err)
	}

	setTeamAutomationOptions(t, map[string]string{
		"AutomationMode": "local",
	})

	body := bytes.NewBufferString(`{"ids":[1],"status":"completed","redeem_site":"teamxz.store","metadata":"batch-a"}`)
	ctx, recorder := newTeamAutomationTestContext(http.MethodPost, "/api/team-automation/accounts/batch-update", body)
	BatchUpdateTeamAutomationAccounts(ctx)

	response := decodeTeamAutomationAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	updated, err := model.GetAutomationAccountByID(account.Id)
	if err != nil {
		t.Fatalf("failed to reload account: %v", err)
	}
	if updated.Status != "completed" {
		t.Fatalf("unexpected updated status: %q", updated.Status)
	}
	if updated.RedeemSite != "teamxz.store" {
		t.Fatalf("unexpected updated redeem site: %q", updated.RedeemSite)
	}
	if updated.Metadata != "batch-a" {
		t.Fatalf("unexpected updated metadata: %q", updated.Metadata)
	}
}

func TestBatchUpdateTeamAutomationTeams(t *testing.T) {
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

	team := &model.AutomationTeam{
		Email:       "team@example.com",
		AccessToken: "access-token",
		MaxMembers:  6,
		Status:      model.AutomationTeamStatusActive,
	}
	if err := team.Insert(); err != nil {
		t.Fatalf("failed to insert team: %v", err)
	}

	setTeamAutomationOptions(t, map[string]string{
		"AutomationMode": "local",
	})

	payload := map[string]any{
		"ids":      []int{team.Id},
		"status":   model.AutomationTeamStatusDisabled,
		"metadata": "team-batch",
	}
	raw, _ := json.Marshal(payload)
	ctx, recorder := newTeamAutomationTestContext(http.MethodPost, "/api/team-automation/teams/batch-update", bytes.NewReader(raw))
	BatchUpdateTeamAutomationTeams(ctx)

	response := decodeTeamAutomationAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	updated, err := model.GetAutomationTeamByID(team.Id)
	if err != nil {
		t.Fatalf("failed to reload team: %v", err)
	}
	if updated.Status != model.AutomationTeamStatusDisabled {
		t.Fatalf("unexpected updated team status: %q", updated.Status)
	}
	if updated.Metadata != "team-batch" {
		t.Fatalf("unexpected updated team metadata: %q", updated.Metadata)
	}
}
