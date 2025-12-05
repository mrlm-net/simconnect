//go:build windows
// +build windows

package log

func New() Logger {
	return &Engine{}
}

type Logger interface {
	//Printf(format string, v ...interface{})
}

type Engine struct {
}
