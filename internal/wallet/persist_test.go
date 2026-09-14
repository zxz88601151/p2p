// PHASE WALLET-PERSISTENCE-HARDENING-1：SaveToFile 原子写（方案 B1）失败注入与不变量测试。
//
// 覆盖审计报告 §7 的 T2–T9。T1（正常 save/load 往返）由 wallet_ext_test.go 的
// TestWalletSaveLoadRoundTrip 沿用，此处不重复实现。
//
// 真实断电/硬杀边界（W2–W6）无法用单元测试模拟，属于
// REQUIRES DISPOSABLE CRASH SIMULATION，由阶段独立的 crash validation 覆盖。
package wallet

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// withHooks 临时替换注入点，测试结束自动恢复（同包测试默认串行，无并发竞争）。
func withHooks(t *testing.T, write func(*os.File, []byte) error, sync func(*os.File) error, rename func(string, string) error) {
	t.Helper()
	ow, os_, or := hookWrite, hookSync, hookRename
	if write != nil {
		hookWrite = write
	}
	if sync != nil {
		hookSync = sync
	}
	if rename != nil {
		hookRename = rename
	}
	t.Cleanup(func() { hookWrite, hookSync, hookRename = ow, os_, or })
}

func newTestWallet(t *testing.T) *Wallet {
	t.Helper()
	w, err := NewWallet()
	if err != nil {
		t.Fatalf("生成钱包失败: %v", err)
	}
	return w
}

func mustSave(t *testing.T, w *Wallet, path string) {
	t.Helper()
	if err := w.SaveToFile(path); err != nil {
		t.Fatalf("SaveToFile 失败: %v", err)
	}
}

func fileBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s 失败: %v", path, err)
	}
	return b
}

var errInjected = errors.New("injected failure")

// T2：成功 Save 后同目录不存在 tmp。
func TestSaveNoTmpResidueOnSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wallet.json")
	mustSave(t, newTestWallet(t), path)
	if _, err := os.Stat(path + ".tmp"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("成功保存后 tmp 残留存在（%v）", err)
	}
}

// T2（失败路径部分）：rename 失败后 tmp 被 best-effort cleanup。
func TestTmpCleanedUpAfterRenameFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wallet.json")
	withHooks(t, nil, nil, func(string, string) error { return errInjected })
	if err := newTestWallet(t).SaveToFile(path); err == nil {
		t.Fatal("注入 rename 失败后 SaveToFile 应返回错误")
	}
	if _, err := os.Stat(path + ".tmp"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rename 失败后 tmp 未被清理（%v）", err)
	}
}

// T3：对已有 wallet 执行 Save → 新 wallet 完整写入、可加载、地址变为新地址（atomic replacement 成立）。
func TestSaveReplacesExistingWalletAtomically(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wallet.json")
	old := newTestWallet(t)
	mustSave(t, old, path)

	fresh := newTestWallet(t)
	mustSave(t, fresh, path)

	loaded, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("replacement 后加载失败: %v", err)
	}
	if loaded.Address() != fresh.Address() {
		t.Fatalf("replacement 后地址错误: got=%s want=%s", loaded.Address(), fresh.Address())
	}
	if loaded.Address() == old.Address() {
		t.Fatal("replacement 未生效：地址仍为旧钱包")
	}
}

// T4：write failure → 原 wallet 字节级完全不变且仍可加载。
func TestWriteFailureKeepsOldWallet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wallet.json")
	old := newTestWallet(t)
	mustSave(t, old, path)
	before := fileBytes(t, path)

	withHooks(t, func(*os.File, []byte) error { return errInjected }, nil, nil)
	if err := newTestWallet(t).SaveToFile(path); err == nil {
		t.Fatal("注入 write 失败后 SaveToFile 应返回错误")
	}
	after := fileBytes(t, path)
	if string(before) != string(after) {
		t.Fatal("write 失败后旧 wallet 字节被改动")
	}
	if _, err := LoadFromFile(path); err != nil {
		t.Fatalf("write 失败后旧 wallet 不可加载: %v", err)
	}
}

