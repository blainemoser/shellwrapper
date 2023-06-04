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
	BACK        = "back"
	QUIT        = "quit"
	EXIT        = "exit"
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
		branches            map[string]flow.FlowFunc
		writer              *uilive.Writer
		wait                chan struct{}
		shellOutChan        chan bool
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

	jitter struct {
		waitFor     int
		message     string
		cancel      context.CancelFunc
		ctx         context.Context
		jitterEnded chan struct{}
		pos         int
		count       int
	}
)

// NewShell returns a new pointer to Shell
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
		branches:     make(map[string]flow.FlowFunc),
		bufferSize:   10,
		flow:         flow.New(), // the root node, if you will
		writer:       getWriter(),
		wait:         make(chan struct{}),
	}
}

// SetGreeting Sets the greeting display for your shell programme
// each ...string passed is a new line in the greeting
func (s *Shell) SetGreeting(greeting ...string) *Shell {
	s.greeter = greeting
	return s
}

// SetBufferSize Sets the buffer size for the programme; a buffer-item is
// an input or output
func (s *Shell) SetBufferSize(bufferSize int) *Shell {
	s.bufferSize = bufferSize
	return s
}

// IfUserInputs Sets up the behaviour for your programme to respond to
// a user's input. input here is variadic so that you can
// set alternatives, for example yes, YES, y and Y
func (s *Shell) IfUserInputs(input ...string) *Shell {
	s.lastSetInputs = input
	s.flow.Commands = append(s.flow.Commands, input[0])
	s.insertFlow()
	return s
}

// Default Sets a default command to be run for when the user
// hits return without providing an input
func (s *Shell) Default(def string) *Shell {
	s.flow.Default = def
	return s
}

// FirstInstruction Sets the first instruction that will appear in the shell
// For example 'what is your name?'
func (s *Shell) FirstInstruction(instruction string) *Shell {
	s.flow.Instruction = instruction
	return s
}

// ThenRun runs the passed function f after a condition has been met
func (s *Shell) ThenRun(f flow.ExecFunc) *Shell {
	flow := s.findLastFlow()
	if flow == nil {
		return s
	}
	flow.Exec = f
	return s
}

// ThenBranch runs the callback function f after a condition has been met.
// The function f should contain further branching rules
func (s *Shell) ThenBranch(instruction string, f flow.FlowFunc) *Shell {
	flow := s.findLastFlow()
	if flow == nil {
		return s
	}
	flow.Instruction = instruction
	flow.Flow = f
	return s
}

// Branch lets the programmer create a branch in memory that can
// be visited at a later stage using GoTo
func (s *Shell) Branch(name string, f flow.FlowFunc) *Shell {
	s.branches[name] = f
	return s
}

// GoTo lets the programmer specify a "go to" on saved Branches,
// so that the Branch's branching rules will be applied
// after a condition has been met
func (s *Shell) GoTo(name string, instruction string) *Shell {
	branch, ok := s.branches[name]
	if !ok {
		panic(fmt.Sprintf("branch %s not found", name))
	}
	s.ThenBranch(instruction, branch)
	return s
}

// ThenQuit quits the programme after some condition has been met
func (s *Shell) ThenQuit(message string) *Shell {
	flow := s.findLastFlow()
	if flow == nil {
		return s
	}
	flow.Exec = func(ctx context.Context, cancel context.CancelFunc) error {
		s.Display("> "+message, false)
		s.quit()
		return nil
	}
	return s
}

// WithTimeout specifies the maximum execution time of a function
// passed into ThenRun. If this is exceeded, a timeout error will be
// generated
func (s *Shell) WithTimeout(timeout uint) *Shell {
	flow := s.findLastFlow()
	if flow == nil {
		return s
	}
	flow.WaitTime = int(timeout)
	return s
}

// WithLoadingMessage specifies the message that will be displayed
// while a function passed into ThenRun is executing
func (s *Shell) WithLoadingMessage(message string) *Shell {
	flow := s.findLastFlow()
	if flow == nil {
		return s
	}
	flow.LoadingMessage = message
	return s
}

// Display displays a message; if overwrite is false, it is displayed
// as a new line
func (s *Shell) Display(message string, overwrite bool) {
	s.waitForShellOutput("", message, overwrite, false)
}

// Start starts the shell programme
func (s *Shell) Start() {
	go s.running()
	<-s.Cancel
	fmt.Println("> exiting...")
	s.writer.Stop()
}

