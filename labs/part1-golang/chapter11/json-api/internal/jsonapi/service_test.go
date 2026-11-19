package jsonapi

import (
	"errors"
	"testing"
)

func TestEchoServiceEcho(t *testing.T) {
	t.Parallel()

	service := NewEchoService()
	tests := []struct {
		name    string
		message string
		want    EchoResponse
		wantErr error
	}{
		{
			name:    "trims message and counts runes",
			message: "  hello, 世界  ",
			want: EchoResponse{
				Echo:   "hello, 世界",
				Length: 9,
			},
		},
		{
			name:    "rejects empty message",
			message: "   ",
			wantErr: ErrEmptyMessage,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := service.Echo(tt.message)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Echo(%q) error = %v, want %v", tt.message, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Echo(%q) error = %v, want nil", tt.message, err)
			}
			if got != tt.want {
				t.Fatalf("Echo(%q) = %+v, want %+v", tt.message, got, tt.want)
			}
		})
	}
}
