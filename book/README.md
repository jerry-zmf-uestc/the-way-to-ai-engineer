# Go AI 全栈学习

从 Go 程序开始，学习后端服务的实现与运行，再把模型和检索组件接成可测试的 Agent 应用。正文、示例和练习配套阅读；旅行助手放在独立项目中，不作为每章必做任务。

第一次使用本书，先读[学习方法与运行约定](introduction.md)，再完成[环境准备](part1-golang/chapter01.md)。

## Go 基础

1. [Go 环境与第一个程序](part1-golang/chapter01.md)
2. [数据类型](part1-golang/chapter02.md)
3. [控制流与边界分支](part1-golang/chapter03.md)
4. [函数](part1-golang/chapter04.md)
5. [错误处理与异常](part1-golang/chapter05.md)
6. [结构体与方法](part1-golang/chapter06.md)
7. [接口、泛型与反射](part1-golang/chapter07.md)
8. [包和工具](part1-golang/chapter08.md)
9. [并发编程](part1-golang/chapter09.md)
10. [Web 服务](part1-golang/chapter10.md)
11. [工程实践与代码规范](part1-golang/chapter11.md)

## 后端实践

- [数据访问与模型配置](part2-backend/chapter12.md)
- [事务边界与回滚](part2-backend/chapter13.md)
- [配置、日志、探针与关闭](part2-backend/chapter21.md)
- [Hertz 路由与网关](part2-backend/chapter16.md)
- [Kitex 与 RPC 契约](part2-backend/chapter17.md)

先掌握数据和运行边界，再看框架，较容易分清哪些行为属于业务，哪些由框架提供。正式章号用于主题归类，阅读顺序不必按编号机械推进。

## Golang Agent

- [LLM API、Message 与结构化输出](part3-agent/chapter22.md)
- [Tool Calling 与最小 Agent Loop](part3-agent/chapter23.md)
- [Eino 消息与模型边界](part3-agent/chapter26.md)
- [Embedding 与向量检索](part3-agent/chapter28.md)
- [用 Eino 组织 RAG 应用](part3-agent/chapter28/rag.md)

这些示例使用本地 fake 模型，无需 API key。先验证组件之间的数据和错误如何传递，再研究真实模型的效果与成本。
