package guiapp

import internallogger "github.com/TIANLI0/THRM/internal/logger"

// GUI 内原先存在两套彼此独立的 zap logger。统一到同一个进程 logger 后，
// Linux journal 中的标识、级别和格式都与核心服务保持一致。
var (
	guiProcessLogger = internallogger.NewProcessLogger(internallogger.GUIIdentifier)
	guiLogger        = guiProcessLogger.Sugar()
	mainLogger       = guiLogger
)
