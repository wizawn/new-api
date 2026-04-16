package service

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

type AutomationTeamImportRecord struct {
	Email                 string
	AccessToken           string
	RefreshToken          string
	SessionToken          string
	ClientID              string
	AccountID             string
	TeamName              string
	PlanType              string
	SubscriptionPlan      string
	ExpiresAt             int64
	CurrentMembers        int
	PendingInvites        int
	MaxMembers            int
	Status                string
	AccountRole           string
	DeviceCodeAuthEnabled bool
	ErrorCount            int
	LastSyncTime          int64
	Metadata              string
}

func normalizeAutomationTeamState(team *model.AutomationTeam) {
	now := common.GetTimestamp()
	switch team.Status {
	case model.AutomationTeamStatusDisabled, model.AutomationTeamStatusBanned, model.AutomationTeamStatusError:
		return
	}
	if team.ExpiresAt > 0 && team.ExpiresAt <= now {
		team.Status = model.AutomationTeamStatusExpired
		return
	}
	if team.RemainingSlots() <= 0 {
		team.Status = model.AutomationTeamStatusFull
		return
	}
	team.Status = model.AutomationTeamStatusActive
}

func FillAutomationTeamClaims(team *model.AutomationTeam) {
	if team == nil {
		return
	}
	token := strings.TrimSpace(team.AccessToken)
	if token != "" {
		if team.Email == "" {
			if email, ok := ExtractEmailFromJWT(token); ok {
				team.Email = strings.TrimSpace(strings.ToLower(email))
			}
		}
		if team.AccountID == "" {
			if accountID, ok := ExtractCodexAccountIDFromJWT(token); ok {
				team.AccountID = strings.TrimSpace(accountID)
			}
		}
	}
	normalizeAutomationTeamState(team)
}

func BuildAutomationTeamUpsert(record AutomationTeamImportRecord) *model.AutomationTeam {
	team := &model.AutomationTeam{
		Email:                 strings.TrimSpace(strings.ToLower(record.Email)),
		AccessToken:           strings.TrimSpace(record.AccessToken),
		RefreshToken:          strings.TrimSpace(record.RefreshToken),
		SessionToken:          strings.TrimSpace(record.SessionToken),
		ClientID:              strings.TrimSpace(record.ClientID),
		AccountID:             strings.TrimSpace(record.AccountID),
		TeamName:              strings.TrimSpace(record.TeamName),
		PlanType:              strings.TrimSpace(strings.ToLower(record.PlanType)),
		SubscriptionPlan:      strings.TrimSpace(record.SubscriptionPlan),
		ExpiresAt:             record.ExpiresAt,
		CurrentMembers:        record.CurrentMembers,
		PendingInvites:        record.PendingInvites,
		MaxMembers:            record.MaxMembers,
		Status:                strings.TrimSpace(strings.ToLower(record.Status)),
		AccountRole:           strings.TrimSpace(strings.ToLower(record.AccountRole)),
		DeviceCodeAuthEnabled: record.DeviceCodeAuthEnabled,
		ErrorCount:            record.ErrorCount,
		LastSyncTime:          record.LastSyncTime,
		Metadata:              strings.TrimSpace(record.Metadata),
	}
	if team.MaxMembers <= 0 {
		team.MaxMembers = 6
	}
	if team.Status == "" {
		team.Status = model.AutomationTeamStatusActive
	}
	FillAutomationTeamClaims(team)
	team.Normalize()
	return team
}

