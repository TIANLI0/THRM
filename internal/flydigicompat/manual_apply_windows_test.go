//go:build windows

package flydigicompat

import (
	"fmt"
	"os"
	"testing"
)

// TestManualApplyRevert 会真的改写系统上的设备安全描述符，因此默认跳过。
//
//	THRM_COMPAT_MANUAL=apply   go test ./internal/flydigicompat/ -run TestManualApplyRevert -v
//	THRM_COMPAT_MANUAL=revert  go test ./internal/flydigicompat/ -run TestManualApplyRevert -v
//
// 需要管理员权限。
func TestManualApplyRevert(t *testing.T) {
	mode := os.Getenv("THRM_COMPAT_MANUAL")
	if mode == "" {
		t.Skip("设置 THRM_COMPAT_MANUAL=apply|revert 才会执行")
	}

	dir := t.TempDir()
	if custom := os.Getenv("THRM_COMPAT_STATE_DIR"); custom != "" {
		dir = custom
	}

	var (
		st  Status
		err error
	)
	switch mode {
	case "apply":
		st, err = Apply(nil, dir)
	case "revert":
		st, err = Revert(nil, dir)
	default:
		t.Fatalf("未知模式: %s", mode)
	}
	if err != nil {
		t.Fatalf("%s 失败: %v", mode, err)
	}

	effective := "未知(无在线设备)"
	if st.Effective != nil {
		effective = fmt.Sprintf("%v", *st.Effective)
	}
	t.Logf("%s 完成 -> 节点=%d 已写入=%d 在线=%d 生效=%s 需重连=%v",
		mode, st.TotalNodes, st.AppliedNodes, st.PresentNodes, effective, st.NeedsReconnect)
}
