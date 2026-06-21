# 三进程重构 TODO

> 关联文档: docs/refactor-plan.md
> 分支: refactor-three-process
> Worktree: /Volumes/sandisk/Work/Work/ulanzi-deck-herdr-refactor

## Phase 0 — 前置 (1 task) ✅

- [x] 0.1 创建 worktree + 清理

## Phase 1 — protocol/ (6 tasks) ✅

- [x] 1.1 创建 protocol 目录 + go.mod
- [x] 1.2 snapshot.go (FleetSnapshot, AgentState, AgentStats, MachineInfo)
- [x] 1.3 status.go (AgentStatus 枚举 + Priority)
- [x] 1.4 subjects.go (NATS subject 常量)
- [x] 1.5 version.go (协议版本)
- [x] 1.6 protocol 测试 + 验证 (4 passed)

## Phase 2 — collector/ (10 tasks) ✅

- [x] 2.1 创建 collector 目录 + go.mod + Makefile
- [x] 2.2 config/config.go 迁移
- [x] 2.3 herdrclient/client.go 迁移
- [x] 2.4 tunnel/tunnel.go 迁移
- [x] 2.5 bridge/bridge.go 改写
- [x] 2.6 fleet/store.go 新建
- [x] 2.7 publisher/publisher.go 新建
- [x] 2.8 natsserver/server.go 新建
- [x] 2.9 cmd/herdr-collector/main.go 新建
- [x] 2.10 collector 全部测试通过 (14 passed, 7 packages)

## Phase 3 — deck/ (12 tasks) ✅

- [x] 3.1 创建 deck 目录 + go.mod + Makefile
- [x] 3.2 subscriber/subscriber.go 新建
- [x] 3.3 fleet/manager.go 改写
- [x] 3.4 viewmodel/builder.go 迁移 + 改写
- [x] 3.5 viewmodel/types.go 新建
- [x] 3.6 render/ 迁移
- [x] 3.7 deckclient/ 迁移 (client + draw + keyhash)
- [x] 3.8 profile/manager.go 迁移
- [x] 3.9 sysstats/sysstats.go 迁移
- [x] 3.10 controller/controller.go 新建
- [x] 3.11 cmd/herdr-deck/main.go 新建
- [x] 3.12 deck 全部测试通过 (53 passed, 8 packages)

## Phase 4 — 整合 (5 tasks) ✅

- [x] 4.1 go.work 创建
- [x] 4.2 scripts/deploy-collector.sh + deploy-deck.sh + deploy-all.sh
- [x] 4.3 collector + deck 构建验证 (vet clean, binaries built)
- [x] 4.4 旧 go/ 目录已删除
- [x] 4.5 AGENTS.md 更新

---

**总计: 34 tasks ✅**

关键验证点 (checkpoints):

- ✅ CP1: Phase 1 → protocol 模块 (4 tests, 1 package)
- ✅ CP2: Phase 2 → collector 二进制 (14 tests, 8 packages)
- ✅ CP3: Phase 3 → deck 二进制 (53 tests, 9 packages)
- ✅ CP4: Phase 4 → go.work + scripts + docs