// T6：Sync failure → 原 wallet 保护 + 返回错误。
func TestSyncFailureKeepsOldWallet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wallet.json")
	old := newTestWallet(t)
	mustSave(t, old, path)
	before := fileBytes(t, path)

	withHooks(t, nil, func(*os.File) error { return errInjected }, nil)
	if err := newTestWallet(t).SaveToFile(path); err == nil {
		t.Fatal("注入 sync 失败后 SaveToFile 应返回错误")
	}
	if string(before) != string(fileBytes(t, path)) {
		t.Fatal("sync 失败后旧 wallet 字节被改动")
	}
	if _, err := LoadFromFile(path); err != nil {
		t.Fatalf("sync 失败后旧 wallet 不可加载: %v", err)
	}
}

// T7：Rename failure → 原 wallet 保持完整、Save 返回错误、tmp 已清理。
func TestRenameFailureKeepsOldWallet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wallet.json")
	old := newTestWallet(t)
	mustSave(t, old, path)
	before := fileBytes(t, path)

	withHooks(t, nil, nil, func(string, string) error { return errInjected })
	if err := newTestWallet(t).SaveToFile(path); err == nil {
		t.Fatal("注入 rename 失败后 SaveToFile 应返回错误")
	}
	if string(before) != string(fileBytes(t, path)) {
		t.Fatal("rename 失败后旧 wallet 字节被改动")
	}
	if _, err := LoadFromFile(path); err != nil {
		t.Fatalf("rename 失败后旧 wallet 不可加载: %v", err)
	}
	if _, err := os.Stat(path + ".tmp"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rename 失败后 tmp 未清理（%v）", err)
	}
}

// T8（无既有文件变体）：首次创建失败后 wallet.json 表现为「不存在」，
// 不存在半写状态或损坏 JSON；LoadFromFile 状态明确（ErrNotExist 类错误）。
func TestFirstCreateFailureLeavesNoHalfWrittenWallet(t *testing.T) {
	for name, inject := range map[string]func(t *testing.T){
		"write-failure":  func(t *testing.T) { withHooks(t, func(*os.File, []byte) error { return errInjected }, nil, nil) },
		"sync-failure":   func(t *testing.T) { withHooks(t, nil, func(*os.File) error { return errInjected }, nil) },
		"rename-failure": func(t *testing.T) { withHooks(t, nil, nil, func(string, string) error { return errInjected }) },
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "wallet.json")
			inject(t)
			if err := newTestWallet(t).SaveToFile(path); err == nil {
				t.Fatalf("%s: SaveToFile 应返回错误", name)
			}
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("%s: wallet.json 不应存在（%v）", name, err)
			}
			if _, err := LoadFromFile(path); err == nil {
				t.Fatalf("%s: 不存在的 wallet 不应加载成功", name)
			}
		})
	}
}

// T9：已有 wallet 在新写入失败期间 old identity/bytes 保持，新内容不破坏旧内容
// （T4/T7 的 identity 断言变体，覆盖预置文件 + 全部三类注入）。
func TestOldIdentityPreservedAcrossAllFailureInjections(t *testing.T) {
	for name, inject := range map[string]func(t *testing.T){
		"write-failure":  func(t *testing.T) { withHooks(t, func(*os.File, []byte) error { return errInjected }, nil, nil) },
		"sync-failure":   func(t *testing.T) { withHooks(t, nil, func(*os.File) error { return errInjected }, nil) },
		"rename-failure": func(t *testing.T) { withHooks(t, nil, nil, func(string, string) error { return errInjected }) },
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "wallet.json")
			old := newTestWallet(t)
			mustSave(t, old, path)
			before := fileBytes(t, path)

			inject(t)
			if err := newTestWallet(t).SaveToFile(path); err == nil {
				t.Fatalf("%s: SaveToFile 应返回错误", name)
			}
			if string(before) != string(fileBytes(t, path)) {
				t.Fatalf("%s: 旧 wallet 字节被改动", name)
			}
			loaded, err := LoadFromFile(path)
			if err != nil {
				t.Fatalf("%s: 旧 wallet 加载失败: %v", name, err)
			}
			if loaded.Address() != old.Address() {
				t.Fatalf("%s: identity 变化: got=%s want=%s", name, loaded.Address(), old.Address())
			}
		})
	}
}