func findExistingAutomationTeamForUpsert(upsert *model.AutomationTeam) (*model.AutomationTeam, error) {
	if upsert == nil {
		return nil, gorm.ErrRecordNotFound
	}
	lookups := []func() (*model.AutomationTeam, error){
		func() (*model.AutomationTeam, error) {
			if strings.TrimSpace(upsert.Email) == "" {
				return nil, gorm.ErrRecordNotFound
			}
			return model.GetAutomationTeamByEmail(upsert.Email)
		},
		func() (*model.AutomationTeam, error) {
			if strings.TrimSpace(upsert.AccountID) == "" {
				return nil, gorm.ErrRecordNotFound
			}
			return model.GetAutomationTeamByAccountID(upsert.AccountID)
		},
		func() (*model.AutomationTeam, error) {
			if strings.TrimSpace(upsert.ClientID) == "" {
				return nil, gorm.ErrRecordNotFound
			}
			return model.GetAutomationTeamByClientID(upsert.ClientID)
		},
	}
	for _, lookup := range lookups {
		existing, err := lookup()
		if err == nil {
			return existing, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func ImportAutomationTeams(records []AutomationTeamImportRecord) (map[string]any, error) {
	created := 0
	updated := 0
	failed := 0
	createdIDs := make([]int, 0)
	updatedIDs := make([]int, 0)
	errorsList := make([]string, 0)
	importBatch := "team_" + strconv.FormatInt(time.Now().Unix(), 10)

	for index, record := range records {
		upsert := BuildAutomationTeamUpsert(record)
		if upsert.Metadata == "" {
			upsert.Metadata = importBatch
		} else if !strings.Contains(upsert.Metadata, importBatch) {
			upsert.Metadata = strings.TrimSpace(upsert.Metadata + "\n" + importBatch)
		}
		if upsert.Email == "" && strings.TrimSpace(upsert.AccountID) == "" && strings.TrimSpace(upsert.ClientID) == "" {
			failed++
			errorsList = append(errorsList, fmt.Sprintf("第 %d 条记录缺少 email 且无法从 access_token 解析", index+1))
			continue
		}
		existing, err := findExistingAutomationTeamForUpsert(upsert)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			failed++
			identity := upsert.Email
			if identity == "" {
				identity = upsert.AccountID
			}
			if identity == "" {
				identity = upsert.ClientID
			}
			errorsList = append(errorsList, fmt.Sprintf("%s 查询失败: %v", identity, err))
			continue
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if upsert.Email == "" {
				failed++
				errorsList = append(errorsList, fmt.Sprintf("第 %d 条记录缺少 email，且无法通过 account_id/client_id 命中已有 Team", index+1))
				continue
			}
			if insertErr := upsert.Insert(); insertErr != nil {
				failed++
				errorsList = append(errorsList, fmt.Sprintf("%s 创建失败: %v", upsert.Email, insertErr))
				continue
			}
			created++
			createdIDs = append(createdIDs, upsert.Id)
			continue
		}

		if upsert.Email == "" {
			upsert.Email = existing.Email
		}
		existing.Email = upsert.Email
		existing.AccessToken = upsert.AccessToken
		existing.RefreshToken = upsert.RefreshToken
		existing.SessionToken = upsert.SessionToken
		existing.ClientID = upsert.ClientID
		existing.AccountID = upsert.AccountID
		existing.TeamName = upsert.TeamName
		existing.PlanType = upsert.PlanType
		existing.SubscriptionPlan = upsert.SubscriptionPlan
		existing.ExpiresAt = upsert.ExpiresAt
		existing.CurrentMembers = upsert.CurrentMembers
		existing.PendingInvites = upsert.PendingInvites
		existing.MaxMembers = upsert.MaxMembers
		existing.Status = upsert.Status
		existing.AccountRole = upsert.AccountRole
		existing.DeviceCodeAuthEnabled = upsert.DeviceCodeAuthEnabled
		existing.ErrorCount = upsert.ErrorCount
		existing.LastSyncTime = upsert.LastSyncTime
		existing.Metadata = upsert.Metadata
		FillAutomationTeamClaims(existing)
		if updateErr := existing.Update(); updateErr != nil {
			failed++
			errorsList = append(errorsList, fmt.Sprintf("%s 更新失败: %v", upsert.Email, updateErr))
			continue
		}
		updated++
		updatedIDs = append(updatedIDs, existing.Id)
	}

	sort.Strings(errorsList)
	if len(errorsList) > 50 {
		errorsList = errorsList[:50]
	}
	return map[string]any{
		"created":      created,
		"updated":      updated,
		"failed":       failed,
		"errors":       errorsList,
		"import_batch": importBatch,
		"created_ids":  createdIDs,
		"updated_ids":  updatedIDs,
	}, nil
}

func SelectAutomationTeam(excludeIDs []int) (*model.AutomationTeam, []*model.AutomationTeam, error) {
	list, _, err := model.ListAutomationTeams("", model.AutomationTeamStatusActive, true, 0, 0)
	if err != nil {
		return nil, nil, err
	}
	if len(excludeIDs) > 0 {
		filtered := make([]*model.AutomationTeam, 0, len(list))
		excluded := make(map[int]struct{}, len(excludeIDs))
		for _, id := range excludeIDs {
			excluded[id] = struct{}{}
		}
		for _, item := range list {
			if _, ok := excluded[item.Id]; ok {
				continue
			}
			filtered = append(filtered, item)
		}
		list = filtered
	}
	sort.SliceStable(list, func(i, j int) bool {
		left := list[i]
		right := list[j]
		if left.OccupiedSlots() != right.OccupiedSlots() {
			return left.OccupiedSlots() < right.OccupiedSlots()
		}
		if left.RemainingSlots() != right.RemainingSlots() {
			return left.RemainingSlots() > right.RemainingSlots()
		}
		return left.CreatedTime > right.CreatedTime
	})
	if len(list) == 0 {
		return nil, list, nil
	}
	return list[0], list, nil
}
