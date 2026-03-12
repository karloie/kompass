package kube

import (
	"errors"
	"testing"
	"time"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func TestClientIoC(t *testing.T) {
	fakeClientset := fake.NewSimpleClientset()
	client := NewClientWithClientset(fakeClientset, nil, nil, "test-context", "test-namespace")
	if client == nil {
		t.Fatal("Expected client to be created")
	}
	if client.GetClientset() != fakeClientset {
		t.Error("Retrieved clientset does not match injected clientset")
	}
	if ctx, err := client.GetContext(); err != nil {
		t.Fatalf("GetContext() error: %v", err)
	} else if ctx != "test-context" {
		t.Errorf("Expected context 'test-context', got '%s'", ctx)
	}
	if ns, err := client.GetNamespace(); err != nil {
		t.Fatalf("GetNamespace() error: %v", err)
	} else if ns != "test-namespace" {
		t.Errorf("Expected namespace 'test-namespace', got '%s'", ns)
	}
	if client.IsMockMode() {
		t.Error("Client should not be in mock mode")
	}
}

func TestSetClientset(t *testing.T) {
	fakeClientset1 := fake.NewSimpleClientset()
	client := NewClientWithClientset(fakeClientset1, nil, nil, "context1", "namespace1")
	if client.GetClientset() != fakeClientset1 {
		t.Error("Initial clientset mismatch")
	}
	if client.GetDynamicClient() != nil {
		t.Error("Expected initial dynamic client to be nil")
	}
	if client.GetConfig() != nil {
		t.Error("Expected initial config to be nil")
	}

	fakeClientset2 := fake.NewSimpleClientset()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	cfg := &rest.Config{Host: "https://after-set"}
	client.SetClientset(fakeClientset2, dynamicClient, cfg)
	if client.GetClientset() != fakeClientset2 {
		t.Error("Clientset was not replaced")
	}
	if client.GetDynamicClient() != dynamicClient {
		t.Error("Dynamic client was not replaced")
	}
	if client.GetConfig() != cfg {
		t.Error("Config was not replaced")
	}

	client.SetClientset(fakeClientset2, dynamicClient, nil)
	if client.GetConfig() != cfg {
		t.Error("Config should remain unchanged when nil config is passed")
	}

	if ctx, _ := client.GetContext(); ctx != "context1" {
		t.Errorf("Context should not have changed, got '%s'", ctx)
	}
}

func TestMockClientImmutable(t *testing.T) {
	mockClient := NewMockClient(nil)
	if !mockClient.IsMockMode() {
		t.Error("Mock client should be in mock mode")
	}
	if mockClient.GetClientset() != nil {
		t.Error("Mock client should return nil clientset")
	}
	fakeClientset := fake.NewSimpleClientset()
	mockClient.SetClientset(fakeClientset, nil, nil)
	if mockClient.GetClientset() != nil {
		t.Error("Mock client should remain immutable")
	}
}

func TestRetryWithBackoff(t *testing.T) {
	client := NewClientWithClientset(fake.NewSimpleClientset(), nil, nil, "test-context", "default")
	client.maxRetries = 3
	client.initialBackoff = 10 * time.Millisecond
	client.maxBackoff = 100 * time.Millisecond

	t.Run("immediate success", func(t *testing.T) {
		attempts := 0
		result, err := retryWithBackoff(client, func() (string, error) {
			attempts++
			return "success", nil
		})
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result != "success" {
			t.Errorf("Expected 'success', got %q", result)
		}
		if attempts != 1 {
			t.Errorf("Expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("eventual success after retries", func(t *testing.T) {
		attempts := 0
		result, err := retryWithBackoff(client, func() (string, error) {
			attempts++
			if attempts < 3 {

				return "", kerrors.NewTimeoutError("timeout", 1)
			}
			return "success", nil
		})
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result != "success" {
			t.Errorf("Expected 'success', got %q", result)
		}
		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("max retries exceeded", func(t *testing.T) {
		attempts := 0
		result, err := retryWithBackoff(client, func() (string, error) {
			attempts++
			return "", kerrors.NewServiceUnavailable("persistent error")
		})
		if err == nil {
			t.Error("Expected error after max retries")
		}
		if result != "" {
			t.Errorf("Expected empty result, got %q", result)
		}
		if attempts != client.maxRetries+1 {
			t.Errorf("Expected %d attempts, got %d", client.maxRetries+1, attempts)
		}
	})

	t.Run("non-retryable error", func(t *testing.T) {
		attempts := 0
		nonRetryableErr := errors.New("permissions denied")
		result, err := retryWithBackoff(client, func() (string, error) {
			attempts++
			return "", nonRetryableErr
		})
		if err != nonRetryableErr {
			t.Errorf("Expected non-retryable error, got %v", err)
		}
		if result != "" {
			t.Errorf("Expected empty result, got %q", result)
		}
		if attempts != 1 {
			t.Errorf("Expected 1 attempt (no retry), got %d", attempts)
		}
	})
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{"nil error", nil, false},
		{"timeout error", &mockRetryableError{message: "timeout"}, true},
		{"server timeout", &mockRetryableError{message: "server timeout"}, true},
		{"service unavailable", &mockRetryableError{message: "service unavailable"}, true},
		{"too many requests", &mockRetryableError{message: "too many requests"}, true},
		{"internal error", &mockRetryableError{message: "internal error"}, true},
		{"regular error", errors.New("regular error"), false},
		{"not found", createK8sError(404, "not found"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryable(tt.err); got != tt.retryable {
				t.Errorf("isRetryable(%v) = %v, want %v", tt.err, got, tt.retryable)
			}
		})
	}
}

type mockRetryableError struct {
	message string
}

func (e *mockRetryableError) Error() string {
	return e.message
}

func (e *mockRetryableError) Status() metav1.Status {
	var reason metav1.StatusReason
	switch e.message {
	case "timeout":
		reason = metav1.StatusReasonTimeout
	case "server timeout":
		reason = metav1.StatusReasonServerTimeout
	case "service unavailable":
		reason = metav1.StatusReasonServiceUnavailable
	case "too many requests":
		reason = metav1.StatusReasonTooManyRequests
	case "internal error":
		reason = metav1.StatusReasonInternalError
	default:
		reason = metav1.StatusReasonUnknown
	}
	return metav1.Status{Reason: reason}
}

func createK8sError(code int32, message string) error {
	return &apiStatusError{
		ErrStatus: metav1.Status{
			Code:    code,
			Message: message,
		},
	}
}

type apiStatusError struct {
	ErrStatus metav1.Status
}

func (e *apiStatusError) Error() string {
	return e.ErrStatus.Message
}

func (e *apiStatusError) Status() metav1.Status {
	return e.ErrStatus
}
