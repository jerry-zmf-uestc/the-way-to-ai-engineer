package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part1-golang/chapter11/review-cli/internal/review"
)

type itemFlags []string

func (f *itemFlags) String() string {
	return fmt.Sprint([]string(*f))
}

func (f *itemFlags) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

func run(_ context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var items itemFlags
	flags := flag.NewFlagSet("review-cli", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Var(&items, "item", "review item in topic=score form, repeatable")

	if err := flags.Parse(args); err != nil {
		return 2
	}

	report, err := review.Summarize(items)
	if err != nil {
		fmt.Fprintf(stderr, "review-cli: %v\n", err)
		if isValidationError(err) {
			return 2
		}
		return 1
	}

	if _, err := fmt.Fprintln(stdout, review.Format(report)); err != nil {
		fmt.Fprintf(stderr, "review-cli: write output: %v\n", err)
		return 1
	}
	return 0
}

func isValidationError(err error) bool {
	return errors.Is(err, review.ErrNoItems) ||
		errors.Is(err, review.ErrInvalidItem) ||
		errors.Is(err, review.ErrInvalidTopic) ||
		errors.Is(err, review.ErrInvalidScore)
}
