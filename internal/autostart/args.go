package autostart

// IsInstallAutoStartRequest 判断命令行是否要求"仅配置开机自启动后退出"。
//
// 该模式由安装器调用（见 cmd/core 的 install_autostart.go），必须与 --autostart
// 严格区分：后者表示"本次是自启动运行"，若被误判成安装模式，每次开机核心就只是
// 重写一遍自启动配置然后退出，控温彻底不工作。
func IsInstallAutoStartRequest(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "--install-autostart", "/install-autostart", "-install-autostart":
			return true
		}
	}
	return false
}
