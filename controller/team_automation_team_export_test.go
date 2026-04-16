package controller

import (
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestExportTeamAutomationTeamsCSVLocal(t *testing.T) {
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
		Email:                 "team@example.com",
		AccessToken:           "access-token",
		AccountID:             "acct-1",
		ClientID:              "client-1",
		TeamName:              "alpha",
		MaxMembers:            6,
		CurrentMembers:        2,
		PendingInvites:        1,
		Status:                model.AutomationTeamStatusActive,
		DeviceCodeAuthEnabled: true,
		Metadata:              "export-tag",
	}
	if err := team.Insert(); err != nil {
		t.Fatalf("failed to insert team: %v", err)
	}

	setTeamAutomationOptions(t, map[string]string{
		"AutomationMode": "local",
	})

	ctx, recorder := newTeamAutomationTestContext(http.MethodGet, "/api/team-automation/teams/export/csv?search=alpha", nil)
	ExportTeamAutomationTeamsCSV(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Disposition"); !strings.Contains(got, "automation_teams.csv") {
		t.Fatalf("unexpected content disposition: %q", got)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "team@example.com") {
		t.Fatalf("export body missing team email: %q", body)
	}
	if !strings.Contains(body, "client-1") {
		t.Fatalf("export body missing client id: %q", body)
	}
	if !strings.Contains(body, "export-tag") {
		t.Fatalf("export body missing metadata: %q", body)
	}
}
