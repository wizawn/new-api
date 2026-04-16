package controller

import (
	"errors"
	"testing"
)

func TestShouldRetryChatMockError(t *testing.T) {
	if !shouldRetryChatMockError(errors.New("System busy!")) {
		t.Fatalf("expected transient system busy error to be retryable")
	}
	if !shouldRetryChatMockError(errors.New("HTTP 503 service unavailable")) {
		t.Fatalf("expected 503 error to be retryable")
	}
	if shouldRetryChatMockError(errors.New("invalid request payload")) {
		t.Fatalf("expected invalid payload error to be non-retryable")
	}
}

func TestRunChatMockWithRetryRetriesTransientUntilSuccess(t *testing.T) {
	attempt := 0
	err := runChatMockWithRetry(4, func() error {
		attempt++
		if attempt < 3 {
			return errors.New("system busy")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected retry to eventually succeed, got error: %v", err)
	}
	if attempt != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempt)
	}
}

func TestRunChatMockWithRetryStopsOnNonRetryableError(t *testing.T) {
	attempt := 0
	err := runChatMockWithRetry(5, func() error {
		attempt++
		return errors.New("invalid request payload")
	})
	if err == nil {
		t.Fatalf("expected error for non-retryable failure")
	}
	if attempt != 1 {
		t.Fatalf("expected non-retryable error to stop at first attempt, got %d", attempt)
	}
}
