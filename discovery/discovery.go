package discovery

import "google.golang.org/protobuf/reflect/protoreflect"

type Watcher interface {
	UpdateState(*State)
	ReportError(error)
}

type Resolver interface {
	Close()
}

type State struct {
	Services []ServiceDesc
}

type ServiceDesc struct {
	Name protoreflect.FullName
}
