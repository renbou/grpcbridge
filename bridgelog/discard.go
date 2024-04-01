package bridgelog

var _ Logger = (*DiscardLogger)(nil)

var discardInstance = new(DiscardLogger)

func Discard() Logger {
	return discardInstance
}

type DiscardLogger struct{}

func (*DiscardLogger) Debug(string, ...any) {}

func (*DiscardLogger) Info(string, ...any) {}

func (*DiscardLogger) Warn(string, ...any) {}

func (*DiscardLogger) Error(string, ...any) {}

func (dl *DiscardLogger) WithComponent(string) Logger {
	return dl
}

func (dl *DiscardLogger) With(...any) Logger {
	return dl
}
