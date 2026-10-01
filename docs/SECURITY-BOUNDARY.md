# P2PChain — Security Boundary（Skeleton）

> **状态：SKELETON / 待补全。** 本文件为最小骨架，仅登记当前**已文档化**的安全边界现状；
> **不声称任何未经代码验证的安全结论**，亦不将规划中的能力列为已实现。

## 当前安全边界（指向已验证文档）

| 层 | 现状 | 权威来源 |
|---|---|---|
| 控制接口（RPC） | 默认仅回环 `127.0.0.1`；mutation 5 条 Bearer `fail-closed`；只读 7 条无鉴权（依赖回环绑定）；无 TLS | `docs/CANONICAL-RPC-SPEC.md` §2、§7 |
| 网络层（P2P） | 明文 TCP；无传输加密、无对端认证、无 peer scoring / ban / isolation、无独立协议版本协商 | `docs/CANONICAL-P2P-SPEC.md` §13 |
| 共识 / 经济 | PoW + UTXO + 确定性创世；规则见共识规范 | `docs/CANONICAL-CONSENSUS-SPEC.md` |

## 待补全（禁止臆造结论）

- 完整威胁模型、攻击矩阵、缓解清单待独立安全审计阶段产出。
- 本骨架不包含任何"已实现但未经验证"或"规划中"的安全能力声明。
