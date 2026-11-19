package main

import (
	"net/http"
	"testing"
)

func TestBurstAdmissionAndRecovery(t *testing.T) {
	for _, tc := range []struct {
		limit int
		want  observation
	}{
		{0, observation{burst, burst, 0, burst, http.StatusNoContent}},
		{capacity, observation{burst, capacity, burst - capacity, capacity, http.StatusNoContent}},
	} {
		got, err := exercise(tc.limit)
		if err != nil || got != tc.want {
			t.Fatalf("limit=%d: got %+v, %v; want %+v", tc.limit, got, err, tc.want)
		}
	}
}
