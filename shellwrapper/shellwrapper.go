package shellwrapper

import (
	"bufio"
	"bytes"
	"container/list"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/blainemoser/shellwrapper/flow"
	"github.com/google/uuid"
	"github.com/gosuri/uilive"
)

const (
	JITTER_TIME = 140
)

var (
	disp             = []string{"/", "-", "\\", "|"}
	Testing   bool   = false
	errorUUID string = uuid.NewString()
)

type (
	Shell struct {
		StdIn               io.Reader
		Reader              *bufio.Reader
		OsInterrupt         chan os.Signal
		UserInput           chan string
		LastCaptured        chan string
		Buffer              *list.List
		Cancel              chan struct{}
		bufferSize          int
		flow                *flow.Flow
		writer              *uilive.Writer
		wait                chan struct{}
		shellOutChan        chan bool
		welcomeMsg          string
		greeter             []string
		lastSetInputs       []string
		firstInstructed     bool
		firstInstructionSet bool
	}

	BufferObject struct {
		In     string
		Out    string
		Time   time.Time
		hidden bool
	}

	Callback func() error
)

func NewShell() *Shell {
	stdIn, reader := getIO()
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	return &Shell{
		UserInput:    make(chan string),
		StdIn:        stdIn,
		Reader:       reader,
		OsInterrupt:  c,
		LastCaptured: make(chan string, 1),
		shellOutChan: make(chan bool, 1),
		Cancel:       make(chan struct{}, 1),
		Buffer:       list.New(),
		bufferSize:   10,
		flow:         flow.New(), // the root node, if you will
		writer:       getWriter(),
		wait:         make(chan struct{}),
	}
}

func (s *Shell) SetGreeting(greeting ...string) *Shell {
	s.greeter = greeting
	return s
}

func (s *Shell) SetBufferSize(bufferSize int) *Shell {
	s.bufferSize = bufferSize
	return s
}

func (s *Shell) IfUserInputs(input ...string) *Shell {
	s.lastSetInputs = input
	s.flow.Commands = append(s.flow.Commands, input...)
	s.insertFlow()
	return s
}

func (s *Shell) Default(def string) *Shell {
	s.flow.Default = def
	return s
}

// Set the first instruction that will appear in the shell
// For example 'what is your name?'
func (s *Shell) FirstInstruction(instruction string) *Shell {
	s.flow.Instruction = instruction
	return s
}

func (s *Shell) ThenRun(f flow.ExecFunc) *Shell {
	flow := s.findLastFlow()
	if flow == nil {
		return s
	}
	flow.Exec = f
	return s
}

func (s *Shell) ThenBranch(instruction string, f flow.FlowFunc) *Shell {
	flow := s.findLastFlow()
	if flow == nil {
		return s
	}
	flow.Instruction = instruction
	flow.Flow = f
	return s
}

func (s *Shell) ThenQuit(message string) *Shell {
	flow := s.findLastFlow()
	if flow == nil {
		return s
	}
	flow.Exec = func(ctx context.Context, cancel context.CancelFunc) error {
		s.Display("> " + message)
		s.quit()
		return nil
	}
	return s
}

func (s *Shell) WithTimeout(timeout uint) *Shell {
	flow := s.findLastFlow()
	if flow == nil {
		return s
	}
	flow.WaitTime = int(timeout)
	return s
}

func (s *Shell) WithLoadingMessage(message string) *Shell {
	flow := s.findLastFlow()
	if flow == nil {
		return s
	}
	flow.LoadingMessage = message
	return s
}

func (s *Shell) Display(message string) {
	fmt.Fprintln(s.writer, message)
}

func (s *Shell) Quit() {
	s.quit()
}

func (s *Shell) insertFlow() {
	flow := flow.New()
	for _, input := range s.lastSetInputs {
		if input == "exit" || input == "quit" || input == "back" {
			continue
		}
		s.flow.Flows[input] = flow
	}
}

func (s *Shell) findLastFlow() *flow.Flow {
	for _, input := range s.lastSetInputs {
		if input == "exit" || input == "quit" || input == "back" {
			continue
		}
		flow, ok := s.flow.Flows[input]
		if ok {
			return flow
		}
	}
	return nil
}

func (s *Shell) findFlow(command string) *flow.Flow {
	flow, ok := s.flow.Flows[command]
	if !ok {
		return nil
	}
	return flow
}

func (s *Shell) runFunc(waitFor int, message string, callback flow.ExecFunc) error {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		s.jitter(waitFor, message, cancel)
	}()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return callback(ctx, cancel)
		}
	}
}

func (s *Shell) jitter(waitFor int, message string, cancel context.CancelFunc) {
	defer s.writer.Flush()
	pos := 0
	load := time.NewTicker(time.Millisecond * JITTER_TIME)
	defer load.Stop()
	count := 0
	for range load.C {
		count += JITTER_TIME
		pos = s.loadScreen(pos, message)
		if count > waitFor {
			cancel()
			break
		}
	}
}

func (s *Shell) WithWelcomeMessage(message string) *Shell {
	s.welcomeMsg = message
	return s
}

func (s *Shell) greeting() {
	msg := s.greeter
	var longestLine int
	var line string
	for _, greet := range s.greeter {
		if longestLine < len(greet) {
			longestLine = len(greet)
		}
	}
	for i := 0; i < longestLine; i++ {
		line += "_"
	}
	fmt.Print("\t" + line + "\n\n\t" + strings.Join(msg, "\n\t") + "\n\t" + line + "\n\n")
}

