package relay

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaychannel "github.com/QuantumNous/new-api/relay/channel"
	openaichannel "github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

// chatMockTextHelper routes standard /v1/chat/completions traffic through ChatMock account pool
// when the selected NewAPI channel type is ChatMockPool.
func chatMockTextHelper(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (*dto.Usage, *types.NewAPIError) {
	if info == nil || request == nil {
		return nil, types.NewError(fmt.Errorf("chatmock relay request is nil"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	if info.RelayMode != relayconstant.RelayModeChatCompletions {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("chatmock channel only supports /v1/chat/completions"),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	chatJSON, err := common.Marshal(request)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeJsonMarshalFailed, types.ErrOptionWithSkipRetry())
	}
	chatJSON, err = relaycommon.RemoveDisabledFields(chatJSON, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	if len(info.ParamOverride) > 0 {
		chatJSON, err = relaycommon.ApplyParamOverrideWithRelayInfo(chatJSON, info)
		if err != nil {
			return nil, newAPIErrorFromParamOverride(err)
		}
	}

	var overriddenReq dto.GeneralOpenAIRequest
	if err = common.Unmarshal(chatJSON, &overriddenReq); err != nil {
		return nil, types.NewError(err, types.ErrorCodeChannelParamOverrideInvalid, types.ErrOptionWithSkipRetry())
	}

	modelName := strings.TrimSpace(overriddenReq.Model)
	if modelName == "" {
		modelName = strings.TrimSpace(info.UpstreamModelName)
	}
	if modelName == "" {
		modelName = strings.TrimSpace(info.OriginModelName)
	}
	if modelName == "" {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("model is required"),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	incomingHeaders := make(http.Header, len(c.Request.Header)+8)
	for key, values := range c.Request.Header {
		for _, value := range values {
			incomingHeaders.Add(key, value)
		}
	}
	headerOverride, err := relaychannel.ResolveHeaderOverride(info, c)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeChannelHeaderOverrideInvalid, types.ErrOptionWithSkipRetry())
	}
	for key, value := range headerOverride {
		incomingHeaders.Set(key, value)
	}

	upstreamResp, account, routeErr := service.RouteChatMockChatCompletion(c.Request.Context(), chatJSON, modelName, incomingHeaders)
	if routeErr != nil {
		return nil, types.NewOpenAIError(routeErr, types.ErrorCodeDoRequestFailed, http.StatusBadGateway)
	}
	if account == nil || upstreamResp == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid chatmock route result"), types.ErrorCodeBadResponse, http.StatusBadGateway)
	}

	isCodexAccount := service.IsLikelyChatMockCodexAccount(account)
	if isCodexAccount && upstreamResp.StatusCode == http.StatusOK {
		shouldStream := overriddenReq.Stream != nil && *overriddenReq.Stream
		if shouldStream {
			info.IsStream = true
			usage, streamErr := openaichannel.OaiResponsesToChatStreamHandler(c, info, upstreamResp)
			if streamErr != nil {
				service.RecordChatMockAccountFailure(account.Id, "Codex 流式兼容转换失败: "+streamErr.Error(), setting.ChatMockDefaultCooldownSeconds)
				return nil, streamErr
			}
			tokenUsed := int64(0)
			if usage != nil && usage.TotalTokens > 0 {
				tokenUsed = int64(usage.TotalTokens)
			}
			service.RecordChatMockAccountSuccess(account.Id, tokenUsed)
			if usage == nil {
				usage = &dto.Usage{
					PromptTokens: info.GetEstimatePromptTokens(),
					TotalTokens:  info.GetEstimatePromptTokens(),
				}
			}
			return usage, nil
		}

		payload, tokenUsed, convertErr := service.ConvertChatMockCodexSSEToChatCompletion(upstreamResp.Body, "chatmock-"+common.GetUUID())
		_ = upstreamResp.Body.Close()
		if convertErr != nil {
			service.RecordChatMockAccountFailure(account.Id, "Codex 非流式兼容转换失败: "+convertErr.Error(), setting.ChatMockDefaultCooldownSeconds)
			return nil, types.NewOpenAIError(convertErr, types.ErrorCodeBadResponseBody, http.StatusBadGateway)
		}
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Status(http.StatusOK)
		if _, writeErr := c.Writer.Write(payload); writeErr != nil {
			service.RecordChatMockAccountFailure(account.Id, "Codex 非流式响应写回失败: "+writeErr.Error(), setting.ChatMockDefaultCooldownSeconds)
			return nil, types.NewOpenAIError(writeErr, types.ErrorCodeBadResponse, http.StatusInternalServerError)
		}
		service.RecordChatMockAccountSuccess(account.Id, tokenUsed)
		return usageFromTotalTokens(info.GetEstimatePromptTokens(), tokenUsed), nil
	}

	if upstreamResp.StatusCode != http.StatusOK {
		// bad request to upstream should not punish account health, but still release in-flight slot.
		service.RecordChatMockAccountSuccess(account.Id, 0)
		return nil, service.RelayErrorHandler(c.Request.Context(), upstreamResp, false)
	}

	contentType := strings.ToLower(strings.TrimSpace(upstreamResp.Header.Get("Content-Type")))
	info.IsStream = info.IsStream || strings.Contains(contentType, "text/event-stream")

	var (
		usage    *dto.Usage
		relayErr *types.NewAPIError
	)
	if info.IsStream {
		usage, relayErr = openaichannel.OaiStreamHandler(c, info, upstreamResp)
	} else {
		usage, relayErr = openaichannel.OpenaiHandler(c, info, upstreamResp)
	}
	if relayErr != nil {
		service.RecordChatMockAccountFailure(account.Id, "ChatMock 响应处理失败: "+relayErr.Error(), setting.ChatMockDefaultCooldownSeconds)
		return nil, relayErr
	}

	tokenUsed := int64(0)
	if usage != nil && usage.TotalTokens > 0 {
		tokenUsed = int64(usage.TotalTokens)
	}
	service.RecordChatMockAccountSuccess(account.Id, tokenUsed)
	if usage == nil {
		usage = &dto.Usage{
			PromptTokens: info.GetEstimatePromptTokens(),
			TotalTokens:  info.GetEstimatePromptTokens(),
		}
	}
	return usage, nil
}

func usageFromTotalTokens(promptTokens int, totalTokens int64) *dto.Usage {
	if promptTokens < 0 {
		promptTokens = 0
	}
	usage := &dto.Usage{
		PromptTokens: promptTokens,
		TotalTokens:  promptTokens,
	}
	if totalTokens <= 0 {
		return usage
	}
	total := int(totalTokens)
	if total < 0 {
		total = 0
	}
	usage.TotalTokens = total
	if promptTokens > total {
		usage.PromptTokens = 0
	}
	usage.CompletionTokens = usage.TotalTokens - usage.PromptTokens
	if usage.CompletionTokens < 0 {
		usage.CompletionTokens = 0
	}
	return usage
}
