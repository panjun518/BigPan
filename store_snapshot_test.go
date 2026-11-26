package main

// 回归测试：盯住 A3 落盘修复（权限 0600 / 上一版备份 / 失败打日志）。
import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteSnapshotBackupAndPerm(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "items.json")
	// 先放一版「旧数据」，模拟线上已有的 24MB items.json
	old := []Item{{Title: "旧条目"}}
	ob, _ := json.Marshal(old)
	if err := os.WriteFile(p, ob, 0644); err != nil {
		t.Fatal(err)
	}
	s := &Store{path: p}
	s.writeSnapshot([]Item{{Title: "新条目A"}, {Title: "新条目B"}})

	st, err := os.Stat(p)
	if err != nil {
		t.Fatal("落盘后主文件不存在:", err)
	}
	if st.Mode().Perm() != 0600 {
		t.Errorf("items.json 权限应为 0600，实际 %o", st.Mode().Perm())
	}
	bak, err := os.ReadFile(p + ".bak")
	if err != nil {
		t.Fatal("缺少上一版备份 .bak:", err)
	}
	if !strings.Contains(string(bak), "旧条目") {
		t.Errorf(".bak 内容不是上一版: %s", bak)
	}
	cur, err := os.ReadFile(p)
	if err != nil || !strings.Contains(string(cur), "新条目A") {
		t.Errorf("主文件不是最新数据: %s err=%v", cur, err)
	}
}

func TestWriteSnapshotLogsOnFailure(t *testing.T) {
	// 指向一个不存在的目录 → WriteFile 必失败，必须打日志而不是静默 return
	p := filepath.Join(t.TempDir(), "no-such-dir", "items.json")
	var buf bytes.Buffer
	oldOut := log.Writer()
	oldFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	defer func() { log.SetOutput(oldOut); log.SetFlags(oldFlags) }()

	(&Store{path: p}).writeSnapshot([]Item{{Title: "x"}})
	if !strings.Contains(buf.String(), "[store] 落盘失败") {
		t.Errorf("落盘失败没打日志，日志内容: %q", buf.String())
	}
}
