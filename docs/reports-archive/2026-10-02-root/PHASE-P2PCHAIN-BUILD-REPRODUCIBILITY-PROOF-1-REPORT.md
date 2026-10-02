# PHASE-P2PCHAIN-BUILD-REPRODUCIBILITY-PROOF-1 — 构建可复现性证明报告

> **阶段**：构建溯源验证（只读 + 本地隔离重建，禁止 code/commit/tag/push/deploy/远端访问）
> **日期**：2026-10-02
> **治理指令**：Owner 授权，从干净 checkout 重建二进制并与现有 node.exe 对比哈希，判定可复现性类别。

---

## §0 基线（BASELINE）

| 项 | 值 |
|----|----|
| HEAD SHA | `3e6d44a248013f814859cf4f5be6feb5bc57f492` |
| Go 版本 | `go1.27.0 windows/amd64` |
| OS/架构 | `windows / amd64` |
| 现有 node.exe SHA256 | `fb3d2b57dd8adbd02d3c6d501f42a2cfa3c076657694e35756967caf9307b717` |
| 现有 node.exe 大小 | 12,130,304 字节 |

---

## §1 干净 Checkout（CLEAN CHECKOUT）

| 项 | 结果 |
|----|------|
| worktree 路径 | `/tmp/p2pchain-rc-build`（`git worktree add --detach 3e6d44a`） |
| worktree HEAD | `3e6d44a248013f814859cf4f5be6feb5bc57f492` ✅ |
| 零 git 漂移 | ✅ `git status --short` 空 |
| 无凭据/运行时数据 | ✅ 仅含源码+文档+脚本（`cmd/internal/docs/scripts`） |

---

## §2 重建（REBUILD）

| 项 | 值 |
|----|----|
| 构建命令 | `go build -o node-clean.exe ./cmd/node` |
| 编译版本 | `go1.27.0 windows/amd64` |
| 二进制大小 | 12,131,328 字节 |
| SHA256 | `446509f5cc3064e9788f0cdb5814b7bc89bfc6ba5984c9c35629b659d77f6853` |
| BUILD EXIT | 0（成功） |

---

## §3 哈希对比与分类（HASH COMPARISON）

### 哈希对比表

| 二进制 | 大小 (B) | SHA256（前 16 字符） | vcs.revision | vcs.modified |
|--------|----------|----------------------|--------------|--------------|
| 现有 `node.exe` | 12,130,304 | `fb3d2b57…` | **`52fb464…`（旧 HEAD）** | **`true`（dirty）** |
| 干净重建 `node-clean` | 12,131,328 | `446509f5…` | `3e6d44a…`（当前 HEAD） | `false` |
| 原目录重建 `node-rebuild` | — | `feef54c9…` | `3e6d44a…`（当前 HEAD） | `true`（dirty） |
| trimpath 重建 | — | `17e5ce81…` | — | — |

### 决定性证据：VCS 内嵌元数据

现有 `node.exe` 通过 `go version -m` 反查，内嵌 VCS 信息为：

```
mod  p2pchain  v0.0.0-20261001104622-52fb464af243+dirty
build vcs=git
build vcs.revision=52fb464af243fbcdfd78bab846b45ea090e918fb
build vcs.time=2026-10-01T10:46:22Z
build vcs.modified=true
```

**关键结论**：现有 `node.exe` 是在 **`52fb464`（旧 HEAD）+ dirty 工作树**状态下构建的，其 VCS 溯源指向的提交早于 RC 三连提交（`36f9630`/`8410036`/`3e6d44a`）。

### 分类判定：**C. Unresolved Provenance Gap（未解决的溯源缺口）**

| 判定依据 | 说明 |
|----------|------|
| 哈希不可复现 | 现有 node.exe 的 SHA256 无法从当前 RC 边界（`3e6d44a`）的任何构建命令复现 |
| VCS 溯源不一致 | 现有 node.exe 内嵌 `vcs.revision=52fb464` + `modified=true`，与 RC 边界 `3e6d44a` 不符 |
| 缺口性质 | 现有二进制虽**内容上含孤儿代码**（字符串特征已证），但其 VCS 元数据指向旧提交，无法证明其构建自当前 RC 源码 |

### 为何不是 A（reproducible）或 B（explainable variance）

- **非 A**：现有 node.exe 与干净重建哈希不一致，且差异根因是「旧 VCS 提交 + dirty」，而非可复现的正常构建。
- **非 B**：虽有可解释因素（dirty 工作树、旧 revision），但「现有 node.exe 构建自哪个精确源码状态」**无法从 VCS 元数据唯一确定**，属于溯源缺口而非可解释的确定性差异。

---

## 结论与建议

| 项 | 结果 |
|----|------|
| 可复现性分类 | **C. Unresolved Provenance Gap** |
| 缺口根因 | 现有 node.exe 构建时 VCS 为 `52fb464`（旧 HEAD）+ dirty 工作树 |
| 干净重建 | ✅ 成功（`3e6d44a`，`vcs.modified=false`），SHA256=`446509f5…` |

**建议（待 Owner 授权，不本阶段执行）**：
1. **废弃现有 node.exe**，改用干净 checkout 重建的 `node-clean.exe`（VCS 溯源干净：`3e6d44a` + `modified=false`）作为 RC 官方二进制。
2. 若要可复现发布，采用固定构建参数（`-trimpath` + 统一构建目录 + 锁定 Go 版本），并在发布说明中记录精确 SHA256。
3. 现有 node.exe 保留仅供本地测试，不对外分发。

---

## 治理确认

本阶段仅做了本地隔离重建 + 哈希对比 + VCS 反查，**未执行**：code 改动、commit、tag、push、deploy、远端访问。临时产物 `node-rebuild.exe` 已清理；`/tmp/p2pchain-rc-build` worktree 保留（含干净二进制 `node-clean.exe`，SHA256=`446509f5…`）供 Owner 决策。

—— PHASE-P2PCHAIN-BUILD-REPRODUCIBILITY-PROOF-1 结束 ——
