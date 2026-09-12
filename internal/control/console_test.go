package control_test

// 本文件覆盖 Developer Console 阶段新增的控制接口面：
//
//	/status 的 bits、difficulty 字段（难度是共识真值 + 派生展示量）
//	/logs   的日志端点（默认 tail、上限截断、非法参数、无来源时的诚实空态）
//	/       的内嵌控制台页面（内容类型、禁用缓存、路由边界、§19 无假数据约束）
//
// 这些用例只验证「协议层契约」，不重复 cmd/node 侧的真实链路测试。

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"

	"p2pchain/internal/control"
)

// newConsoleTestPair 在 newTestPair 基础上允许注入日志来源。
func newConsoleTestPair(t *testing.T, node control.Node, logs control.LogProvider) (*control.Client, *httptest.Server) {
	t.Helper()
	srv := control.NewServer(node)
	if logs != nil {
		srv.SetLogProvider(logs)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return control.NewClient(strings.TrimPrefix(ts.URL, "http://")), ts
}

// fakeLogs 是 control.LogProvider 的测试替身，并记录最近一次被请求的行数，
// 用于确认服务端的默认值与上限截断确实生效（而不是由实现方自行决定）。
//
// 加锁：RecentLogs 由 HTTP 处理协程调用，而断言在测试协程读取，不加锁在
// -race 下会报数据竞争（既有 fakeNode 是历史遗留写法，新代码不再沿用）。
type fakeLogs struct {
	mu      sync.Mutex
	entries []control.LogEntry
	lastN   int
}

func (f *fakeLogs) RecentLogs(n int) []control.LogEntry {
	f.mu.Lock()
	f.lastN = n
	f.mu.Unlock()

	if n <= 0 || n > len(f.entries) {
		n = len(f.entries)
	}
	out := make([]control.LogEntry, n)
	copy(out, f.entries[len(f.entries)-n:])
	return out
}

// requested 返回最近一次被请求的行数（-race 安全的读取）。
func (f *fakeLogs) requested() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastN
}

// nilLogs 模拟「已注入来源但暂时没有日志」的节点。
type nilLogs struct{}

func (nilLogs) RecentLogs(int) []control.LogEntry { return nil }

// getBody 发起 GET 并返回状态码、响应体与响应头。
func getBody(t *testing.T, url string) (int, string, http.Header) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s 失败: %v", url, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取 %s 响应失败: %v", url, err)
	}
	return resp.StatusCode, string(data), resp.Header
}

// ---- /status：bits + difficulty ----

// TestStatusCarriesBitsAndDifficulty 难度必须由节点给出，且 JSON 字段名固定。
//
// 字段名是前端契约：一旦改名，页面会静默把难度渲染成 Unavailable（不报错、
// 只是看起来"没有数据"），这类问题很难在人工点检中暴露，因此在此锁定。
func TestStatusCarriesBitsAndDifficulty(t *testing.T) {
	node := &fakeNode{status: control.StatusInfo{Height: 4, Bits: 20, Difficulty: 16}}
	client, srv := newConsoleTestPair(t, node, nil)

	st, err := client.Status()
	if err != nil {
		t.Fatalf("Status 失败: %v", err)
	}
	if st.Bits != 20 {
		t.Fatalf("Bits = %d, want 20", st.Bits)
	}
	if st.Difficulty != 16 {
		t.Fatalf("Difficulty = %v, want 16", st.Difficulty)
	}

	code, body, _ := getBody(t, srv.URL+"/status")
	if code != http.StatusOK {
		t.Fatalf("状态码 = %d, want 200", code)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		t.Fatalf("响应不是 JSON 对象: %v\n%s", err, body)
	}
	for _, key := range []string{"bits", "difficulty"} {
		if _, ok := raw[key]; !ok {
			t.Fatalf("/status 缺少字段 %q，实际响应: %s", key, strings.TrimSpace(body))
		}
	}
}

