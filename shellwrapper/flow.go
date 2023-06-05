package shellwrapper

import "context"

type (
	Flow struct {
		Flows          Flows
		Exec           ExecFunc
		Quit           ExecFunc
		Flow           FlowFunc
		Commands       []string
		Instruction    string
		Default        string
		WaitTime       int
		LoadingMessage string
		Executed       bool
		QAs            QAs
	}

	FlowFunc func()
	ExecFunc func(context.Context, context.CancelFunc) error
	Flows    map[string]*Flow
	QA       struct {
		Question string
	}
	QAs map[string]*QA
)

func NewFlow() *Flow {
	return &Flow{
		Flows:    make(Flows),
		Commands: make([]string, 0),
		QAs:      make(QAs),
		WaitTime: 10 * 1000,
	}
}
