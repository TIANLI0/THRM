//go:build linux

package logger

import (
	"io"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func newPlatformBackend(atom zap.AtomicLevel, _, identifier string, _ bool) (platformBackend, error) {
	sink, err := openJournalSink(identifier)
	if err != nil {
		// 非 systemd 环境（容器、精简发行版等）仍应能够启动；标准错误是最通用的
		// 降级出口，也能被其他服务管理器接管。
		return platformBackend{
			cores: []zapcore.Core{consoleCore(atom, zapcore.AddSync(os.Stderr), false)},
		}, nil
	}

	return platformBackend{
		cores:   []zapcore.Core{newJournalCore(atom, sink)},
		closers: []io.Closer{sink},
	}, nil
}

// defaultLogDir 保留原有包内辅助函数的语义入口。Linux 使用 journal，没有文件目录。
func defaultLogDir(_ string) string {
	return ""
}
