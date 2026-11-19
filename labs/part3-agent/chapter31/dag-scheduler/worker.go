package main

import "context"

func executeWork(ctx context.Context, run func(context.Context) (string, error)) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	text, err := run(ctx)
	if stop := ctx.Err(); stop != nil {
		return "", stop
	}
	if len(text) > 4096 {
		return "", ErrGraph
	}
	if err != nil {
		return "", err
	}
	return text, nil
}
