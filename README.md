# task242-shipstrata

船舶沉没遗址层位关系复核台（shipstrata）：面向水下考古人员的层位关系推理后端服务。

## 业务域

水下考古人员需要判断一处构件究竟属于**原始沉没层**、**后期侵扰层**还是**测绘误连**。系统导入构件、沉积层、接触关系与采样点，构建层位偏序，检测不可能循环矛盾，生成侵扰候选；研究者复核接触证据、否决误连、裁决层位并发布不可变剖面版本。

## 状态机

| 实体 | 状态 |
| --- | --- |
| 遗址批次 site_batch | collecting → pending_review → published → sealed |
| 地层单元 strata_unit | candidate → stable / disturbed / excluded |
| 接触关系 contact | pending → overlies / cuts / conflict / confirmed / excluded |
| 剖面版本 profile | draft → shared → frozen → superseded |
| 侵扰候选 intrusion | open → accepted / dismissed |

层位偏序：接触关系 `overlies`（覆盖）与 `cuts`（切割）均表示「上覆单元晚于下伏单元」，即 A 在上 → A 晚于 B。求解器据此构建有向无环图，检测环路（不可能循环）并标记 `conflict`。

## 构建与运行

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go build -o shipstrata ./cmd/shipstrata
./shipstrata --addr :8080 --db ./shipstrata.db
./shipstrata --smoke-test
```

## 目录结构

```
env/
├── go.mod / go.sum
├── component-versions.json
├── Dockerfile / benzhi.Dockerfile / build_benzhi_docker.sh
├── BENZHI_README.md / README.md
├── cmd/shipstrata/main.go
└── internal/{model,store,site,contact,strata,intrusion,profile,sample,service,httpapi}
```
