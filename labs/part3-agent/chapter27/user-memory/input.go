package main

import "context"

type MemoryInput struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Revision int64  `json:"revision"`
}

func (m Memory) Input(ctx context.Context, scope Scope, keys []string) ([]MemoryInput, error) {
	records, err := m.Resolve(ctx, scope, keys)
	if err != nil {
		return nil, err
	}
	input := make([]MemoryInput, 0, len(records))
	for _, record := range records {
		input = append(input, MemoryInput{Key: record.Key, Value: record.Value, Revision: record.Revision})
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return input, nil
}

func (i MemoryInput) EvidenceText() string { return i.Key + "=" + i.Value }
