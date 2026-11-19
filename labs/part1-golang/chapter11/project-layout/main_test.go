package main

import (
	"errors"
	"strings"
	"testing"
)

func TestFindRoleForCurrentDirectory(t *testing.T) {
	role, err := FindRole("chapterN/<topic>")
	if err != nil {
		t.Fatalf("FindRole(%q) error = %v", "chapterN/<topic>", err)
	}

	if role.Status != StatusCurrent {
		t.Fatalf("status = %q, want %q", role.Status, StatusCurrent)
	}
	if !strings.Contains(role.Responsibility, "学习示例") {
		t.Fatalf("responsibility = %q, want it to mention 学习示例", role.Responsibility)
	}
}

func TestFindRoleForDeferredDirectory(t *testing.T) {
	role, err := FindRole("pkg")
	if err != nil {
		t.Fatalf("FindRole(%q) error = %v", "pkg", err)
	}

	if role.Status != StatusDeferred {
		t.Fatalf("status = %q, want %q", role.Status, StatusDeferred)
	}
	if !strings.Contains(role.UseWhen, "对外复用") {
		t.Fatalf("useWhen = %q, want it to mention 对外复用", role.UseWhen)
	}
}

func TestFindRoleUnknownDirectory(t *testing.T) {
	_, err := FindRole("misc")
	if !errors.Is(err, ErrUnknownDirectory) {
		t.Fatalf("error = %v, want ErrUnknownDirectory", err)
	}
}

func TestCurrentRolesExcludeDeferredScaffolding(t *testing.T) {
	roles := CurrentRoles(ProjectRoles())
	for _, role := range roles {
		if role.Status != StatusCurrent {
			t.Fatalf("CurrentRoles returned %q with status %q", role.Path, role.Status)
		}
		if role.Path == "pkg" || role.Path == "vendor" {
			t.Fatalf("CurrentRoles included deferred directory %q", role.Path)
		}
	}
}

func TestFormatRoles(t *testing.T) {
	got := FormatRoles([]DirectoryRole{
		{
			Path:           "chapterN/<topic>",
			Status:         StatusCurrent,
			Responsibility: "学习示例",
			UseWhen:        "每天产出 topic",
		},
	})

	for _, want := range []string{"chapterN/<topic>", "current", "学习示例", "每天产出 topic"} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatted roles = %q, want it to contain %q", got, want)
		}
	}
}
