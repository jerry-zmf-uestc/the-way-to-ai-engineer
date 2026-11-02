# 实验：产物引用、分页与任务隔离

应用在真实私有临时目录发布UTF8产物，Agent只通过注册ID和行页读取。验证任务根、内容摘要、分页、配额、过期与撤销。

不开放任意文件读写或Shell，不提供持久catalog、崩溃恢复或多实例配额。每个任务一个Workspace实例，目录结构由可信应用管理。

## 前置与入口

阅读[Workspace正文](../../../../content/book/part3-agent/runtime/chapter30-state.md#topic-agent-workspace)、Context与文件基础，准备见[运行约定](../../../../content/book/introduction.md#running-examples)。

- [main.go](main.go)：可信Scope、os.Root、应用Publish、ID读取、摘要与Cleanup。
- [main_test.go](main_test.go)：UTF8行页、跨任务/路径拒绝、内容变化、链接逃逸、期限与墓碑。
- [page_test.go](page_test.go)：同页文本不同产物、完整内容摘要、连续行范围与同长度改写后的拒绝。

## 预测与运行

三行产物只读两行，Complete应是什么？产物过期后复用原ID，会不会让旧引用指向新内容？

```bash
go -C labs run ./part3-agent/chapter30/workspace
go -C labs test ./part3-agent/chapter30/workspace -race -count=1
go -C labs test ./part3-agent/chapter30/workspace -cover
go -C labs build -o /tmp/workspace-demo ./part3-agent/chapter30/workspace
```

输出artifact=report-1、bytes=18、next_line=2、complete=false、page="line1\nline2\n"。继续从NextLine读取才能得到完整内容。

## 页面怎样绑定到同一产物

Page另返回ArtifactID、完整内容Digest与StartLine，默认CLI仍打印原字段。续读时核对同一ID/Digest，并让本页StartLine接前页NextLine；Complete只说明到文件末尾，不证明此前全部行都已读取。

```bash
go -C labs test ./part3-agent/chapter30/workspace -run 'TestPagesCarry|TestChangedContinuation' -race -count=1 -v
```

两个产物均以shared一行开头，返回页Text相同，ID与整份内容摘要仍不同。同一产物第二页保持原摘要与连续范围；末尾空页仍有身份、范围与Complete=true。同长度替换尾页后，Read返回ErrChanged和零Page，不能只靠长度或相同前页接纳新内容。

Digest不是当前页摘要或授权凭证，ID还须结合Host持有的Workspace任务绑定；测试没有把内部Tenant/User标识公开给模型。原来源/版本与持久化练习继续保留，页元信息不替代实际回查、访问准入或恢复协议。

## 应用侧完整页组装

[assembly.go](assembly.go)的Assemble使用当前Workspace的catalog取得ID/Digest/Bytes/期限，检查应用已读pages，从第0行连续到最后Complete，再核对总字节和整份hash。Collected与Page分开；最多256页，单页128行/4KiB，总预算1..32KiB。坏范围/混入/改写返回ErrPageSequence，配额ErrQuota，未登记/过期ErrDenied，均不交付部分Collected。

```bash
go -C labs test ./part3-agent/chapter30/workspace -list '^TestAssemble'
go -C labs test ./part3-agent/chapter30/workspace -run '^TestAssemble' -race -count=1 -v
```

[assembly_test.go](assembly_test.go)真实Publish/Read三页UTF8文本，末行无换行仍可完整组装；合法空产物EOF保留身份。反例覆盖缺首/中页、重叠/乱序、错ID/digest/文本、范围不符、提前Complete、末尾后再加页、无进度、半行与坏UTF8，以及页数、128行、单页/总字节、其他任务无登记、取消和返回时到期。

调用方持有输入页列表，组装期间不并发修改。该方法不重新读取当前磁盘、不认证页的调用来源，不自动将全文提供给模型，不提供持久catalog或跨实例撤销；应用只在确需全文时组装，按需局部读取仍是原路径。原CLI输出不变。

## 边界观察

- 相同ID在其他任务无登记：拒绝；模型不提供可信Scope。
- 绝对路径、..或未知ID：拒绝，不转成任意文件路径。
- 文件被替换：长度/摘要不同返回ErrChanged。
- 链接指向Root外文件：操作系统边界拒绝，不读取目标。
- 进入读取与返回页面前均检查期限；TestPageExpiresAtReturnBoundary用可控时钟在读取期间到期，返回ErrDenied且无页面。拒绝访问不自动删除文件或减少Used。
- 精确到期后不读取或列出；Cleanup成功减少当前已登记字节，ID墓碑不复用。
- 单行超过页预算：ErrQuota，不伪装成完整内容。

最多16个已登记ID（包含撤销墓碑）、文件32KiB、活跃内容64KiB、页4KiB及128行。分页前先读取有界全文校验，是小型教学后端，不是大文件流式索引。临时文件只属于实验，不提交仓库。

## 迁移练习

1. 构造超长单行，设计明确的字符/字段分页与单位，验证UTF8和续读范围。
2. 为每份产物增加来源、版本与摘要，检查删除缓存和Trace副本的策略；文件删除不表示全部副本已清除。
3. 将catalog持久化，设计文件发布与元信息提交窗口，不能只重启程序就复用旧目录。
4. 多worker共享时设计原子配额与所有权，不能以单实例Mutex代替分布式约束。
5. 比较直接回填与产物按需读取的任务效果和输入消耗，已保存但未读取的文件不能作为回答依据。
6. 从Assemble与Context交接起步，分别设计“只使用指定行范围”和“必须取得全文”的任务；局部页保留范围，不标成完整报告。自行补关键条件位于未读页、总输入预算不足及消费前资料失效的测试，原组装测试只作身份/完整性回归，不能代替新任务效果。

## 来源与许可

原创代码，MIT。标准库依据[os.Root](https://pkg.go.dev/os#Root)、[io](https://pkg.go.dev/io)、[crypto/sha256](https://pkg.go.dev/crypto/sha256)。

<a id="book-agent-workspace-最佳实践"></a>

## 最佳实践

## 最佳实践

### 在工具返回处决定卸载

大结果先受限保存，再返回可追溯的摘要与ID；Context根据任务补读。避免收到无限文本以后再裁剪，也避免产物存在却没有任何读取入口。

### 公开逻辑身份，隐藏物理布局

模型需要知道资料是什么，不需要服务器绝对路径。外部接口暴露注册ID和范围，应用负责解析到后端对象。

### 把副本与生命周期一起设计

发布、引用、缓存、过期与删除使用同一内容身份。涉及隐私时核对所有衍生副本，不能用“文件已删”推导全部数据已消失。

前面已经把发布、分页、变更和回收串成一条生命周期。最后只保留排错入口，让读者遇到“页读对了但任务还是错”时，能迅速回到对应边界。


<a id="book-agent-workspace-常见问题排查"></a>

## 常见问题排查

## 常见问题排查

### 第一页正常，模型却把它当成全文

检查Complete与NextLine是否进入观察，确认任务所需范围是否已读。修复结构化完整性标记，不仅在Prompt里写“注意可能不完整”。

### 换了任务还能读旧产物

检查Workspace是否由可信任务身份构造、是否共享同一实例或catalog。用相同ID在不同任务测试拒绝，不能只看目录名。

### 文件仍存在，但引用不可用

核对过期、撤销、内容长度与Digest。需要新版时发布新ID，不能忽略变化直接沿用旧引用；恢复场景还要确认catalog已持久化。

