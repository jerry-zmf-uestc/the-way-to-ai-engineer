package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestJointRejectsWrongFeeAttemptBinding(t *testing.T) {
	program, digest := buildJointFee(t)
	s, f, _ := jointRoots(t, program, digest)
	state, err := RunJoint(context.Background(), s, id(), actions[:1], 3, false, f, jointRead(s.Root), nil)
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := f.Invoke(context.Background(), FeeRequest{Action: "prepare", Key: "other-attempt", Hash: strings.Repeat("b", 64)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Invoke(context.Background(), FeeRequest{Action: "begin", EntryID: foreign.EntryID}); err != nil {
		t.Fatal(err)
	}
	usage := int64(40)
	if _, err := f.Invoke(context.Background(), FeeRequest{Action: "settle", EntryID: foreign.EntryID, InputUsage: &usage}); err != nil {
		t.Fatal(err)
	}
	state.Calls[0].FeeID = foreign.EntryID
	if err := s.saveJoint(state); err != nil {
		t.Fatal(err)
	}
	_, err = RunJoint(context.Background(), s, id(), actions[:1], 3, false, f, func(context.Context, string) JointResponse {
		t.Fatal("wrong binding dispatched")
		return JointResponse{}
	}, nil)
	if !errors.Is(err, ErrState) {
		t.Fatal("same amount hid wrong attempt identity", err)
	}
}