// ---- /logs ----

// TestLogsEmptyWithoutProvider 未接入日志来源时必须返回 []，不能是 null、不能报错。
//
// null 会让前端的 .map/.forEach 直接抛异常，整页卡在加载态；
// 而"伪造几条日志"则违反 §19。空数组是唯一正确选择。
func TestLogsEmptyWithoutProvider(t *testing.T) {
	client, srv := newConsoleTestPair(t, &fakeNode{}, nil)

	entries, err := client.Logs(0)
	if err != nil {
		t.Fatalf("Logs 失败: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("未注入日志来源时应返回空列表，实际 %d 条", len(entries))
	}

	code, body, hdr := getBody(t, srv.URL+"/logs")
	if code != http.StatusOK {
		t.Fatalf("状态码 = %d, want 200", code)
	}
	if got := strings.TrimSpace(body); got != "[]" {
		t.Fatalf("响应体 = %q, want []", got)
	}
	if ct := hdr.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json*", ct)
	}
}

// TestLogsProviderNilYieldsEmptyArray 来源存在但返回 nil 时同样序列化为 []。
func TestLogsProviderNilYieldsEmptyArray(t *testing.T) {
	client, srv := newConsoleTestPair(t, &fakeNode{}, nilLogs{})

	if entries, err := client.Logs(10); err != nil || len(entries) != 0 {
		t.Fatalf("Logs = %v, %v; want 空列表且无错误", entries, err)
	}
	_, body, _ := getBody(t, srv.URL+"/logs")
	if got := strings.TrimSpace(body); got != "[]" {
		t.Fatalf("响应体 = %q, want []", got)
	}
}

// TestLogsTailHandling 默认行数、显式 tail、超限截断、非法参数四条边界。
func TestLogsTailHandling(t *testing.T) {
	all := make([]control.LogEntry, 0, 150)
	for i := 0; i < 150; i++ {
		all = append(all, control.LogEntry{
			Time:      "10:00:00",
			Level:     "INFO",
			Component: "node",
			Message:   fmt.Sprintf("第 %d 行", i+1),
		})
	}
	logs := &fakeLogs{entries: all}
	client, srv := newConsoleTestPair(t, &fakeNode{}, logs)

	// 1) 不传 tail：服务端默认 100
	if _, err := client.Logs(0); err != nil {
		t.Fatalf("Logs(0) 失败: %v", err)
	}
	if logs.requested() != 100 {
		t.Fatalf("默认 tail = %d, want 100", logs.requested())
	}

	// 2) 显式 tail：只取最后 n 条，且顺序为旧→新
	got, err := client.Logs(3)
	if err != nil {
		t.Fatalf("Logs(3) 失败: %v", err)
	}
	if logs.requested() != 3 {
		t.Fatalf("透传的 tail = %d, want 3", logs.requested())
	}
	if len(got) != 3 {
		t.Fatalf("返回 %d 条, want 3", len(got))
	}
	if got[0].Message != "第 148 行" || got[2].Message != "第 150 行" {
		t.Fatalf("顺序或内容不符: %q ... %q", got[0].Message, got[2].Message)
	}

	// 3) 超限：截断到上限而不是报错（用户误传大数不该让页面整块失败）
	if _, err := client.Logs(99999); err != nil {
		t.Fatalf("Logs(99999) 失败: %v", err)
	}
	if logs.requested() != control.MaxLogTail {
		t.Fatalf("超限 tail 应截断为 %d, 实际 %d", control.MaxLogTail, logs.requested())
	}

	// 4) 非法 tail：400（不能静默当成默认值，否则前端传参错误无法察觉）
	for _, bad := range []string{"abc", "0", "-1", "1.5"} {
		code, body, _ := getBody(t, srv.URL+"/logs?tail="+bad)
		if code != http.StatusBadRequest {
			t.Fatalf("tail=%q 状态码 = %d, want 400（响应: %s）", bad, code, strings.TrimSpace(body))
		}
	}

	// 5) 空 tail（?tail=）按「未指定」处理，走默认值：这是前端最常误发的形式，
	//    必须与「非法值」区别对待，否则页面会因为一个空参数整块报错。
	code, body, _ := getBody(t, srv.URL+"/logs?tail=")
	if code != http.StatusOK {
		t.Fatalf("空 tail 状态码 = %d, want 200（响应: %s）", code, strings.TrimSpace(body))
	}
	if logs.requested() != 100 {
		t.Fatalf("空 tail 应走默认 100, 实际 %d", logs.requested())
	}
}

