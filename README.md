# dendro-chronology-workbench

`dendro-chronology-workbench` 是面向树木年轮实验室的样芯年代序列裁定工作台。它只处理一条有证据约束的业务主线：创建并冻结样芯批次基线，登记制备影像，录入年轮观测和跨芯锚点，运行确定性质量规则，完成异常整改与独立复核，最后生成可校验的不可变 JSON 封存清单。

服务由 Go 单进程运行，直接提供原生 HTML、CSS、JavaScript 页面和同源 JSON API，不需要 Node.js 构建链。数据保存在本地目录：批次聚合、幂等响应和快照摘要写入原子 JSON 快照，审计事件写入带前序摘要的只追加 JSONL 哈希链。每次写操作都必须携带唯一 `request_id` 和 `expected_revision`；重复请求会重放第一次持久化的响应，陈旧修订会返回冲突。工作台批次总览可组合筛选状态、站点、树种、测量员和批次编号，并显示筛选范围统计、证据覆盖与当前阻塞原因。

## 构建与测试

需要 Go 1.22 或更高版本。

```bash
go build ./cmd/dendro-workbench
go test ./...
```

测试覆盖确定性规则与清单摘要、快照恢复、审计尾部修复、幂等行为、人员分离、封存不可变性、页面资源和稳定错误协议。

## 运行

标准运行命令：

```bash
go run ./cmd/dendro-workbench -addr=127.0.0.1:19081 -data-dir=./data
```

浏览器访问 `http://127.0.0.1:19081/`。默认只监听高位回环地址 `127.0.0.1:19081`，不会绑定 `0.0.0.0`。可以显式传入其他回环地址和高位端口：

```bash
go run ./cmd/dendro-workbench -addr=127.0.0.1:19444
```

未显式设置 `-addr` 时，也可以通过 `PORT` 提供端口号；服务仍只绑定 `127.0.0.1`：

```bash
PORT=19444 go run ./cmd/dendro-workbench
```

`-addr` 和 `PORT` 的端口范围为 1024 到 65535。非回环监听地址会被拒绝。正常运行时收到 `SIGINT` 或 `SIGTERM` 后，服务会在超时范围内优雅关闭。

## 有界自检

以下命令会创建临时证据目录，在真实回环 HTTP 服务上走完含异常整改的完整流程，并验证幂等重放、陈旧 revision 冲突、独立复核、封存清单摘要和主动退出：

```bash
go run ./cmd/dendro-workbench -self-check -addr=127.0.0.1:19081
```

成功时输出 `SELF_CHECK_OK`，临时目录随后删除。自检不会写入标准 `./data` 目录。

## 业务状态

批次严格沿以下状态推进：

`BASELINED → IMAGED → ANALYZED → CORRECTION_REQUIRED | REVIEW_READY → VERIFIED → SEALED`

- `CORRECTION_REQUIRED` 支持一次选择最多 50 项异常原子整改。所有替换先在批次副本中验证，通过后分别生成带 `supersedes_id` 的新观测，仅统一运行一次规则并产生一次 revision；整改理由、前后值和完成时间保存在异常轨迹中。
- 质量规则检查序号与年份连续性、宽度范围、边界顺序、缺轮或伪轮解释、跨芯锚点冲突及相关阈值。
- 复核员必须不同于测量员。复核签署前的只读预检会检查基线、影像与测量覆盖、规则结果、未关闭异常、人员分离、审计链和修订引用，并列出稳定排序的整改差异。提交复核必须携带预检返回的 `inspected_revision`。
- `SEALED` 为不可编辑状态。清单通过 `GET /api/batches/{batch_id}/manifest` 查看，附加 `?download=1` 可下载 JSON。

## 主要接口

- `GET /`：浏览器工作台。
- `GET /api/batches`、`GET /api/batches/{batch_id}`：批次总览和详情。总览支持 `status`、`site_code`、`species`、`operator_id`、`batch_id` 查询参数，返回 `statistics` 与每项 `progress`。
- `POST /api/batches`：创建并冻结采样基线。
- `POST /api/batches/{batch_id}/images`：登记全部样芯影像。
- `POST /api/batches/{batch_id}/observations`：提交候选年代序列。
- `POST /api/batches/{batch_id}/validate`：运行确定性质量规则。
- `POST /api/batches/{batch_id}/corrections`：通过 `items` 提交批量异常整改并原子复验；旧单项请求格式继续兼容。
- `GET /api/batches/{batch_id}/review-inspection?reviewer_id=...`：读取当前 revision 的复核证据预检与修订差异。
- `POST /api/batches/{batch_id}/review`：独立复核签署或退回。
- `POST /api/batches/{batch_id}/seal`：生成清单并封存。
- `GET /api/batches/{batch_id}/events`：查看只追加审计事件。
- `GET /api/batches/{batch_id}/manifest`：查看或下载封存清单。

API 请求体限制为 2 MiB，未知 JSON 字段会被拒绝。错误响应采用稳定的 `error.code`、中文 `error.message` 和可选 `error.field`。页面及 API 默认设置 CSP、禁止嵌入、MIME 嗅探防护等安全响应头。

## 数据恢复与完整性

启动时服务会验证每个快照摘要、连续 revision、事件前序摘要和事件自身摘要，以及封存清单摘要。如果进程在审计事件完成 `fsync` 后、快照原子替换前中止，重启会识别并裁剪这段完全有效但未提交的尾事件。任何哈希分叉、revision 缺口或内容损坏都会阻止加载，避免把证据损坏误当作正常状态。
