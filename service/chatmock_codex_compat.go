package service

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

const (
	chatMockSSEInitialBufferSize = 64 << 10
	chatMockSSEMaxBufferSize     = 64 << 20
)

// ConvertChatMockCodexSSEToChatCompletion aggregates codex responses SSE events
// and converts the final response.completed payload into a chat/completions JSON.
func ConvertChatMockCodexSSEToChatCompletion(reader io.Reader, chatID string) ([]byte, int64, error) {
	if reader == nil {
		return nil, 0, fmt.Errorf("codex stream body is nil")
	}

	outputItemsByIndex := make(map[int]dto.ResponsesOutput)
	outputItemsFallback := make([]dto.ResponsesOutput, 0, 8)
	var completed *dto.OpenAIResponsesResponse

	err := scanChatMockSSEDataEvents(reader, func(data string) error {
		var streamResp dto.ResponsesStreamResponse
		if err := common.UnmarshalJsonStr(data, &streamResp); err != nil {
			return fmt.Errorf("解析 codex 流事件失败: %w", err)
		}

		switch strings.TrimSpace(streamResp.Type) {
		case "response.output_item.done":
			if streamResp.Item == nil {
				return nil
			}
			item := *streamResp.Item
			if streamResp.OutputIndex != nil {
				outputItemsByIndex[*streamResp.OutputIndex] = item
			} else {
				outputItemsFallback = append(outputItemsFallback, item)
			}
			return nil
		case "response.completed":
			if streamResp.Response != nil {
				completed = streamResp.Response
			}
			return nil
		case "response.error", "response.failed":
			if streamResp.Response != nil {
				if oaiErr := streamResp.Response.GetOpenAIError(); oaiErr != nil {
					msg := strings.TrimSpace(oaiErr.Message)
					if msg != "" {
						return fmt.Errorf("codex 上游错误: %s", msg)
					}
				}
			}
			return fmt.Errorf("codex 上游流事件异常: %s", streamResp.Type)
		default:
			return nil
		}
	})
	if err != nil {
		return nil, 0, err
	}

	if completed == nil {
		return nil, 0, fmt.Errorf("codex 流未返回 response.completed")
	}

	if len(completed.Output) == 0 && (len(outputItemsByIndex) > 0 || len(outputItemsFallback) > 0) {
		patched := make([]dto.ResponsesOutput, 0, len(outputItemsByIndex)+len(outputItemsFallback))
		indexes := make([]int, 0, len(outputItemsByIndex))
		for idx := range outputItemsByIndex {
			indexes = append(indexes, idx)
		}
		sort.Ints(indexes)
		for _, idx := range indexes {
			patched = append(patched, outputItemsByIndex[idx])
		}
		patched = append(patched, outputItemsFallback...)
		completed.Output = patched
	}

	if oaiErr := completed.GetOpenAIError(); oaiErr != nil {
		msg := strings.TrimSpace(oaiErr.Message)
		if msg != "" {
			return nil, 0, fmt.Errorf("codex 上游错误: %s", msg)
		}
		return nil, 0, fmt.Errorf("codex 上游返回错误响应")
	}

	if strings.TrimSpace(chatID) == "" {
		chatID = "chatcmpl-" + common.GetUUID()
	}
	chatResp, usage, err := ResponsesResponseToChatCompletionsResponse(completed, chatID)
	if err != nil {
		return nil, 0, fmt.Errorf("转换 codex completed 响应失败: %w", err)
	}

	payload, err := common.Marshal(chatResp)
	if err != nil {
		return nil, 0, fmt.Errorf("序列化 chat/completions 响应失败: %w", err)
	}

	tokenUsed := int64(chatResp.Usage.TotalTokens)
	if usage != nil && usage.TotalTokens > 0 {
		tokenUsed = int64(usage.TotalTokens)
	}
	return payload, tokenUsed, nil
}

func scanChatMockSSEDataEvents(reader io.Reader, onData func(data string) error) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, chatMockSSEInitialBufferSize), chatMockSSEMaxBufferSize)

	dataParts := make([]string, 0, 4)
	flush := func() error {
		if len(dataParts) == 0 {
			return nil
		}
		data := strings.TrimSpace(strings.Join(dataParts, "\n"))
		dataParts = dataParts[:0]
		if data == "" {
			return nil
		}
		if data == "[DONE]" {
			return io.EOF
		}
		return onData(data)
	}

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			if err := flush(); err != nil {
				if err == io.EOF {
					return nil
				}
				return err
			}
			continue
		}

		if strings.HasPrefix(line, "data:") {
			dataParts = append(dataParts, strings.TrimSpace(line[5:]))
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if err := flush(); err != nil && err != io.EOF {
		return err
	}
	return nil
}
