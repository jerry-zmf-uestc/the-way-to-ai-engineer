package main

import (
	"fmt"
	"io"
	"os"

	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part1-golang/chapter11/json-api/internal/jsonapi"
)

func main() {
	os.Exit(run(os.Stdout))
}

func run(stdout io.Writer) int {
	service := jsonapi.NewEchoService()
	handler := jsonapi.NewServer(service)
	fmt.Fprintln(stdout, jsonapi.DemoRequest(handler))
	return 0
}
