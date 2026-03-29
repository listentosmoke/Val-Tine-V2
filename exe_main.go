//go:build !dll

package main

func main() {
	runAgent()
}

// Sideload stubs — only functional in DLL build

func installSideload() string {
	return "Sideload only available in DLL build (build with -tags dll -buildmode=c-shared)"
}

func uninstallSideload() string {
	return "Sideload only available in DLL build"
}
