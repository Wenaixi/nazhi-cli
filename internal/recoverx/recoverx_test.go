package recoverx

import (
	"errors"
	"testing"
)

var errSentinel = errors.New("sentinel error")

func TestRecoverPanic_Nil(t *testing.T) {
	got := RecoverPanic(nil, errSentinel, "test")
	if got != nil {
		t.Fatalf("RecoverPanic(nil, sentinel, name) = %v, want nil", got)
	}
}

func TestRecoverPanic_WithSentinel(t *testing.T) {
	r := errors.New("oops")
	got := RecoverPanic(r, errSentinel, "test")
	if got == nil {
		t.Fatal("RecoverPanic(r, sentinel, name) = nil, want error")
	}
	if !errors.Is(got, errSentinel) {
		t.Errorf("errors.Is(result, sentinel) = false, want true")
	}
}

func TestRecoverPanic_NoSentinel(t *testing.T) {
	r := errors.New("oops")
	got := RecoverPanic(r, nil, "test")
	if got == nil {
		t.Fatal("RecoverPanic(r, nil, name) = nil, want error")
	}
	if errors.Is(got, errSentinel) {
		t.Errorf("errors.Is(result, sentinel) = true, want false")
	}
	// 验证错误消息包含 name
	if s := got.Error(); s != "test: oops" {
		t.Errorf("RecoverPanic error = %q, want %q", s, "test: oops")
	}
}
