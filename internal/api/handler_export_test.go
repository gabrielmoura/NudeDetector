package api

import "testing"

func TestOptionalExportValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		fn   func() (any, error)
		want any
	}{
		{name: "default boolean", fn: func() (any, error) { return optionalBool("", true) }, want: true},
		{name: "explicit boolean", fn: func() (any, error) { return optionalBool("false", true) }, want: false},
		{name: "default frame", fn: func() (any, error) { return optionalInt64("", 0) }, want: int64(0)},
		{name: "frame", fn: func() (any, error) { return optionalInt64("1200", 0) }, want: int64(1200)},
		{name: "default radius", fn: func() (any, error) { return optionalFloat64("", 25) }, want: float64(25)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.fn()
			if err != nil {
				t.Fatalf("parser error = %v", err)
			}
			if got != test.want {
				t.Fatalf("parser = %v, want %v", got, test.want)
			}
		})
	}
}

func TestOptionalExportValuesRejectMalformedInput(t *testing.T) {
	t.Parallel()

	if _, err := optionalBool("maybe", false); err == nil {
		t.Fatal("optionalBool() error = nil, want error")
	}
	if _, err := optionalInt64("frame", 0); err == nil {
		t.Fatal("optionalInt64() error = nil, want error")
	}
	if _, err := optionalFloat64("wide", 25); err == nil {
		t.Fatal("optionalFloat64() error = nil, want error")
	}
}
