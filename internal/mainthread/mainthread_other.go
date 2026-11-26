//go:build !darwin

package mainthread

// Run 在非 macOS 平台上直接执行 fn。
func Run(fn func()) { fn() }

// Call 在非 macOS 平台上就地执行 f。
func Call(f func()) { f() }
