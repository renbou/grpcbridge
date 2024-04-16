package bridgetest

import (
	"fmt"
	"strings"
	"testing"

	"github.com/renbou/grpcbridge/bridgelog"
)

func Logger(t *testing.T) bridgelog.Logger {
	return bridgelog.WrapPlainLogger(&testLogger{t})
}

type testLogger struct {
	t *testing.T
}

func (l *testLogger) Debug(msg string, args ...any) {
	l.t.Logf("[DEBUG] %s: %s", msg, l.fmt(args...))
}

func (l *testLogger) Info(msg string, args ...any) {
	l.t.Logf("[INFO] %s: %s", msg, l.fmt(args...))
}

func (l *testLogger) Warn(msg string, args ...any) {
	l.t.Logf("[WARN] %s: %s", msg, l.fmt(args...))
}

func (l *testLogger) Error(msg string, args ...any) {
	l.t.Logf("[ERROR] %s: %s", msg, l.fmt(args...))
}

func (l *testLogger) fmt(args ...any) string {
	var b strings.Builder

	for i := 0; i < len(args); {
		if i+1 < len(args) {
			b.WriteString(fmt.Sprint(args[i]))
			b.WriteString("=")
			b.WriteString(fmt.Sprint(args[i+1]))
			i += 2
		} else {
			b.WriteString(fmt.Sprint(args[i]))
			i++
		}

		b.WriteString(", ")
	}

	return b.String()
}
