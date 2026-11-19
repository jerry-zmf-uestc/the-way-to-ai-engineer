package review

import (
	"errors"
	"fmt"
	"testing"
)

func TestParseItem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want Entry
	}{
		{name: "normal", raw: "gofmt=5", want: Entry{Topic: "gofmt", Score: 5}},
		{name: "trims spaces", raw: " errors = 0 ", want: Entry{Topic: "errors", Score: 0}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseItem(tt.raw)
			if err != nil {
				t.Fatalf("ParseItem() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ParseItem() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseItemErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want error
	}{
		{name: "missing separator", raw: "bad", want: ErrInvalidItem},
		{name: "empty topic", raw: "=3", want: ErrInvalidTopic},
		{name: "long topic", raw: "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklm=3", want: ErrInvalidTopic},
		{name: "not integer", raw: "gofmt=fast", want: ErrInvalidScore},
		{name: "too high", raw: "gofmt=6", want: ErrInvalidScore},
		{name: "negative", raw: "gofmt=-1", want: ErrInvalidScore},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := ParseItem(tt.raw)
			if !errors.Is(err, tt.want) {
				t.Fatalf("ParseItem() error = %v, want errors.Is(%v)", err, tt.want)
			}
		})
	}
}

func TestSummarize(t *testing.T) {
	t.Parallel()

	report, err := Summarize([]string{"gofmt=5", "errors=4", "json=4"})
	if err != nil {
		t.Fatalf("Summarize() error = %v", err)
	}

	if report.Average != 13.0/3.0 {
		t.Fatalf("Average = %v, want %v", report.Average, 13.0/3.0)
	}
	if report.Status != "on-track" {
		t.Fatalf("Status = %q, want on-track", report.Status)
	}
	if report.Weakest != (Entry{Topic: "errors", Score: 4}) {
		t.Fatalf("Weakest = %#v, want errors score 4", report.Weakest)
	}
}

func TestSummarizeNoItems(t *testing.T) {
	t.Parallel()

	_, err := Summarize(nil)
	if !errors.Is(err, ErrNoItems) {
		t.Fatalf("Summarize(nil) error = %v, want ErrNoItems", err)
	}
}

func TestStatusFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		average float64
		want    string
	}{
		{name: "on track", average: 4, want: "on-track"},
		{name: "needs review", average: 2.5, want: "needs-review"},
		{name: "restart", average: 2.49, want: "restart"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := statusFor(tt.average); got != tt.want {
				t.Fatalf("statusFor(%v) = %q, want %q", tt.average, got, tt.want)
			}
		})
	}
}

func ExampleFormat() {
	report, err := Summarize([]string{"gofmt=5", "errors=4"})
	if err != nil {
		return
	}
	fmt.Println(Format(report))
	// Output:
	// items=2 average=4.50 status=on-track weakest=errors
}
