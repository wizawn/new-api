package service

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

type ChatMockRouteLogInput struct {
	AccountID       int
	AccountName     string
	Model           string
	Strategy        string
	RouteScore      int64
	SelectionReason string
	LatencyMS       int64
	Success         bool
	FailoverReason  string
	TokenUsage      int64
	RequestID       string
}

func RecordChatMockRouteLog(input ChatMockRouteLogInput) {
	if input.AccountID <= 0 {
		return
	}
	log := &model.ChatMockRouteLog{
		AccountID:       input.AccountID,
		AccountName:     strings.TrimSpace(input.AccountName),
		Model:           strings.TrimSpace(input.Model),
		Strategy:        strings.TrimSpace(input.Strategy),
		RouteScore:      input.RouteScore,
		SelectionReason: strings.TrimSpace(input.SelectionReason),
		LatencyMS:       input.LatencyMS,
		Success:         input.Success,
		FailoverReason:  strings.TrimSpace(input.FailoverReason),
		TokenUsage:      input.TokenUsage,
		RequestID:       strings.TrimSpace(input.RequestID),
	}
	if err := log.Insert(); err != nil {
		common.SysError("记录 ChatMock route log 失败: " + err.Error())
	}
}

func RecordChatMockAccountSnapshot(accountID int, snapshotType string, status string, summary string, payload any) {
	if accountID <= 0 {
		return
	}
	rawPayload := ""
	if payload != nil {
		if b, err := common.Marshal(payload); err == nil {
			rawPayload = string(b)
		}
	}
	snapshot := &model.ChatMockAccountSnapshot{
		AccountID:    accountID,
		SnapshotType: strings.TrimSpace(strings.ToLower(snapshotType)),
		Status:       strings.TrimSpace(strings.ToLower(status)),
		Summary:      strings.TrimSpace(summary),
		Payload:      rawPayload,
	}
	if err := snapshot.Insert(); err != nil {
		common.SysError("记录 ChatMock snapshot 失败: " + err.Error())
	}
}
