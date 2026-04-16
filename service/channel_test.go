package service

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/types"
)

func TestShouldDisableChannelSkipsDistributorModelUnavailable(t *testing.T) {
	origin := common.AutomaticDisableChannelEnabled
	common.AutomaticDisableChannelEnabled = true
	defer func() {
		common.AutomaticDisableChannelEnabled = origin
	}()

	err := types.NewOpenAIError(
		fmt.Errorf("No available channel for model gpt-5.1-codex under group default (distributor)"),
		types.ErrorCodeBadResponse,
		http.StatusServiceUnavailable,
	)
	if ShouldDisableChannel(constant.ChannelTypeOpenAI, err) {
		t.Fatalf("distributor model unavailable should not trigger auto disable")
	}
}

func TestShouldDisableChannelStillHandlesUnauthorized(t *testing.T) {
	origin := common.AutomaticDisableChannelEnabled
	common.AutomaticDisableChannelEnabled = true
	defer func() {
		common.AutomaticDisableChannelEnabled = origin
	}()

	err := types.NewOpenAIError(
		fmt.Errorf("invalid api key"),
		types.ErrorCodeBadResponse,
		http.StatusUnauthorized,
	)
	if !ShouldDisableChannel(constant.ChannelTypeOpenAI, err) {
		t.Fatalf("401 invalid api key should trigger auto disable")
	}
}
