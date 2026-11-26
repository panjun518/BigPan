package main

// 构建信息与健康检查。
// 版本号由 Dockerfile / CI 用 -ldflags "-X main.buildVersion=..." 注入，
// 未注入时显示 dev，方便一眼看出盒子上跑的到底是哪一版。
import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"
)

var (
	buildVersion = "dev"
	buildCommit  = "none"
	buildTime    = "unknown"
)

var (
	flagVersion = flag.Bool("version", false, "打印版本信息后退出")
	flagHealthz = flag.Bool("healthcheck", false, "容器内健康检查：探测本机 /healthz 后退出")
)

func printVersion() {
	fmt.Printf("BigPan %s\n  commit: %s\n  built:  %s\n", buildVersion, buildCommit, buildTime)
}

// apiVersion 需要登录，跟其它 /api 一致。
func apiVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{
		"version": buildVersion,
		"commit":  buildCommit,
		"built":   buildTime,
	})
}

// healthz 公开：只回状态和版本，不含凭据、条目或账号信息。
func healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "version": buildVersion})
}

// healthcheckExit 供 scratch 镜像做 HEALTHCHECK：镜像里没有 curl/wget，
// 让二进制自己探一次本机 /healthz，成功退 0、失败退 1。
func healthcheckExit(port int) int {
	c := &http.Client{Timeout: 3 * time.Second}
	resp, err := c.Get(fmt.Sprintf("http://127.0.0.1:%d/healthz", port))
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "healthcheck: HTTP %d\n", resp.StatusCode)
		return 1
	}
	return 0
}
