package main

// 回归测试：分段扫描时，后台任务必须把「最新全量文件」回写 batchParseCache。
//
// 背景（2026-10-26 线上问题）：v2.2.1 加了后台续扫，但只把全量放在 job 里，
// 解析缓存还停在第 1 段的几十个文件；入库走缓存 → 「扫出几千个、只导入几十个」。
// 这个测试盯住两条：
//  1. 任务在跑的时候，缓存里的文件数要跟着涨（等于 job 的文件数）；
//  2. 入库取数时 job 比缓存多就用 job（谁多信谁）。

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestDbTaskWritesBackCache(t *testing.T) {
	// 真机测试：默认跳过。需要真跑时设 BIGPAN_LIVE=1 并准备 /tmp/dbacc.json。
	// 原先无条件读这个文件——沙箱里恰好残留一份凭证就会联网扫真实豆包目录，
	// 撞上限流后 100 秒超时判 FAIL，clone 下来的人 go test 直接红。
	if os.Getenv("BIGPAN_LIVE") == "" {
		t.Skip("真机测试：设 BIGPAN_LIVE=1 才跑（需 /tmp/dbacc.json 与可用网络）")
	}
	b, err := os.ReadFile("/tmp/dbacc.json")
	if err != nil {
		t.Skip("没有 /tmp/dbacc.json")
	}
	var acc []struct {
		Cookie string `json:"cookie"`
	}
	json.Unmarshal(b, &acc)
	cli := newDoubao(acc[0].Cookie)
	if cli.v2Key == nil {
		t.Skip("没有 DPoP keypair")
	}

	// 「天剑豆包影视/电影」整棵树：281 个目录，必然分段
	const nodeID = "22373253810384642"
	title, files, pending, err := scanDoubaoOwn(cli, "doubao1", nodeID, "电影", dbScanBudget)
	if err != nil {
		t.Fatalf("第一段失败: %v", err)
	}
	t.Logf("第一段：%d 个文件, pending=%v", len(files), pending)
	if !pending {
		t.Skip("这个目录没触发分段，跳过")
	}

	shareID := doubaoNodeIDPrefix + nodeID
	key := "own|doubao1|" + nodeID
	task := dbTaskStart(key, "doubao1", shareID, title, files,
		func() (string, []ShareFile, bool, error) {
			return scanDoubaoOwn(cli, "doubao1", nodeID, "电影", dbScanBudget)
		})
	if task == nil {
		t.Fatal("任务没起来")
	}

	// 等后台再扫一段：轮询到「比第一段多」就继续，不必死等 55 秒
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if cur := task.asShareInfo(); len(cur.Files) > len(files) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	snap := task.asShareInfo()
	t.Logf("后台扫了一段后：job=%d 个文件", len(snap.Files))

	cached, ok := batchParseCache.get("doubao1", shareID)
	if !ok {
		t.Fatal("缓存里没有这个分享 —— 后台任务没回写")
	}
	t.Logf("缓存里：%d 个文件", len(cached.Files))

	if len(snap.Files) <= len(files) {
		t.Logf("后台还没扫出更多（可能刚好没新文件），没法判断回写是否生效")
	}
	if len(cached.Files) != len(snap.Files) {
		t.Fatalf("缓存与任务不一致：cache=%d job=%d（入库会少文件）", len(cached.Files), len(snap.Files))
	}
	if len(cached.Files) <= 0 {
		t.Fatal("缓存文件数为 0")
	}
}