// TestLogsMethodNotAllowed /logs 仅接受 GET。
func TestLogsMethodNotAllowed(t *testing.T) {
	_, srv := newConsoleTestPair(t, &fakeNode{}, &fakeLogs{})

	resp, err := http.Post(srv.URL+"/logs", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("状态码 = %d, want 405", resp.StatusCode)
	}
	if got := resp.Header.Get("Allow"); got != http.MethodGet {
		t.Fatalf("Allow = %q, want GET", got)
	}
}

// ---- / 与 /console：内嵌控制台页面 ----

// TestConsoleServesEmbeddedPage 页面必须内嵌在二进制里（零外部资源、可离线打开），
// 且禁用缓存，避免开发者拿到旧页面误判后端行为。
func TestConsoleServesEmbeddedPage(t *testing.T) {
	_, srv := newConsoleTestPair(t, &fakeNode{}, nil)

	for _, path := range []string{"/", "/console"} {
		code, body, hdr := getBody(t, srv.URL+path)
		if code != http.StatusOK {
			t.Fatalf("GET %s 状态码 = %d, want 200", path, code)
		}
		if ct := hdr.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Fatalf("GET %s Content-Type = %q, want text/html*", path, ct)
		}
		if cc := hdr.Get("Cache-Control"); cc != "no-store" {
			t.Fatalf("GET %s Cache-Control = %q, want no-store", path, cc)
		}
		if !strings.Contains(body, "P2PChain Developer Console") {
			t.Fatalf("GET %s 返回内容不是控制台页面（缺标题），前 200 字节: %.200s", path, body)
		}
	}
}

