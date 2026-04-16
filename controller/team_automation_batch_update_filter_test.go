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

func TestBatchUpdateTeamAutomationAccountsByFilter(t *testing.T) {
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

	target := &model.AutomationAccount{
		Name:   "batch-target-account",
		Email:  "target@example.com",
		Status: "pending",
	}
	other := &model.AutomationAccount{
		Name:   "non-target-account",
		Email:  "other@example.com",
		Status: "failed",
	}
	if err := target.Insert(); err != nil {
		t.Fatalf("failed to insert target account: %v", err)
	}
	if err := other.Insert(); err != nil {
		t.Fatalf("failed to insert other account: %v", err)
	}

	setTeamAutomationOptions(t, map[string]string{
		"AutomationMode": "local",
	})

	payload := map[string]any{
		"filter_status": "pending",
		"search":        "batch-target",
		"status":        "completed",
		"redeem_site":   "teamxz.store",
		"metadata":      "filter-updated",
	}
	raw, _ := json.Marshal(payload)
	ctx, recorder := newTeamAutomationTestContext(http.MethodPost, "/api/team-automation/accounts/batch-update", bytes.NewReader(raw))
	BatchUpdateTeamAutomationAccounts(ctx)

	response := decodeTeamAutomationAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	reloadedTarget, err := model.GetAutomationAccountByID(target.Id)
	if err != nil {
		t.Fatalf("failed to reload target account: %v", err)
	}
	if reloadedTarget.Status != "completed" {
		t.Fatalf("unexpected updated status: %q", reloadedTarget.Status)
	}
	if reloadedTarget.RedeemSite != "teamxz.store" {
		t.Fatalf("unexpected updated redeem site: %q", reloadedTarget.RedeemSite)
	}
	if reloadedTarget.Metadata != "filter-updated" {
		t.Fatalf("unexpected updated metadata: %q", reloadedTarget.Metadata)
	}

	reloadedOther, err := model.GetAutomationAccountByID(other.Id)
	if err != nil {
		t.Fatalf("failed to reload other account: %v", err)
	}
	if reloadedOther.Status != "failed" {
		t.Fatalf("unexpected other account status: %q", reloadedOther.Status)
	}
	if reloadedOther.Metadata != "" {
		t.Fatalf("unexpected other account metadata: %q", reloadedOther.Metadata)
	}
}

func TestBatchUpdateTeamAutomationTeamsByFilter(t *testing.T) {
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

	target := &model.AutomationTeam{
		Email:       "batch-target-team@example.com",
		AccessToken: "access-token",
		TeamName:    "batch-target",
		MaxMembers:  6,
		Status:      model.AutomationTeamStatusActive,
	}
	other := &model.AutomationTeam{
		Email:       "other-team@example.com",
		AccessToken: "access-token",
		TeamName:    "other",
		MaxMembers:  6,
		Status:      model.AutomationTeamStatusExpired,
	}
	if err := target.Insert(); err != nil {
		t.Fatalf("failed to insert target team: %v", err)
	}
	if err := other.Insert(); err != nil {
		t.Fatalf("failed to insert other team: %v", err)
	}

	setTeamAutomationOptions(t, map[string]string{
		"AutomationMode": "local",
	})

	payload := map[string]any{
		"filter_status": model.AutomationTeamStatusActive,
		"search":        "batch-target",
		"status":        model.AutomationTeamStatusDisabled,
		"metadata":      "team-filter-updated",
	}
	raw, _ := json.Marshal(payload)
	ctx, recorder := newTeamAutomationTestContext(http.MethodPost, "/api/team-automation/teams/batch-update", bytes.NewReader(raw))
	BatchUpdateTeamAutomationTeams(ctx)

	response := decodeTeamAutomationAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	reloadedTarget, err := model.GetAutomationTeamByID(target.Id)
	if err != nil {
		t.Fatalf("failed to reload target team: %v", err)
	}
	if reloadedTarget.Status != model.AutomationTeamStatusDisabled {
		t.Fatalf("unexpected updated team status: %q", reloadedTarget.Status)
	}
	if reloadedTarget.Metadata != "team-filter-updated" {
		t.Fatalf("unexpected updated team metadata: %q", reloadedTarget.Metadata)
	}

	reloadedOther, err := model.GetAutomationTeamByID(other.Id)
	if err != nil {
		t.Fatalf("failed to reload other team: %v", err)
	}
	if reloadedOther.Status != model.AutomationTeamStatusExpired {
		t.Fatalf("unexpected other team status: %q", reloadedOther.Status)
	}
	if reloadedOther.Metadata != "" {
		t.Fatalf("unexpected other team metadata: %q", reloadedOther.Metadata)
	}
}