func (s *Shell) Start() {
	if Testing {
		go s.running()
		return
	}
	go s.running()
	<-s.Cancel
	fmt.Println("> exiting...")
	close(s.shellOutChan)
	close(s.LastCaptured)
	close(s.Cancel)
	close(s.OsInterrupt)
	close(s.UserInput)
	s.writer.Stop()
}

func (s *Shell) quit() bool {
	s.Cancel <- struct{}{}
	return true
}

func (s *Shell) running() {
	s.greeting()
	go s.waitForInput()
	for {
		select {
		case <-s.Cancel:
			return
		case <-s.OsInterrupt:
			s.Cancel <- struct{}{}
			return
		case command := <-s.UserInput:
			s.capture(&command)
			if s.handleCommand(command) {
				return
			}
			go s.waitForInput()
		}
	}
}

func (s *Shell) handleCommand(command string) bool {
	switch command {
	case "":
		if len(s.flow.Default) < 1 {
			return false
		}
		command = s.flow.Default
	case errorUUID:
		return false
	case "quit", "exit":
		return s.quit()
	}
	if result := s.flowCommand(command); result != nil {
		return *result
	}
	return s.badCommand(command)
}

func (s *Shell) flowCommand(command string) *bool {
	flow := s.findFlow(command)
	if flow == nil {
		return nil
	}
	result := false
	s.flow = flow
	s.runFlowFunc()
	return &result
}

func (s *Shell) runFlowFunc() {
	if s.flow.Flow != nil {
		s.flow.Flow()
		return
	}
	if s.flow.Exec != nil {
		if err := s.runFunc(s.flow.WaitTime, s.flow.LoadingMessage, s.flow.Exec); err != nil {
			s.bufferError(err)
		}
	}
}

func (s *Shell) badCommand(command string) bool {
	s.waitForShellOutput(
		command,
		fmt.Sprintf(
			"> unrecognised command '%s'",
			strings.ReplaceAll(command, "\n", ""),
		),
		false,
	)
	return false
}

func (s *Shell) capture(command *string) {
	s.sanitize(command)
	if len(s.LastCaptured) > 0 {
		// Drain channel
		<-s.LastCaptured
	}
	s.LastCaptured <- *command
}

func (s *Shell) waitForShellOutput(input, msg string, hidden bool) {
	if Testing {
		s.shellOutput(input, msg, hidden)
		return
	}
	// if not in testing drain the channel
	<-s.shellOutput(input, msg, hidden)
}

func (s *Shell) shellOutput(input, msg string, hidden bool) <-chan bool {
	b := &BufferObject{
		In:     input,
		Out:    msg,
		Time:   time.Now(),
		hidden: hidden,
	}
	if s.Buffer.Len() >= s.bufferSize {
		e := s.Buffer.Back()
		s.Buffer.Remove(e)
	}
	s.Buffer.PushFront(b)
	fmt.Println(msg)
	s.shellOutChan <- true
	return s.shellOutChan
}

func (s *Shell) bufferError(err error) {
	switch err.Error() {
	case "EOF":
		return
	case "carriage_return":
		s.UserInput <- errorUUID
		return
	default:
		s.waitForShellOutput(errorUUID, fmt.Sprintf("> An error occured (%s), please try again", err.Error()), true)
	}
}

func (s *Shell) waitForInput() {
	s.instruct()
	userInput, err := s.Reader.ReadString('\n')
	if err != nil && userInput == "\n" {
		err = errors.New("carriage_return")
	}
	if err != nil {
		s.bufferError(err)
		return
	}
	s.special(&userInput)
	if userInput == "" {
		s.UserInput <- errorUUID
	}
	userInput = strings.TrimSuffix(userInput, "\n")
	s.UserInput <- userInput
}

func (s *Shell) instruct() {
	instruction := fmt.Sprintf("> %s [options: %s]", s.flow.Instruction, strings.Join(s.flow.Commands, ", "))
	if len(s.flow.Default) > 0 {
		instruction = fmt.Sprintf("%s (default '%s')", instruction, s.flow.Default)
	}
	fmt.Println(instruction)
	fmt.Print("> ")
}

func (s *Shell) loadScreen(pos int, message string) int {
	s.Display(fmt.Sprintf("> %s %s", message, disp[pos]))
	if pos == 3 {
		pos = 0
	} else {
		pos++
	}
	time.Sleep(time.Millisecond * 140)
	return pos
}

func getIO() (stdIn io.Reader, reader *bufio.Reader) {
	if Testing {
		var b []byte
		stdIn = bytes.NewReader(b)
	} else {
		stdIn = os.Stdin
	}
	reader = bufio.NewReader(stdIn)
	return
}

func getWriter() *uilive.Writer {
	writer := uilive.New()
	writer.Start()
	return writer
}

func (s *Shell) special(userInput *string) {
	bytes := []byte(*userInput)
	if len(bytes) >= 3 {
		if bytes[0] == 27 && bytes[1] == 91 {
			if bytes[2] == 65 {
				*userInput = s.lastCommand()
				return
			}
			*userInput = ""
		}
	}
}

func (s *Shell) lastCommand() string {
	for e := s.Buffer.Front(); e != nil; e = e.Next() {
		if e.Value != nil {
			if add, ok := e.Value.(*BufferObject); ok {
				if len(add.In) < 0 {
					continue
				}
				return add.In
			}
		}
	}
	return ""
}

func (s *Shell) sanitize(command *string) {
	*command = strings.ReplaceAll(*command, "\n", "")
	*command = strings.ReplaceAll(*command, "\t", "")
	*command = strings.Trim(*command, " ")
}
