package worksetapi

import "testing"

func TestErrorTypes(t *testing.T) {
	if (ValidationError{Message: "invalid"}).Error() == "" {
		t.Fatalf("expected validation error message")
	}
	if (NotFoundError{Message: "missing"}).Error() == "" {
		t.Fatalf("expected not found message")
	}
	if (ConflictError{Message: "conflict"}).Error() == "" {
		t.Fatalf("expected conflict message")
	}
	if (ConfirmationRequired{Message: "confirm"}).Error() == "" {
		t.Fatalf("expected confirmation message")
	}
	unsafe := UnsafeOperation{}
	if unsafe.Error() == "" {
		t.Fatalf("expected unsafe error message")
	}
	unsafe = UnsafeOperation{Message: "stop"}
	if unsafe.Error() != "stop" {
		t.Fatalf("expected explicit unsafe message")
	}
}
