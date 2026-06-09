//go:build !windows

package main

func setupFatalOutput() (func(), string) {
	return func() {}, ""
}
