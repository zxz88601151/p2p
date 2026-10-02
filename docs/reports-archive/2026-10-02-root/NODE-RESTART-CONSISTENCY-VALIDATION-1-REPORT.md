# NODE-RESTART-CONSISTENCY-VALIDATION-1-REPORT

> **Phase**：NODE-RESTART-CONSISTENCY-VALIDATION-1 — 节点重启一致性验证
> **Owner authorization**：PHASE P2PCHAIN — NODE-RESTART-CONSISTENCY-VALIDATION-1
> **Date**：2026-10-02
> **Scope**：Runtime restart consistency validation only · No code changes · No consensus changes · No protocol changes
> **HARD STOP（强制）**：No commit · No deployment · No cloud modification · No source changes

---

## 结论速览

**同步后的节点可安全停止、重启、重新同步，并收敛回同一 canonical 链。** A/B/C 先收敛至高度 8；停 B 后 A/C 推进至高度 22；B 用既有数据目录重启后，**存储恢复 + peer 重连 + 补齐 14 个缺失块 + 无非法 reorg**，最终 A/B/C 三者同高、同 tip、同 chainwork、`verify=true`。

---

## §0 Baseline

| 项 | 值 |
|---|---|
| `git rev-parse HEAD` | `52fb464af243fbcdfd78bab846b45ea090e918fb` |
| 二进制 `node.exe` SHA-256 | `fb3d2b57dd8adbd02d3c6d501f42a2cfa3c076657694e35756967caf9307b717` |
| 创世身份 `CanonicalGenesisHash` | `00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3` |
| 当前规则集 | v1（链高 < 2000）；bits=16 恒，`Work(bits)=2^bits` |

---

## §1 Initial synchronization

| 节点 | 角色 | 高度 | tip | chainwork |
|---|---|---:|---:|---:|
| A | 矿工（挖 8 块） | 8 | `0000233e0b937286…` | 589,824（9×2^16） |
| B | 跟随（seed→A） | 8 | 同上 | 589,824 |
| C | 跟随（seed→A） | 8 | 同上 | 589,824 |

✅ 三节点同 tip、同高度、同 chainwork（初始收敛成立）。

---

## §2 Controlled shutdown（停 B，A/C 继续推进）

- **停 B**：`kill` B 进程（无 token 故无法优雅 `stop`，采用进程终止模拟受控关停），B 下线。
- **A/C 推进**：A 继续挖矿至高度 22（tip `000032f49622273e…`），C 经 seed 同步到 22。
- **B 落伍**：B 停留在高度 8，缺失 14 个区块（9–22）。

| 时间点 | A 高度 | C 高度 | B 高度 |
|---|---:|---:|---:|
| 停 B 前 | 8 | 8 | 8 |
| A 续挖后 | 22 | 22 | 8（离线） |

---

## §3 Restart recovery（用既有数据目录重启 B）

B 重启后（`-datadir` 不变，`-seed A`）：

**存储恢复**：
```
本地区块链已就绪: 高度=8 链尾=0000233e0b937286… 数据文件=…\b\blocks.dat
```
（从既有 `blocks.dat` 回放，正确恢复到停机前高度 8，未丢链、未重建。）

**peer 重连**：
```
握手完成: 对端=127.0.0.1:17101 对端高度=22 本地高度=8 对端工作量="1507328" 对端链尾=000032f4
```
（重连 A，发现对端更高，工作量 1,507,328 = 23×2^16 与独立重算一致。）

**缺块同步**：
```
新区块已上链: 高度=9 … 高度=22（14 条，连续无缺）
同步进度: 应用 14 个区块，缺父待补 0 个，本地高度=22，对方已到链尾=true
```
（精确补齐 14 个缺失块，**0 缺父待补**，无孤儿残留。）

**无非法 reorg**：B 重启日志中**无 `REORG_REJECT` / `REORG_ATTEMPT` / 链切换事件**——B 的存量 8 块是 A 链的正确前缀，重启后仅顺序**追加** 14 块，未触发任何 reorg（正确，因非分叉）。

✅ 存储恢复、peer 重连、缺块同步、无非法 reorg 全部满足。

---

## §4 Final convergence

| 节点 | height | tip hash | chainwork | verify |
|---|---|---:|---:|---|
| A | 22 | `000032f49622273edb16851f7d1510abed9534be83f4c1d82f25e047339596bf` | 1,507,328 | ✅ valid=true |
| B | 22 | 同上 | 1,507,328 | ✅ valid=true |
| C | 22 | 同上 | 1,507,328 | ✅ valid=true |

- **同高度**：22 ✅
- **同 tip hash**：`000032f4…` ✅
- **同 chainwork**：1,507,328（= 23 块 × 2^16）✅（与握手日志 `对端工作量="1507328"` 精确一致）
- **verify=true**：三节点均 `valid=true` ✅

**最终收敛成立**：B 从高度 8（离线 14 块）重启后，经存储回放 + 重连 + 同步，精确回到与 A/C 相同的 canonical 链。

---

## 关键结论

1. **重启一致性成立**：节点停止 → 重启 → 重同步 → 收敛，全程无损、无分叉、无非法 reorg。
2. **存储持久化正确**：`blocks.dat` 回放精确恢复到停机前高度（8），未丢失已确认区块。
3. **增量同步正确**：重启后仅拉取缺失的 14 块（9–22），`缺父待补 0`，无孤儿/缺父。
4. **chainwork 跨层一致**：握手上报的工作量（1,507,328）与独立 Python 重算（23×2^16）完全一致。
5. **无非法 reorg**：B 的存量链是 A 链前缀，重启同步仅顺序追加，无链切换——符合「非分叉不 reorg」预期。

---

## HARD STOP confirmation

- ❌ 未执行任何 `git commit` / `git push`。
- ❌ 未部署到任何环境（腾讯云三节点集群未触碰）。
- ❌ 未修改云/生产配置。
- ❌ **未改动任何源码**（`cmd/`、`internal/` diff 均为前序 B1/B1.5/B2 阶段既存；本阶段仅新增 `.workbuddy/restart_test.sh` 临时脚本 + 本报告，零源码改动）。
- ❌ 未产生任何生产数据变更（实验隔离在 `/tmp` scratch，验证后已停节点、清理端口监听）。

**验证方法**：① 真实启动 A/B/C 三节点；② 真实停 B、A 续挖推进；③ B 用既有 datadir 真实重启，观察存储回放、握手重连、增量同步日志；④ `node verify` + 独立 Python 重算交叉核对高度/tip/chainwork。全程隔离、零源码改动。