// Quit quits the shell programme
func (s *Shell) Quit() {
	s.quit()
}

func (s *Shell) insertFlow() *flow.Flow {
	flow := flow.New()
	for _, input := range s.lastSetInputs {
		if s.reservedWord(input) {
			continue
		}
		s.flow.Flows[input] = flow
	}
	return flow
}

func (s *Shell) reservedWord(input string) bool {
	return input == EXIT || input == QUIT || input == BACK
}

func (s *Shell) findLastFlow() *flow.Flow {
	for _, input := range s.lastSetInputs {
		if s.reservedWord(input) {
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

func (s *Shell) newJitter(waitFor int, message string) *jitter {
	ctx, cancel := context.WithCancel(context.Background())
	jitterEnded := make(chan struct{}, 1)
	return &jitter{
		ctx:         ctx,
		cancel:      cancel,
		jitterEnded: jitterEnded,
		waitFor:     waitFor,
		message:     message,
		pos:         0,
		count:       0,
	}
}

func (s *Shell) runFunc(waitFor int, message string, callback flow.ExecFunc) error {
	jitter := s.newJitter(waitFor, message)
	go func() {
		s.jitter(jitter)
	}()
	for {
		select {
		case <-jitter.ctx.Done():
			err := jitter.ctx.Err()
			<-jitter.jitterEnded
			return err
		default:
			err := callback(jitter.ctx, jitter.cancel)
			jitter.cancel()
			<-jitter.jitterEnded
			return err
		}
	}
}

func (j *jitter) displayError(s *Shell) {
	s.Display(fmt.Sprintf("> %s %s", j.message, " ...error"), true)
}

func (j *jitter) displayDone(s *Shell) {
	s.Display(fmt.Sprintf("> %s %s", j.message, " ...done"), true)
}

func (s *Shell) jitter(j *jitter) {
	defer s.writer.Flush()
	load := time.NewTicker(time.Millisecond * JITTER_TIME)
	defer load.Stop()
	for {
		select {
		case <-load.C:
			j.count += JITTER_TIME
			j.pos = s.loadScreen(j.pos, j.message)
			if j.count > j.waitFor {
				j.cancel()
				j.displayError(s)
				j.jitterEnded <- struct{}{}
				return
			}
		case <-j.ctx.Done():
			j.displayDone(s)
			j.jitterEnded <- struct{}{}
			return
		}
	}
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
	case errorUUID:
		return false
	case QUIT, EXIT:
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
	if s.flow.Exec != nil && !s.flow.Executed {
		if err := s.runFunc(s.flow.WaitTime, s.flow.LoadingMessage, s.flow.Exec); err != nil {
			s.bufferError(err)
		}
		s.flow.Executed = true
	}
	if s.flow.Flow != nil {
		s.flow.Flow()
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

func (s *Shell) waitForShellOutput(input, msg string, overwrite, hidden bool) {
	if Testing {
		s.shellOutput(input, msg, overwrite, hidden)
		return
	}
	// if not in testing drain the channel
	<-s.shellOutput(input, msg, overwrite, hidden)
}

func (s *Shell) shellOutput(input, msg string, overwrite, hidden bool) <-chan bool {
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
	if overwrite {
		fmt.Fprintln(s.writer, msg)
	} else {
		fmt.Println(msg)
	}
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
		s.waitForShellOutput(errorUUID, fmt.Sprintf("> An error occured (%s), please try again", err.Error()), false, true)
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
	s.emptyUserInput(&userInput)
	s.UserInput <- userInput
}

func (s *Shell) emptyUserInput(userInput *string) {
	*userInput = strings.TrimSuffix(*userInput, "\n")
	if len(*userInput) < 1 && len(s.flow.Default) > 0 {
		*userInput = s.flow.Default
		s.waitForShellOutput(*userInput, *userInput, false, false)
	}
}

func (s *Shell) emptyFlow() bool {
	return len(s.flow.Commands) < 1
}

func (s *Shell) instruct() {
	if s.emptyFlow() {
		return
	}
	instruction := fmt.Sprintf("> %s [options: %s]", s.flow.Instruction, strings.Join(s.flow.Commands, ", "))
	if len(s.flow.Default) > 0 {
		instruction = fmt.Sprintf("%s (default '%s')", instruction, s.flow.Default)
	}
	s.waitForShellOutput("", instruction, false, false)
}

func (s *Shell) loadScreen(pos int, message string) int {
	s.Display(fmt.Sprintf("> %s %s", message, disp[pos]), true)
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
