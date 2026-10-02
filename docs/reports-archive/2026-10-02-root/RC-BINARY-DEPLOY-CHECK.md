# PHASE P2PCHAIN — RC BINARY DEPLOY CHECK
## RC Linux binary 部署可行性检查（本地，只读）

- **阶段**：`PHASE-P2PCHAIN-RC-CLOUD-UPGRADE-EXECUTION-READINESS-1` §2
- **性质**：本地只读检查 RC binary 产物，未上传、未替换。
- **时间**：`2026-10-02 11:00 CST`

---

## 产物身份

| 项 | 值 |
|----|----|
| 文件路径 | `release/v0.9.0-rc1/p2pchain-linux-amd64-v0.9.0-rc1` |
| 文件大小 | 11,950,947 bytes |

## SHA256 校验

| 项 | 值 |
|----|----|
| 实测 SHA256 | `b26181173ba1ce8795a598f89c1fb5ee64f84c7cf5b2fbe0fa77709ad24877bb` |
| 期望 SHA256 | `b26181173ba1ce8795a598f89c1fb5ee64f84c7cf5b2fbe0fa77709ad24877bb` |
| 结果 | ✅ **PASS** |

## 架构与目标平台

| 项 | 实测值 | 期望 | 结果 |
|----|--------|------|------|
| ELF magic | `7f 45 4c 46` | ELF | ✅ |
| ELF class（第5字节） | `02` | 64-bit | ✅ |
| machine | `0x3e` | x86-64 (amd64) | ✅ |
| GOOS | `linux` | linux | ✅ |
| GOARCH | `amd64` | amd64 | ✅ |
| CGO_ENABLED | `0` | 0 | ✅ |
| trimpath | `true` | true | ✅ |
| buildvcs | vcs.revision 内嵌 = **0** | false | ✅ |
| Go 版本 | go1.27.0 | — | ✅ |

## 与云上运行 binary 对比

| 项 | 云上（当前） | RC（待部署） |
|----|-------------|-------------|
| 大小 | 11,921,942 | 11,950,947 |
| SHA256 | `a95df7c9...` | `b2618117...` |
| vcs.revision | `c3cec3f3`（内嵌） | 无内嵌（buildvcs=false） |
| 源码基线 | c3cec3f3 | 2647ba0（v0.9.0-rc1） |
| trimpath | true | true |

## 结论

✅ **RC Linux binary 部署可行性 PASS**——架构、目标平台、构建参数、SHA256 全部符合 RC 发布基准，可安全用于云上部署（待 Owner 授权后）。
