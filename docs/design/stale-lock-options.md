# Stale-lock Recovery 候选方案对比（仅记录，未实现）

> 阶段：PHASE P3.1 §5 要求留存的扩展点文档。  
> 性质：**仅记录结论与风险，不实现任何自动 stale-lock 删除/恢复。**  
> 当前政策（P2.1 + P3.1 维持不变）：禁止 lock exists → PID not running → 自动删除 → 继续启动；
> 异常残留 lock 必须由用户手动 `rm <datadir>/node.lock` 后重启。

---

## 背景

当前 `<datadir>/node.lock` 以 `O_CREATE|O_EXCL` 原子独占创建，内容记录 `pid` 与 `started_at`。
进程正常退出 / 收到 SIGINT·SIGTERM / 初始化失败 / panic（P3.1 已加固）都会释放锁；但**进程被
`taskkill /F`、`kill -9` 等不可捕获方式强杀时，锁会残留**（内容 pid 指向已退出进程）。此时同目录
再次启动会正确拒绝（DATADIR_LOCKED），但需用户手动清理。本文件评估两种未来可能的自动恢复方案。

---

## 候选 A — PID Liveness Check（检测 lock 内 pid 是否仍存活）

思路：启动时若发现 `node.lock` 已存在，读取其中 `pid` 字段，探测该 pid 对应进程是否仍存活；
若**不存活**，认为锁为残留，提示/允许清理后继续启动。

探测方式（跨平台）：
- Unix：`kill -0 <pid>`（或 `os.FindProcess(pid).Signal(syscall.Signal(0))`）返回 nil 表示进程存在。
- Windows：`OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, FALSE, pid)` 成功即存在；或 `tasklist` 解析。

### 优点
- 零外部依赖，标准库即可实现（`os.FindProcess` 跨平台）。
- 不引入平台特定的文件锁语义，与现有 `O_EXCL` 存在性锁正交，改动面小。
- 信息充分：lock 已记录 `pid` 与 `started_at`，可直接复用。

### 风险 / 缺陷
- **PID 复用竞态**：操作系统回收 pid 后可被新进程复用；误判「残留锁对应的 pid 仍存活」→ 误放行
  或误删新进程的锁。需配合 `started_at` / 进程启动时间二次校验，仍非 100% 可靠。
- **跨权限不可见**：低权限进程探测高权限 pid 可能被拒（Windows `OpenProcess` 失败），需把「不可探测」
  保守当成「存活」（不自动删），否则会误删他人锁。
- 即便检测到「不存活」，自动删除仍属 P3.1 §5 明确禁止的「自动 stale-lock 删除」——若要落地，
  **必须**放在显式用户确认之后（如单独的 `node unlock --datadir=...` 子命令，且打印 pid/started_at 供人工核对），
  绝不在 `newNodeRuntime` 启动时静默执行。

---

## 候选 B — OS Advisory Lock（flock / LockFileEx）

思路：放弃「存在性」语义，改用操作系统级咨询锁持有 `node.lock`：
- Unix：`syscall.Flock(fd, LOCK_EX|LOCK_NB)`。
- Windows：`LockFileEx` / `LockFile` 对文件区间加独占锁。

进程崩溃时操作系统自动回收该锁，因此**从根本上消除了 stale-lock**——崩溃后锁自动释放，
同目录可立即重启，无需手动 `rm`。

### 优点
- 崩溃后锁由 OS 自动释放，**彻底消除 stale-lock 问题**，无需任何恢复逻辑。
- 语义更精确：锁代表「进程当前持有」，而非「文件存在过」。

### 风险 / 缺陷
- **平台相关**：Unix `flock` 与 Windows `LockFileEx` API 不同，需要 `build tag` 分平台实现
  （与项目「标准库零依赖、跨平台」基调冲突，P3.1 §5 明确禁止引入此类实现）。
- **改变锁语义**：咨询锁持有期间，其他进程**连只读打开该文件都可能被拒**（尤其 Windows），
  与当前「lock 仅作存在性标记、内容可被读取用于诊断」的设计不一致。
- 不防「用户手动 `rm` 残留锁后又启动」之外的场景；且若进程以只读方式打开他人锁做诊断会被阻塞。
- 与现有 `O_EXCL` 存在性 + `pid` 校验释放（P3.1 §2.2）机制需整体替换，重构面大。

---

## 结论与推荐

- **当前（P3.1）保持手动恢复政策**：残留 lock 由用户 `rm` 后重启，简单、可审计、零风险。
- 若未来确有「减少人工干预」需求，**优先候选 A（PID Liveness Check）**：零依赖、与现状正交、
  改动小；但**只能**以独立 `node unlock` 子命令 + 显式人工确认的方式暴露，绝不在启动时静默自动删除
  （严守 P3.1 §5）。
- 候选 B（OS advisory lock）虽能根治 stale-lock，但引入平台代码、改变锁语义、与「零依赖跨平台」
  基调冲突，**不推荐**纳入本项目；仅作为「若未来切换到强锁语义」的备选记录。
- 无论选哪个，都必须保留 `ErrDatadirLocked` 的明确拒绝与用户引导文案（P3.1 §2.5），
  且与回放失败（REPLAY FAILURE）错误语义保持独立。
