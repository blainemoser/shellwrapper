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
	}

	FlowFunc func()
	ExecFunc func(context.Context, context.CancelFunc) error
	Flows    map[string]*Flow
)

func NewFlow() *Flow {
	return &Flow{
		Flows:    make(Flows),
		Commands: make([]string, 0),
		WaitTime: 10 * 1000,
	}
}