// TestConsoleRouteBoundaries 未知路径 404，非 GET 方法 405，HEAD 正常。
//
// 重点：通配路由 "/" 不能把别的路径也吞掉并返回页面，否则任何拼错的
// 接口名都会得到 200 + 一坨 HTML，前端 JSON 解析失败的报错会非常难定位。
func TestConsoleRouteBoundaries(t *testing.T) {
	_, srv := newConsoleTestPair(t, &fakeNode{blockHex: "00"}, nil)

	for _, path := range []string{"/nope", "/web/console.html", "/status/extra", "/logs/"} {
		code, body, _ := getBody(t, srv.URL+path)
		if code != http.StatusNotFound {
			t.Fatalf("GET %s 状态码 = %d, want 404（响应前 120 字节: %.120s）", path, code, body)
		}
	}

	// 非 GET：405 + Allow: GET
	resp, err := http.Post(srv.URL+"/", "text/html", strings.NewReader("<x/>"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST / 状态码 = %d, want 405", resp.StatusCode)
	}
	if got := resp.Header.Get("Allow"); got != http.MethodGet {
		t.Fatalf("POST / Allow = %q, want GET", got)
	}

	// HEAD：允许（部分浏览器/健康检查会先发 HEAD）
	hr, err := http.Head(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer hr.Body.Close()
	if hr.StatusCode != http.StatusOK {
		t.Fatalf("HEAD / 状态码 = %d, want 200", hr.StatusCode)
	}
}

// TestConsoleHasNoFabricatedData 把规格 §19 从口头约定变成可回归的断言：
// 页面不得用 Math.random 造数、不得引用任何外部资源。
func TestConsoleHasNoFabricatedData(t *testing.T) {
	_, srv := newConsoleTestPair(t, &fakeNode{}, nil)
	_, body, _ := getBody(t, srv.URL+"/")

	for _, banned := range []string{"Math.random(", "mockData", "fakeData", "dummyData"} {
		if strings.Contains(body, banned) {
			t.Fatalf("控制台页面出现被禁止的伪造数据痕迹: %q", banned)
		}
	}
	for _, banned := range []string{"cdn.", "unpkg", "jsdelivr", "googleapis"} {
		if strings.Contains(body, banned) {
			t.Fatalf("控制台页面引用了外部资源 %q，破坏零依赖与离线可用", banned)
		}
	}
}

// TestConsoleEndpointLabelNotHardcoded 页面显示的端点必须来自页面自身地址。
//
// 硬编码默认端口（6689）在节点用其它端口启动时会显示一个「看起来像真数据」的
// 错误地址，比显示空值更有害——用户据此排查会找错方向，因此在此锁死。
func TestConsoleEndpointLabelNotHardcoded(t *testing.T) {
	_, srv := newConsoleTestPair(t, &fakeNode{}, nil)
	_, body, _ := getBody(t, srv.URL+"/")

	if strings.Contains(body, control.DefaultAddr) {
		t.Fatalf("页面硬编码了控制接口默认地址 %s，换端口启动将显示错误端点", control.DefaultAddr)
	}
	if !strings.Contains(body, "location.host") {
		t.Fatal("页面未从 location.host 取端点，端点标签不会是真实地址")
	}
}

// TestConsoleEveryFieldIsWired §4 要求每个生产 UI 字段都能追溯到真实数据。
//
// 判据：元素 id 必须在声明之外至少被引用一次（即真的被赋值），否则就是死字段——
// 死字段要么 REMOVE，要么显式渲染 Unavailable，不得留一个空白占位给使用者看。
func TestConsoleEveryFieldIsWired(t *testing.T) {
	_, srv := newConsoleTestPair(t, &fakeNode{}, nil)
	_, body, _ := getBody(t, srv.URL+"/")

	// 先排除「外部工具回写」这一已知干扰源：把 HTML 交给页面预览/可视化编辑器时，
	// 工具会给每个元素注入 data-page-node-id，那会让本测试报出一堆假死字段。
	// 这里直接给出可操作的错误信息，而不是让人去猜。
	if strings.Contains(body, "data-page-node-id") {
		t.Fatal("页面被外部工具回写，注入了 data-page-node-id 编辑器属性；请先剥离后再运行（见 PHASE BRAND-0D.3 报告 F-4）")
	}

	re := regexp.MustCompile(`\bid="([A-Za-z][A-Za-z0-9]*)"`)
	var dead []string
	for _, m := range re.FindAllStringSubmatch(body, -1) {
		id := m[1]
		if strings.Count(body, `"`+id+`"`) < 2 {
			dead = append(dead, id)
		}
	}
	if len(dead) > 0 {
		t.Fatalf("存在从未被赋值的死字段（应 REMOVE 或改为 Unavailable）: %v", dead)
	}
}

// TestConsoleRejectsNonFiniteValues §14：null / undefined / NaN 绝不允许被渲染成
// 正常状态，页面里也不允许出现把非数值转成字面量的代码路径。
func TestConsoleRejectsNonFiniteValues(t *testing.T) {
	_, srv := newConsoleTestPair(t, &fakeNode{}, nil)
	_, body, _ := getBody(t, srv.URL+"/")

	if strings.Contains(body, "return String(n)") {
		t.Fatal("存在把非数值转成字面字符串的路径（会渲染出 null/undefined/NaN）")
	}
	for _, need := range []string{"isFinite", "validateStatus", "shapeError", "setFailureState"} {
		if !strings.Contains(body, need) {
			t.Fatalf("缺少非有限值/畸形响应防护: %q", need)
		}
	}
}

// TestConsoleSingleFlightPolling 并发刷新必须去重（§11：不得产生重复请求），
// 且轮询注册点与间隔必须唯一、固定（§17：记录并锁定 UI 轮询频率）。
func TestConsoleSingleFlightPolling(t *testing.T) {
	_, srv := newConsoleTestPair(t, &fakeNode{}, nil)
	_, body, _ := getBody(t, srv.URL+"/")

	if !strings.Contains(body, "if (inFlight) return inFlight") {
		t.Fatal("刷新缺少单飞（single-flight）保护，连击会产生重复请求")
	}
	if n := strings.Count(body, "setInterval(tick"); n != 1 {
		t.Fatalf("轮询注册点应恰好 1 处，实际 %d 处", n)
	}
	if !strings.Contains(body, "setInterval(tick, 2000)") {
		t.Fatal("轮询间隔被改动：预期 2000ms")
	}
}

// TestConsoleStaleStateIsMarked 节点离线后，核心指标不得继续以「实时值」的面貌呈现
// （§11 禁止 stale status）：数值位必须退化为 —，最后一次成功读数只能降级到副标题。
func TestConsoleStaleStateIsMarked(t *testing.T) {
	_, srv := newConsoleTestPair(t, &fakeNode{}, nil)
	_, body, _ := getBody(t, srv.URL+"/")

	for _, need := range []string{"markStale", "最后读数"} {
		if !strings.Contains(body, need) {
			t.Fatalf("缺少离线陈旧态处理: %q", need)
		}
	}
	if !strings.Contains(body, "markStale(!v)") {
		t.Fatal("setOnline 未联动陈旧态标记，离线后指标会保持旧值")
	}
}

// TestConsoleOnlyUsesSameOriginAPI 页面只能访问自己节点的控制接口，
// 不允许出现跨站地址——否则本机无鉴权的控制接口会被第三方页面利用。
func TestConsoleOnlyUsesSameOriginAPI(t *testing.T) {
	_, srv := newConsoleTestPair(t, &fakeNode{}, nil)
	_, body, _ := getBody(t, srv.URL+"/")

	for _, pat := range []string{"fetch(\"http", "fetch('http", "XMLHttpRequest", "WebSocket"} {
		if strings.Contains(body, pat) {
			t.Fatalf("控制台页面出现绝对地址/额外通道痕迹 %q，应只使用同源相对路径", pat)
		}
	}
	if !strings.Contains(body, "fetch(API") {
		t.Fatal("控制台页面未按约定通过 API 前缀发起同源请求")
	}
}

// TestConsoleHiddenAttributeIsEffective 凡是通过 `$("x").hidden = ...` 切换可见性的元素，
// 其 class 若声明了 display，就会覆盖浏览器 UA 样式表的 [hidden]{display:none}，
// 使 hidden 语义静默失效（元素无法真正隐藏）。
//
// 真实缺陷（BRAND-0D.3 §14 发现）：#panels 带 class="grid3" 且 .grid3{display:grid}，
// 因此节点离线时 panels.hidden=true 但实际仍以 grid 显示，离线卡片与陈旧指标同时出现。
// 本用例把它固化成永久断言：只读 DOM 属性（.hidden）的检查是空断言，必须校验 CSS 生效性。
func TestConsoleHiddenAttributeIsEffective(t *testing.T) {
	_, srv := newConsoleTestPair(t, &fakeNode{}, nil)
	_, body, _ := getBody(t, srv.URL+"/")

	reToggle := regexp.MustCompile(`\$\("([A-Za-z0-9_]+)"\)\.hidden\s*=`)
	ids := map[string]bool{}
	for _, m := range reToggle.FindAllStringSubmatch(body, -1) {
		ids[m[1]] = true
	}
	if len(ids) == 0 {
		t.Fatal("未发现任何 .hidden 切换点，本断言失去意义（请同步更新测试）")
	}

	classes := map[string]bool{}
	for id := range ids {
		reEl := regexp.MustCompile(`<[^>]*id="` + regexp.QuoteMeta(id) + `"[^>]*>`)
		el := reEl.FindString(body)
		if el == "" {
			t.Fatalf("元素 #%s 在页面中不存在，.hidden 赋值会静默失效", id)
		}
		if cm := regexp.MustCompile(`class="([^"]+)"`).FindStringSubmatch(el); cm != nil {
			for _, c := range strings.Fields(cm[1]) {
				classes[c] = true
			}
		}
	}

	var conflict []string
	for c := range classes {
		re := regexp.MustCompile(`\.` + regexp.QuoteMeta(c) + `\s*\{[^}]*display\s*:`)
		if re.MatchString(body) {
			conflict = append(conflict, c)
		}
	}
	if len(conflict) == 0 {
		return // 没有任何冲突，无需兜底规则
	}
	if !strings.Contains(body, "[hidden]{display:none !important}") {
		t.Fatalf("以下 class 声明了 display，会覆盖 UA 的 [hidden]{display:none}，"+
			"令 .hidden 切换失效（元素无法真正隐藏）: %v\n"+
			"修复：在样式表中加入 [hidden]{display:none !important}", conflict)
	}
}

// TestConsoleReadRequestsHaveTimeout §11：接口「连得上但不回」时不得产生 stale status。
// 缺少超时的 fetch 会永久挂起，而轮询采用单飞去重，一次挂起会让之后所有 tick 复用同一个
// 未落定的 Promise —— 轮询彻底停摆，UI 永久停留在陈旧的 Online。
// 同时必须保证 /mine 不带这个短超时：出块耗时不确定，套用短超时会把正常出块误判为失败。
func TestConsoleReadRequestsHaveTimeout(t *testing.T) {
	_, srv := newConsoleTestPair(t, &fakeNode{}, nil)
	_, body, _ := getBody(t, srv.URL+"/")

	for _, need := range []string{
		"AbortController", "REQ_TIMEOUT_MS", "ctl.abort()",
		"clearTimeout(timer)", `e0.name === "AbortError"`,
	} {
		if !strings.Contains(body, need) {
			t.Fatalf("jget 缺少读取超时防护（接口挂起会让单飞轮询永久停摆）: %q", need)
		}
	}
	if regexp.MustCompile(`fetch\(API \+ "/mine"[\s\S]{0,400}?signal`).MatchString(body) {
		t.Fatal("/mine 不应携带读取超时信号：出块耗时不确定，短超时会把正常出块误判为失败")
	}
}

// TestConsoleHasNoMarkdownLeak 页面是 HTML，不是 Markdown。
// 写入粗体时必须用 <b>，写成 **粗体** 会在浏览器里原样显示星号（真实出现过两次：
// 披露块的「未暴露」与难度说明），属于面向最终读者的内容缺陷，必须永久拦截。
func TestConsoleHasNoMarkdownLeak(t *testing.T) {
	_, srv := newConsoleTestPair(t, &fakeNode{}, nil)
	_, body, _ := getBody(t, srv.URL+"/")

	if n := strings.Count(body, "**"); n != 0 {
		idx := strings.Index(body, "**")
		lo := idx - 60
		if lo < 0 {
			lo = 0
		}
		hi := idx + 60
		if hi > len(body) {
			hi = len(body)
		}
		t.Fatalf("页面出现 %d 处 Markdown 粗体语法 '**'，会以字面星号呈现给读者。上下文: %q",
			n, body[lo:hi])
	}
	// 同理：Markdown 标题/列表标记不应出现在正文文本里
	for _, pat := range []string{"## ", "- [ ] "} {
		if strings.Contains(body, pat) {
			t.Fatalf("页面文本出现 Markdown 标记 %q", pat)
		}
	}
}
