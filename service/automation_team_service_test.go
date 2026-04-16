package service

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestImportAutomationTeamsMatchesExistingByAccountID(t *testing.T) {
	oldDB := model.DB
	defer func() {
		model.DB = oldDB
	}()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&model.AutomationTeam{}); err != nil {
		t.Fatalf("failed to migrate automation team table: %v", err)
	}
	model.DB = db

	existing := &model.AutomationTeam{
		Email:       "old@example.com",
		AccessToken: "legacy-token",
		AccountID:   "acct-1",
		ClientID:    "client-1",
		TeamName:    "legacy",
		MaxMembers:  6,
		Status:      model.AutomationTeamStatusActive,
	}
	if err := existing.Insert(); err != nil {
		t.Fatalf("failed to insert existing team: %v", err)
	}

	result, err := ImportAutomationTeams([]AutomationTeamImportRecord{
		{
			Email:       "new@example.com",
			AccessToken: "fresh-token",
			AccountID:   "acct-1",
			ClientID:    "client-1",
			TeamName:    "updated",
			MaxMembers:  8,
			Status:      model.AutomationTeamStatusActive,
		},
	})
	if err != nil {
		t.Fatalf("ImportAutomationTeams failed: %v", err)
	}
	if created := result["created"].(int); created != 0 {
		t.Fatalf("unexpected created count: %d", created)
	}
	if updated := result["updated"].(int); updated != 1 {
		t.Fatalf("unexpected updated count: %d", updated)
	}

	updatedTeam, err := model.GetAutomationTeamByEmail("new@example.com")
	if err != nil {
		t.Fatalf("expected updated team by new email: %v", err)
	}
	if updatedTeam.Id != existing.Id {
		t.Fatalf("expected existing team to be updated, got id=%d want=%d", updatedTeam.Id, existing.Id)
	}
	if updatedTeam.TeamName != "updated" {
		t.Fatalf("unexpected team name: %q", updatedTeam.TeamName)
	}
	_, err = model.GetAutomationTeamByEmail("old@example.com")
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected old email to be replaced, got err=%v", err)
	}
}
