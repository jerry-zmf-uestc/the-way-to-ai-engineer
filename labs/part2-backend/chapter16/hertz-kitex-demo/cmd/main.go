package main

import (
	"os"

	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/app"
)

func main() {
	os.Exit(app.Run(os.Stdout, os.Stderr, os.Environ()))
}
