package app

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const banner = "\033[36m" + `
    _   __          _   __          __
   / | / /__  ____ / | / /__  _____/ /_
  /  |/ / _ \/ __ \/  |/ / _ \/ ___/ __/
 / /|  /  __/ /_/ / /|  /  __/ /__/ /_
/_/ |_/\___/\____/_/ |_/\___/\___/\__/
` + "\033[0m\n"

type AppLogger struct {
	debug      bool
	isTerminal bool
}

func newAppLogger(debug bool) *AppLogger {
	isTerminal := false
	if fileInfo, err := os.Stdout.Stat(); err == nil {
		isTerminal = (fileInfo.Mode() & os.ModeCharDevice) != 0
	}

	logger := &AppLogger{
		debug:      debug,
		isTerminal: isTerminal,
	}

	if isTerminal {
		fmt.Print(banner)
	} else {
		fmt.Print(strings.ReplaceAll(strings.ReplaceAll(banner, "\033[36m", ""), "\033[0m", ""))
	}

	return logger
}

func (l *AppLogger) printLine(level, color, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("15:04:05")

	paddedLevel := fmt.Sprintf("%-5s", strings.ToUpper(level))
	if l.isTerminal {
		fmt.Fprintf(os.Stdout, "\033[90m[%s]\033[0m \033[%sm%s\033[0m %s\n", timestamp, color, paddedLevel, msg)
	} else {
		fmt.Fprintf(os.Stdout, "[%s] %s %s\n", timestamp, paddedLevel, msg)
	}
}

func (l *AppLogger) Infof(format string, args ...interface{}) {
	l.printLine("info", "32", format, args...)
}

func (l *AppLogger) Warnf(format string, args ...interface{}) {
	l.printLine("warn", "33", format, args...)
}

func (l *AppLogger) Errorf(format string, args ...interface{}) {
	l.printLine("error", "31", format, args...)
}

func (l *AppLogger) Fatalf(format string, args ...interface{}) {
	l.printLine("fatal", "41;37", format, args...)
	os.Exit(1)
}

func (l *AppLogger) Debugf(format string, args ...interface{}) {
	if !l.debug {
		return
	}
	l.printLine("debug", "36", format, args...)
}
