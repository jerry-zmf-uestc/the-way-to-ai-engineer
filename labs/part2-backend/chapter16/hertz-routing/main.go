package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type bookResponse struct {
	ID        string `json:"id"`
	RequestID string `json:"request_id"`
}

type healthResponse struct {
	Status    string `json:"status"`
	RequestID string `json:"request_id"`
}

type searchResponse struct {
	Query     string `json:"query"`
	RequestID string `json:"request_id"`
}

func requestIDMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		requestID := string(c.Request.Header.Peek("X-Request-ID"))
		if requestID == "" {
			requestID = "generated-demo-id"
		}
		c.Set("request_id", requestID)
		c.Next(ctx)
		c.Response.Header.Set("X-Request-ID", requestID)
	}
}

func requestID(c *app.RequestContext) string {
	value, ok := c.Get("request_id")
	if !ok {
		return ""
	}
	id, ok := value.(string)
	if !ok {
		return ""
	}
	return id
}

func newRouter() *server.Hertz {
	h := server.New(server.WithHostPorts(":8888"))
	h.Use(requestIDMiddleware())

	h.GET("/healthz", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(consts.StatusOK, healthResponse{
			Status:    "ok",
			RequestID: requestID(c),
		})
	})

	api := h.Group("/api")
	api.GET("/books/:id", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(consts.StatusOK, bookResponse{
			ID:        c.Param("id"),
			RequestID: requestID(c),
		})
	})
	api.GET("/search", func(ctx context.Context, c *app.RequestContext) {
		query := c.Query("q")
		if query == "" {
			c.JSON(consts.StatusBadRequest, utils.H{"error": "q is required"})
			return
		}
		c.JSON(consts.StatusOK, searchResponse{
			Query:     query,
			RequestID: requestID(c),
		})
	})

	return h
}

func run(serve bool) {
	router := newRouter()
	fmt.Println("hertz routes ready: GET /healthz, GET /api/books/:id, GET /api/search?q=go")

	// 默认保持 non-blocking，便于 go run 在自动化里跑完即退。
	// 带 -serve 时才真正 Spin() 阻塞监听 :8888，供浏览器 / curl 验证。
	if !serve {
		_ = router
		return
	}

	fmt.Println("listening on http://127.0.0.1:8888 (Ctrl+C to stop)")
	router.Spin()
}

func main() {
	serve := flag.Bool("serve", false, "block and listen on :8888 so the routes can be hit from a browser")
	flag.Parse()
	run(*serve)
}
