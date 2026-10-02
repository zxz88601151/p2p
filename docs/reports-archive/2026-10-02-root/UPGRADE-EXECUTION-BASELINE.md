# PHASE P2PCHAIN — RC CLOUD UPGRADE EXECUTION BASELINE
## 云节点 RC 升级执行前基线（§0）

- **阶段**：`PHASE-P2PCHAIN-RC-CLOUD-UPGRADE-EXECUTION-1` §0
- **时间**：`2026-10-02 11:01 CST`
- **RC binary SHA256**：`b26181173ba1ce8795a598f89c1fb5ee64f84c7cf5b2fbe0fa77709ad24877bb`
- **源码**：tag v0.9.0-rc1 / commit 2647ba0

---

## 升级前基线（before）

| 节点 | PID | 角色 | P2P 监听 | RPC | DataDir | seed |
|------|-----|------|----------|-----|---------|------|
| A | 2872772 | seed 枢纽 | 16688 | 16689 | /data/p2pchain/blockchain | — |
| B | 2873329 | -seed→A | 16690 | 16691 | /data/p2pchain/blockchain-b | 127.0.0.1:16688 |
| C | 2872762 | -seed→A | 16692 | 16693 | /data/p2pchain/blockchain-c | 127.0.0.1:16688 |

| 项 | 值 |
|----|----|
| 当前 binary SHA256 | `a95df7c9a960fd1896ff4d5b1e2504ea0778fcba87961f6423f8a9e970c5f217` |
| height | 136（三节点一致） |
| tip hash | `0000e923a6f536b23dc8dc1ebe1e5d4b27d5fce657d7d7507e4d63fed09ed544`（三节点一致） |
| peers | A↔B、A↔C 星型 |

## 精确启动命令（从 /proc/<pid>/cmdline 提取，升级后必须原样复原）

```bash
# A（seed，无 -seed 参数）
/opt/p2pchain/bin/p2pchain -listen 127.0.0.1:16688 -rpc 127.0.0.1:16689 -datadir /data/p2pchain/blockchain -auth-token-file /opt/p2pchain/secrets/control-token

# B
/opt/p2pchain/bin/p2pchain -listen 127.0.0.1:16690 -rpc 127.0.0.1:16691 -datadir /data/p2pchain/blockchain-b -auth-token-file /opt/p2pchain/secrets/control-token -seed 127.0.0.1:16688

# C
/opt/p2pchain/bin/p2pchain -listen 127.0.0.1:16692 -rpc 127.0.0.1:16693 -datadir /data/p2pchain/blockchain-c -auth-token-file /opt/p2pchain/secrets/control-token -seed 127.0.0.1:16688
```

## 升级顺序

**C（2872762）→ B（2873329）→ A（2872772，seed 最后）**

## 数据目录（升级前 hash，禁止改动）

| DataDir | blocks.dat | wallet.json |
|---------|-----------|-------------|
| blockchain (A) | `9035fb101cd7f46c503a1f7d1b4e370eeea0e1f0ca4f3096ab471b0b1a6227d0` | `02f19c1da42dd06e76981c98c0a98721dbda117ddaa7f7c4c6b289258f1e7c32` |
| blockchain-b (B) | `ea733ecebdda18dd596ca174f68236a8880425a10515ae87118ef6546ca9c97e` | `c36a44b3da9d44ce0582b8dbcceb3b5d2e015e08d29681da362af01aee9a4087` |
| blockchain-c (C) | `a1c606c2e91f4310d0889c7bdfaf53754051df03c360bd8e72a689cbc7e4d6ee` | `da7342364f2002605acea7c683030f2a5e123f82c3248b2edaabd2eef2fb9701` |
