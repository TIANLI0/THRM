// Package mainthread 把必须在进程主线程上执行的调用（macOS 的 AppKit/NSApp）
// 排队交给 main.main 所在线程。
//
// 只有 macOS 有这个限制：NSApplication 只能在主线程创建和运行，否则 AppKit
// 直接抛 NSInternalInconsistencyException 让进程崩溃。Windows、Linux 上 Run
// 原样调用 fn，Call 就地执行 f，行为与引入本包之前完全一致。
package mainthread
