# BENZHI_README

## 项目说明
- 项目：benzhi-project-99539411-448f-47f9-a486-fecffaac196b
- 项目用途：完整实现树木年轮样芯批次从基线冻结、影像登记、年代序列测量、确定性质量裁定、异常整改、独立复核到不可变证据封存的单流程浏览器工作台，并以原子快照、持久幂等响应和只追加哈希审计链保存证据。
- Go 工具链：`golang:1.22`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 项目描述
- 项目名称：dendro-chronology-workbench
- 项目介绍：面向树木年轮实验室的样芯年代序列裁定工作台，以一个样芯批次从建档、影像测量、交叉定年、异常整改、独立复核到证据封存的状态闭环为唯一业务流程。Go 服务直接提供原生 HTML、CSS、JavaScript 和同源 JSON 接口，使用本地原子快照与只追加审计链保存过程证据。项目根目录包含简体中文 README.md，说明用途、构建、运行、测试及有界自检方式。生产实现目标不少于 2000 行有效 Go 代码、20 个生产 Go 文件，前端资源不计入该规模。
- 项目概述：面向树木年轮实验室的样芯年代序列裁定工作台，以一个样芯批次从建档、影像测量、交叉定年、异常整改、独立复核到证据封存的状态闭环为唯一业务流程。Go 服务直接提供原生 HTML、CSS、JavaScript 和同源 JSON 接口，使用本地原子快照与只追加审计链保存过程证据。项目根目录包含简体中文 README.md，说明用途、构建、运行、测试及有界自检方式。生产实现目标不少于 2000 行有效 Go 代码、20 个生产 Go 文件，前端资源不计入该规模。
- 核心工作流：测量员创建样芯批次并冻结采样基线，登记制备影像与比例尺，逐芯录入年轮边界和交叉定年锚点；规则校验发现缺轮、伪轮或序列错位后进入待整改状态，测量员逐项修订并重新校验，另一名复核员确认人员分离与证据完整性后签署，系统生成确定性封存清单并把批次置为不可变的 SEALED 状态。
- 对外接口：浏览器单页工作台由 Go 服务直接提供原生 HTML、CSS 和 JavaScript，通过同源 JSON 接口完成批次列表、分步表单、年轮序列表格、异常整改、独立复核及封存清单查看；不引入 Node 构建链。服务支持 -addr=127.0.0.1:<port>，默认监听 127.0.0.1:19081，并在未显式传入 -addr 时读取 PORT 且仅绑定 127.0.0.1:<PORT>，绝不默认绑定 0.0.0.0。

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...

cd /app && GOTOOLCHAIN=local go run ./cmd/dendro-workbench -self-check -addr=127.0.0.1:19081

cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh

./build_benzhi_docker.sh benzhi-project-99539411-448f-47f9-a486-fecffaac196b-amd64 linux/amd64

./build_benzhi_docker.sh benzhi-project-99539411-448f-47f9-a486-fecffaac196b-arm64 linux/arm64

docker run -it benzhi-project-99539411-448f-47f9-a486-fecffaac196b-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/dendro-workbench -self-check -addr=127.0.0.1:19081`
