package summary

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/config"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/obs"
)

const DemoTrace = "Frontend→Hertz→UserSvc→OrderSvc→ProductSvc"

type Result struct {
	Status     int    `json:"status"`
	Orders     int    `json:"orders"`
	TotalCents int64  `json:"total_cents"`
	Trace      string `json:"trace"`
	Attempts   int    `json:"attempts"`
}

func Fetch(ctx context.Context, cfg config.Config, gatewayAddr string) (Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+gatewayAddr+"/api/users/u-100/orders", nil)
	if err != nil {
		return Result{}, fmt.Errorf("new demo request: %w", err)
	}
	req.Header.Set(obs.TraceIDHeader, DemoTrace)

	resp, err := (&http.Client{Timeout: cfg.HTTPTimeout}).Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("call gateway: %w", err)
	}
	defer resp.Body.Close()

	var result Result
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return Result{}, fmt.Errorf("decode gateway response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("gateway status %d", resp.StatusCode)
	}
	return result, nil
}
