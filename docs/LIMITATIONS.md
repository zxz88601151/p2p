# P2PChain — Known Limitations（Skeleton）

> **状态：SKELETON / 待补全。** 本文件为最小骨架，仅登记**已确认**的已知限制；
> **不声称未来不存在的限制，也不将规划中的修复列为已实现**。

## 已确认限制（指向已验证文档）

| ID | 层 | 限制 | 权威来源 |
|---|---|---|---|
| LTM-001 | P2P | 出站队列满 → 消息被丢弃 → 无重传（静默丢块/交易） | `docs/CANONICAL-P2P-SPEC.md` §12.2 |
| REORG-1G/B5 | P2P | 完整 orphan pool 与重播策略**明确未做** | `docs/CANONICAL-P2P-SPEC.md` §8 |
| LTM-002 | P2P | 无 peer scoring / ban / isolation | `docs/CANONICAL-P2P-SPEC.md` §13 |
| LTM-003 | P2P | 无独立 P2P 协议版本 / 能力协商字段 | `docs/CANONICAL-P2P-SPEC.md` §13 |

## 待补全

- 完整限制登记待独立审计阶段产出。
- 本骨架不将任何"规划中"的修复列为已实现。
