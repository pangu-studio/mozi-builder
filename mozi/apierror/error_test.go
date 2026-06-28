package apierror

import "testing"

func TestWrapDoesNotMutateSource(t *testing.T) {
	source := New("DECK_NOT_FOUND", "missing", nil)
	envelope := Wrap(source, "req-1")
	if source.RequestID != "" || envelope.Error.RequestID != "req-1" {
		t.Fatalf("source=%#v envelope=%#v", source, envelope)
	}
}
