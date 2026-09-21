package blockchain

// timestamp_clamp_internal_test.go —— R3 clampMiningTimestamp 纯函数的分支
// 矩阵（内部包测试，直接访问非导出函数）。
//
// 三分支 + 窗口边界：
//   now < mtp+1            → mtp+1（下界：MTP 追上来）
//   mtp+1 <= now <= mtp+7200 → now（正常路径恒等）
//   now > mtp+7200         → mtp+7200（上界：停滞 >2h 后恢复，防模板死锁）

import "testing"

func TestClampMiningTimestampBranches(t *testing.T) {
	const mtp = int64(1_000_000)
	cases := []struct {
		name string
		now  int64
		want int64
	}{
		{"now 远小于下界", mtp - 5000, mtp + 1},
		{"now 恰在 mtp（越下界）", mtp, mtp + 1},
		{"now 恰在下边界 mtp+1", mtp + 1, mtp + 1},
		{"now 窗口内", mtp + 3600, mtp + 3600},
		{"now 恰在上边界 mtp+7200", mtp + 7200, mtp + 7200},
		{"now 恰越上界 mtp+7201", mtp + 7201, mtp + 7200},
		{"now 远超上界（停滞 1 天后恢复）", mtp + 86400, mtp + 7200},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := clampMiningTimestamp(c.now, mtp); got != c.want {
				t.Fatalf("clampMiningTimestamp(now=%d, mtp=%d) = %d, want %d", c.now, mtp, got, c.want)
			}
		})
	}
}
