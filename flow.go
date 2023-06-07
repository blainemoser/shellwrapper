package shellwrapper

import (
	"container/list"
	"context"
)

type (
	Flow struct {
		Quit           ExecFunc
		Instruction    string
		Default        string
		WaitTime       int
		LoadingMessage string
		Events         *list.List
		Flows          Flows
		BaseCommands   []string
	}

	ExecFunc  func(context.Context, context.CancelFunc) error
	FlowFunc  func()
	Flows     map[string]*Flow
	EventFunc func(*list.Element) *list.Element
)

func NewFlow() *Flow {
	return &Flow{
		Events:       list.New(),
		BaseCommands: make([]string, 0),
		Flows:        make(Flows),
		WaitTime:     10 * 1000,
	}
}

func (f *Flow) AddEvent(e EventFunc) {
	f.Events.PushBack(e)
}
