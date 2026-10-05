//go:build !darwin

package desktop

// InstallWebViewHooks is only needed on macOS. WebView2 (Windows) and
// WebKitGTK (Linux) implement window.confirm() and window.print() themselves.
func InstallWebViewHooks() {}
