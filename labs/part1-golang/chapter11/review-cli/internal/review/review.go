package review

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	// ErrNoItems means the caller did not provide any review entries.
	ErrNoItems = errors.New("review: no items")
	// ErrInvalidItem means one input does not match topic=score.
	ErrInvalidItem = errors.New("review: invalid item")
	// ErrInvalidTopic means a topic is empty or too long.
	ErrInvalidTopic = errors.New("review: invalid topic")
	// ErrInvalidScore means a score is not an integer from 0 through 5.
	ErrInvalidScore = errors.New("review: invalid score")
)

// InputError describes which raw input failed validation.
type InputError struct {
	Input string
	Err   error
}

// Error returns a human-readable validation error.
func (e *InputError) Error() string {
	return fmt.Sprintf("%s: %v", e.Input, e.Err)
}

// Unwrap returns the underlying validation category.
func (e *InputError) Unwrap() error {
	return e.Err
}

// Entry is one reviewed learning topic and its 0-5 score.
type Entry struct {
	Topic string
	Score int
}

// Report summarizes all reviewed items.
type Report struct {
	Entries []Entry
	Weakest Entry
	Average float64
	Status  string
}

// ParseItem parses one topic=score CLI value.
func ParseItem(raw string) (Entry, error) {
	topic, scoreText, ok := strings.Cut(raw, "=")
	if !ok {
		return Entry{}, inputError(raw, ErrInvalidItem)
	}

	topic = strings.TrimSpace(topic)
	scoreText = strings.TrimSpace(scoreText)
	if topic == "" || utf8.RuneCountInString(topic) > 64 {
		return Entry{}, inputError(raw, ErrInvalidTopic)
	}

	score, err := strconv.Atoi(scoreText)
	if err != nil || score < 0 || score > 5 {
		return Entry{}, inputError(raw, ErrInvalidScore)
	}

	return Entry{Topic: topic, Score: score}, nil
}

// Summarize parses all raw items and returns an aggregate report.
func Summarize(items []string) (Report, error) {
	if len(items) == 0 {
		return Report{}, ErrNoItems
	}

	entries := make([]Entry, 0, len(items))
	total := 0
	for _, item := range items {
		entry, err := ParseItem(item)
		if err != nil {
			return Report{}, err
		}
		entries = append(entries, entry)
		total += entry.Score
	}

	weakest := entries[0]
	for _, entry := range entries[1:] {
		if entry.Score < weakest.Score {
			weakest = entry
		}
	}

	average := float64(total) / float64(len(entries))
	return Report{
		Entries: entries,
		Weakest: weakest,
		Average: average,
		Status:  statusFor(average),
	}, nil
}

// Format renders a report as one stable CLI output line.
func Format(report Report) string {
	return fmt.Sprintf(
		"items=%d average=%.2f status=%s weakest=%s",
		len(report.Entries),
		report.Average,
		report.Status,
		report.Weakest.Topic,
	)
}

func inputError(raw string, err error) error {
	return &InputError{Input: raw, Err: err}
}

func statusFor(average float64) string {
	if average >= 4 {
		return "on-track"
	}
	if average >= 2.5 {
		return "needs-review"
	}
	return "restart"
}
