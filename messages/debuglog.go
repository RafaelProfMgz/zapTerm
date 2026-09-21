package messages

import (
	"fmt"
	"os"

	waLog "go.mau.fi/whatsmeow/util/log"
)

// stderrLogger sends whatsmeow's logs to stderr. stdout carries the NDJSON
// protocol of the Ink frontend, so the library must never write there.
type stderrLogger struct {
	module string
	level  string
}

func (l stderrLogger) output(level, msg string, args ...any) {
	if level == "DEBUG" && l.level != "DEBUG" {
		return
	}
	fmt.Fprintf(os.Stderr, "[%s %s] %s\n", l.module, level, fmt.Sprintf(msg, args...))
}

func (l stderrLogger) Errorf(msg string, args ...any) { l.output("ERROR", msg, args...) }
func (l stderrLogger) Warnf(msg string, args ...any)  { l.output("WARN", msg, args...) }
func (l stderrLogger) Infof(msg string, args ...any)  { l.output("INFO", msg, args...) }
func (l stderrLogger) Debugf(msg string, args ...any) { l.output("DEBUG", msg, args...) }

func (l stderrLogger) Sub(module string) waLog.Logger {
	return stderrLogger{module: l.module + "/" + module, level: l.level}
}

// waLogger is whatsmeow's logger: silent by default, on stderr when
// ZAPTERM_DEBUG is set (=debug for the full protocol trace). Diagnosing a
// connection that never produces a QR code is impossible without it.
func waLogger(module string) waLog.Logger {
	level := os.Getenv("ZAPTERM_DEBUG")
	if level == "" {
		return waLog.Noop
	}
	if level != "debug" && level != "DEBUG" {
		level = "INFO"
	} else {
		level = "DEBUG"
	}
	return stderrLogger{module: module, level: level}
}
