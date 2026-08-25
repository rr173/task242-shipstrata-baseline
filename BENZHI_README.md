基于 Go 实现的水下考古遗址层位关系复核 Web 项目，一款后端服务，在统一层位偏序上构建构件与沉积层接触关系、检测不可能循环矛盾并识别后期侵扰层与测绘误连，发布不可变剖面版本。

# BENZHI 评测说明

本目录为 Git 根（`go.mod` 位于此），提供可运行、可测试的 Go 服务，使用纯 Go SQLite 持久化（`modernc.org/sqlite`，无需 CGO）。

## 运行命令

```bash
# 构建
CGO_ENABLED=0 GOTOOLCHAIN=local go build -o shipstrata ./cmd/shipstrata

# 启动 HTTP 服务（默认 :8080，库文件 ./shipstrata.db）
./shipstrata --addr :8080 --db ./shipstrata.db

# 自检（不启动长驻服务，建库、跑核心闭环、关闭重开验证恢复后以 0 退出）
./shipstrata --smoke-test
```

## 业务闭环

导入构件 / 沉积层 / 接触关系 / 采样点 → 构建层位偏序 → 检测不可能循环矛盾 → 生成侵扰候选 → 研究者复核误连、裁决层位 → 发布不可变剖面版本。

## API 契约（前缀 `/api`）

- `POST /api/sites` 创建遗址批次
- `POST /api/sites/{id}/start-review` 进入待复核
- `POST /api/sites/{id}/publish` 发布
- `POST /api/sites/{id}/seal` 封存
- `GET /api/sites` / `GET /api/sites/{id}`
- `POST /api/sites/{id}/units` 创建地层单元
- `GET /api/sites/{id}/units` / `GET /api/units/{id}`
- `PATCH /api/units/{id}/status` 设置单元状态（stable/disturbed/excluded）
- `POST /api/sites/{id}/contacts` 单条导入接触关系
- `POST /api/sites/{id}/contacts/batch` 批量导入
- `GET /api/sites/{id}/contacts` / `GET /api/contacts/{id}`
- `POST /api/contacts/{id}/confirm` 确认
- `POST /api/contacts/{id}/reject` 否决误连（排除）
- `POST /api/sites/{id}/adjudicate` 运行层位偏序求解
- `GET /api/sites/{id}/contradictions` 循环矛盾
- `GET /api/sites/{id}/intrusion-candidates` 侵扰候选
- `POST /api/sites/{id}/profiles` 创建剖面版本（草稿）
- `POST /api/profiles/{id}/share` 共享
- `POST /api/profiles/{id}/freeze` 冻结（不可变快照）
- `POST /api/profiles/{id}/supersede` 替代
- `GET /api/sites/{id}/profiles` / `GET /api/profiles/{id}`
- `POST /api/sites/{id}/samples` 采样点（越界拒绝）
- `GET /api/sites/{id}/samples`
- `GET /api/sites/{id}/graph` 层位关系图
- `GET /api/stats` 统计
- `POST /api/self-check` 内置自检

## --smoke-test 契约

脚本真实创建遗址批次、地层单元与构成不可能循环的三条接触关系，运行偏序求解确认矛盾被检出，否决其中一条误连后重解得到一致偏序，再将侵扰单元裁决为受扰并冻结剖面版本；随后关闭并重新打开同一数据库，断言全部实体、状态与冻结快照完整恢复，最终以退出码 0 结束。该契约是评测判定健康基线的唯一依据。
