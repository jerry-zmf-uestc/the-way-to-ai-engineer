package main

import (
	"errors"
	"fmt"
	"strings"
)

type RoleStatus string

const (
	StatusCurrent  RoleStatus = "current"
	StatusDeferred RoleStatus = "deferred"
)

var ErrUnknownDirectory = errors.New("unknown project directory")

type DirectoryRole struct {
	Path           string
	Status         RoleStatus
	Responsibility string
	UseWhen        string
}

func ProjectRoles() []DirectoryRole {
	return []DirectoryRole{
		{
			Path:           "chapterN/<topic>",
			Status:         StatusCurrent,
			Responsibility: "学习示例按章节和 topic 归档",
			UseWhen:        "每天产出可运行 Go topic 时更新所属章节目录",
		},
		{
			Path:           "docs",
			Status:         StatusCurrent,
			Responsibility: "学习计划、工程规则和目录约束",
			UseWhen:        "结论已被完成示例或 spec 支撑后沉淀规则",
		},
		{
			Path:           "records",
			Status:         StatusCurrent,
			Responsibility: "飞书不可用时的本地学习记录兜底",
			UseWhen:        "文档读写或私聊失败时追加脱敏复盘",
		},
		{
			Path:           "scripts",
			Status:         StatusCurrent,
			Responsibility: "自动化入口和本地验证辅助脚本",
			UseWhen:        "需要稳定复用的仓库级操作，不承载业务代码",
		},
		{
			Path:           ".specs",
			Status:         StatusCurrent,
			Responsibility: "SDD topic 的 requirements、design、tasks 和 acceptance",
			UseWhen:        "CLI、小服务、并发或 mock 类任务开始前复制模板",
		},
		{
			Path:           "chapterN/<name>/cmd",
			Status:         StatusCurrent,
			Responsibility: "章节内分层项目的薄入口",
			UseWhen:        "练手项目需要稳定二进制入口时使用",
		},
		{
			Path:           "chapterN/<name>/internal",
			Status:         StatusCurrent,
			Responsibility: "章节内分层项目的私有实现",
			UseWhen:        "入口背后的业务逻辑需要拆分和测试时使用",
		},
		{
			Path:           "pkg",
			Status:         StatusDeferred,
			Responsibility: "对外可复用库代码",
			UseWhen:        "出现跨仓库对外复用需求并愿意维护稳定 API 时再使用",
		},
		{
			Path:           "api",
			Status:         StatusDeferred,
			Responsibility: "OpenAPI、proto 或其他接口契约",
			UseWhen:        "HTTP 服务进入契约化或多客户端协作阶段",
		},
		{
			Path:           "configs",
			Status:         StatusDeferred,
			Responsibility: "示例配置和默认配置模板",
			UseWhen:        "服务示例开始读取外部配置文件",
		},
		{
			Path:           "deployments",
			Status:         StatusDeferred,
			Responsibility: "部署描述和运行环境配置",
			UseWhen:        "服务需要 Docker、Kubernetes 或平台部署说明",
		},
		{
			Path:           "vendor",
			Status:         StatusDeferred,
			Responsibility: "提交第三方依赖源码快照",
			UseWhen:        "有明确离线构建要求；默认依赖 go mod",
		},
	}
}

func FindRole(path string) (DirectoryRole, error) {
	clean := strings.Trim(strings.TrimSpace(path), "/")
	for _, role := range ProjectRoles() {
		if role.Path == clean {
			return role, nil
		}
	}

	return DirectoryRole{}, fmt.Errorf("%w: %s", ErrUnknownDirectory, path)
}

func CurrentRoles(roles []DirectoryRole) []DirectoryRole {
	current := make([]DirectoryRole, 0, len(roles))
	for _, role := range roles {
		if role.Status == StatusCurrent {
			current = append(current, role)
		}
	}

	return current
}

func FormatRoles(roles []DirectoryRole) string {
	lines := make([]string, 0, len(roles))
	for _, role := range roles {
		lines = append(lines, fmt.Sprintf("%s [%s]: %s; use when %s",
			role.Path,
			role.Status,
			role.Responsibility,
			role.UseWhen,
		))
	}

	return strings.Join(lines, "\n")
}

func main() {
	fmt.Println(FormatRoles(CurrentRoles(ProjectRoles())))
}
