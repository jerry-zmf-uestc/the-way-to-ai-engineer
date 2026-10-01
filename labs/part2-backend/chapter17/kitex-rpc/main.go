package main

import (
	"context"
	"fmt"

	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter17/kitex-rpc/kitex_gen/api"
)

// Demo runs bounded handler calls and returns deterministic output.
func Demo() string {
	service := EchoServiceImpl{}

	echo, err := service.Echo(context.Background(), &api.EchoRequest{Message: " hello kitex "})
	if err != nil {
		return fmt.Sprintf("Echo error: %v", err)
	}

	add, err := service.Add(context.Background(), &api.AddRequest{First: 512, Second: 512})
	if err != nil {
		return fmt.Sprintf("Add error: %v", err)
	}

	return fmt.Sprintf("Echo -> %s (%d), Add -> %d", echo.GetMessage(), echo.GetLength(), add.GetSum())
}

func main() {
	fmt.Println(Demo())
}
