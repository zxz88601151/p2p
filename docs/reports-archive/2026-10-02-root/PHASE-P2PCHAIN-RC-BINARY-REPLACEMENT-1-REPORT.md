# PHASE-P2PCHAIN-RC-BINARY-REPLACEMENT-1 — RC 二进制替换报告

> **阶段**：替换溯源无效的 RC 二进制为干净构建产物（禁止 code/commit/tag/push/deploy/远端访问）
> **日期**：2026-10-02
> **治理指令**：Owner 授权，将溯源有缺口的开发二进制替换为 VCS 干净的官方 RC 二进制。

---

## §0 历史二进制记录（HISTORICAL BINARY RECORD）

| 项 | 值 |
|----|----|
| 文件 | `node.exe`（**未删除，保留为开发工件**） |
| 大小 | 12,130,304 字节 |
| SHA-256 | `fb3d2b57dd8adbd02d3c6d501f42a2cfa3c076657694e35756967caf9307b717` |
| VCS revision | `52fb464af243fbcdfd78bab846b45ea090e918fb`（旧 HEAD） |
| VCS modified | `true`（dirty 工作树） |
| 分类 | 开发工件，溯源缺口，**仅本地测试，不对外分发** |

> 处置：**保留不删**。仅记录其身份，明确其为 development artifact，不进入 RC 发布。

---

## §1 构建参数冻结（BUILD PARAMETERS FREEZE）

| 参数 | 值 |
|------|----|
| 源码提交 | `3e6d44a248013f814859cf4f5be6feb5bc57f492` |
| tree 哈希 | `d5065ee7714bea533674b6d723d7ec11187e47f9` |
| Go 版本 | `go1.27.0 windows/amd64` |
| OS / 架构 | `windows / amd64` |
| 构建标志 | `-trimpath` |
| 构建命令 | `go build -trimpath -o node-rc.exe ./cmd/node` |

---

## §2 RC 二进制构建（RC BINARY BUILD）

| 项 | 结果 |
|----|------|
| 干净 checkout | `/tmp/p2pchain-rc-build`（detached `3e6d44a`，零漂移） |
| 构建命令 | `go build -trimpath -o node-rc.exe ./cmd/node` |
| BUILD EXIT | 0 |
| 二进制大小 | 12,112,896 字节 |
| SHA-256 | `17e5ce8180f66526ed06826bbe84c74949e8f01b114f2d1895aa2964bfeb0edf` |

### VCS 溯源核验（全部通过 ✅）

| 校验项 | 结果 |
|--------|------|
| `vcs.revision = 3e6d44a248013f814859cf4f5be6feb5bc57f492` | ✅ 精确匹配 |
| `vcs.modified = false` | ✅ 干净工作树 |
| `mod` 伪版本无 `+dirty` 后缀 | ✅ `v0.0.0-20261002015259-3e6d44a24801` |
| 哈希可复现性 | ✅ 与此前 FREEZE 阶段 `-trimpath` 构建一致（`17e5ce81…`） |

### 交付物落地

| 项 | 值 |
|----|----|
| 项目内官方二进制 | `node-v0.9.0-rc1.exe`（已复制到项目根目录） |
| SHA-256 | `17e5ce8180f66526ed06826bbe84c74949e8f01b114f2d1895aa2964bfeb0edf` |
| 现有 node.exe | 保留未动（SHA256 `fb3d2b57…` 不变） |

---

## §3 二进制清单（BINARY MANIFEST）

已产出 `RC-BINARY-MANIFEST.md`，包含：
- 官方 RC 二进制身份（文件名 / 大小 / SHA256 / 版本标签）
- 冻结构建参数（源码提交 / tree 哈希 / Go 版本 / OS-arch / 构建标志）
- VCS 溯源（revision / time / modified / mod 伪版本）
- 复现验证命令
- 历史开发工件 `node.exe` 的隔离说明（溯源缺口，不对外分发）

---

## 结论

| 项 | 结果 |
|----|------|
| 历史二进制 | ✅ 已记录（不删除），明确为开发工件 |
| 构建参数 | ✅ 已冻结（`3e6d44a` + `-trimpath` + Go1.27.0） |
| 官方 RC 二进制 | ✅ 已生成并落地 `node-v0.9.0-rc1.exe`，VCS 溯源干净 |
| 溯源缺口 | ✅ 已解决（`3e6d44a` + `modified=false`） |
| 清单 | ✅ `RC-BINARY-MANIFEST.md` 已产出 |

**本阶段零 git 变更**：无 code 改动、无 commit、无 tag、无 push、无 deploy、无远端访问。新增文件均为工作树未跟踪产物（`node-v0.9.0-rc1.exe`、`RC-BINARY-MANIFEST.md`），未 `git add`。

—— PHASE-P2PCHAIN-RC-BINARY-REPLACEMENT-1 结束 ——
