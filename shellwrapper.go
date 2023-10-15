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
	"strconv"
	"strings"
	"time"

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
	Testing          = false
	disp             = []string{"/", "-", "\\", "|"}
	errorUUID string = uuid.NewString()
	exitUUID  string = uuid.NewString()
)

type (
	Shell struct {
		StdIn          io.Writer
		Reader         *bufio.Reader
		OsInterrupt    chan os.Signal
		UserInput      chan string
		QuestionInput  chan string
		LastCaptured   chan string
		command        string
		Buffer         *list.List
		cancel         chan struct{}
		quit           chan struct{}
		exited         bool
		bufferSize     int
		flow           *Flow
		branches       map[string]FlowFunc
		writer         *uilive.Writer
		wait           chan struct{}
		awaitingAnswer string
		shellOutChan   chan bool
		greeter        []string
		lastSetInputs  []string
		qas            map[string]string
		intQas         map[string]int
		floatQas       map[string]float64
	}

	BufferObject struct {
		In     string
		Out    string
		Time   time.Time
		hidden bool
	}

	DisplayFunc func() string

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
		cancel:       make(chan struct{}),
		quit:         make(chan struct{}),
		Buffer:       list.New(),
		branches:     make(map[string]FlowFunc),
		bufferSize:   10,
		flow:         NewFlow(), // the root node, if you will
		writer:       getWriter(),
		wait:         make(chan struct{}),
		qas:          make(map[string]string),
		intQas:       make(map[string]int),
		floatQas:     make(map[string]float64),
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
	if s.flow == nil {
		s.flow = NewFlow()
	}
	s.lastSetInputs = input
	if !s.addCommand(input...) {
		return s
	}
	s.flow.AddEvent(func(e *list.Element) *list.Element {
		if err := s.awaitAnyInput(s.handleCommand, ""); err != nil {
			<-s.exit()
		}
		s.runEvents()
		return e.Next()
	})
	return s
}

func (s *Shell) awaitAnyInput(f func(string) bool, message string) error {
	ok := false
	for !ok {
		go s.waitForInput()
		if len(message) > 0 {
			s.waitForShellOutput("", message, false, false)
		}
		select {
		case <-s.OsInterrupt:
			return errors.New("interrupt")
		case input := <-s.UserInput:
			ok = f(input)
		}
	}
	return nil
}

func (s *Shell) addCommand(input ...string) bool {
	flow := NewFlow()
	var commandAdded bool
	for _, command := range input {
		s.reservedWord(command)
		if !commandAdded {
			s.flow.BaseCommands = append(s.flow.BaseCommands, command)
			s.command = command
			commandAdded = true
		}
		s.setFlow(flow, command)
	}
	return commandAdded
}

func (s *Shell) nextEvent(e *list.Element) *list.Element {
	next := e.Next()
	if next == nil {
		return nil
	}
	if _, ok := next.Value.(EventFunc); ok {
		return next
	}
	return nil
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
	s.getFlow().Instruction = instruction
	return s
}

// ThenRun runs the passed function f after a condition has been met
func (s *Shell) ThenRun(f ExecFunc, loadingMessage string, timeout uint) *Shell {
	s.getFlow().AddEvent(func(e *list.Element) *list.Element {
		s.runExec(f, loadingMessage, timeout)
		return s.nextEvent(e)
	})
	return s
}

// ThenBranch runs the callback function f after a condition has been met.
// The function f should contain further branching rules
func (s *Shell) ThenBranch(instruction string, f FlowFunc) *Shell {
	s.getFlow().AddEvent(func(e *list.Element) *list.Element {
		s.command = ""
		s.getFlow().Instruction = instruction
		f()
		return s.nextEvent(e)
	})
	return s
}

// Branch lets the programmer create a branch in memory that can
// be visited at a later stage using the function GoTo
func (s *Shell) Branch(name string, f FlowFunc) *Shell {
	s.branches[name] = f
	return s
}

// ThenDisplay schedules a display event
func (s *Shell) ThenDisplay(display DisplayFunc) *Shell {
	s.getFlow().AddEvent(func(e *list.Element) *list.Element {
		s.waitForShellOutput("display_event", "> "+display(), false, false)
		return s.nextEvent(e)
	})
	return s
}

// GoTo lets the programmer specify a "go to" on saved Branches,
// so that branching rules can be applied in multiple contexts
// after a condition has been met
func (s *Shell) GoTo(name string, instruction string) *Shell {
	branch, ok := s.branches[name]
	if !ok {
		s.ThenQuit(fmt.Sprintf("branch '%s' not found", name))
	}
	s.getFlow().AddEvent(func(e *list.Element) *list.Element {
		s.command = ""
		s.getFlow().Instruction = instruction
		branch()
		return s.nextEvent(e)
	})
	return s
}

// ThenQuit quits the programme after some condition has been met
func (s *Shell) ThenQuit(message string) *Shell {
	s.getFlow().AddEvent(func(e *list.Element) *list.Element {
		s.Display("> "+message, false)
		s.exited = true
		<-s.exit()
		return nil
	})
	return s
}

// Display displays a message; if overwrite is false, it is displayed
// as a new line
func (s *Shell) Display(message string, overwrite bool) *Shell {
	s.waitForShellOutput("", message, overwrite, false)
	return s
}

// Ask promps the user for any string
func (s *Shell) Ask(question, storeAs string) *Shell {
	return s.ask(question, storeAs, s.handleAnswer)
}

// AskForInt promps the user for an integer value
// If the user's input is unacceptable then they will be prompted again
func (s *Shell) AskForInt(question, storeAs string) *Shell {
	return s.ask(question, storeAs, s.handleIntAnswer)
}

// AskForFloat promps the user for a float value
// If the user's input is unacceptable then they will be prompted again
func (s *Shell) AskForFloat(question, storeAs string) *Shell {
	return s.ask(question, storeAs, s.handleFloatAnswer)
}

// Start starts the shell programme
func (s *Shell) Start() {
	go s.running()
	<-s.cancel
	s.Display("> exiting...", false)
	s.writer.Stop()
}

// GetValue is used to retrieve strings inputted by the user
// in the function Ask
func (s *Shell) GetValue(storedAs string) string {
	return s.qas[storedAs]
}

// GetIntValue is used to retrieve integers inputted by the user
// in the function AskForInt
func (s *Shell) GetIntValue(storedAs string) (result int, found bool) {
	result, found = s.intQas[storedAs]
	return
}

// GetFloatValue is used to retrieve floats inputted by the user
// in the function AskForFloat
func (s *Shell) GetFloatValue(storedAs string) (result float64, found bool) {
	result, found = s.floatQas[storedAs]
	return
}

func (s *Shell) getFlow() *Flow {
	if s.flow == nil {
		s.flow = NewFlow()
		return s.flow
	}
	if len(s.command) < 1 {
		return s.flow
	}
	return s.flow.Flows[s.command]
}

func (s *Shell) setFlow(flow *Flow, command string) {
	if s.flow == nil {
		s.flow = NewFlow()
	}
	s.flow.Flows[command] = flow
}

func (s *Shell) reservedWord(input string) {
	if input == EXIT || input == QUIT || input == BACK {
		panic(fmt.Sprintf("%s is a reserved word; please use inputs other than: %s", input, strings.Join([]string{
			EXIT, BACK, QUIT,
		}, ", ")))
	}
}

func (s *Shell) ask(question, storeAs string, handler func(string) bool) *Shell {
	s.getFlow().AddEvent(func(e *list.Element) *list.Element {
		defer func() { s.awaitingAnswer = "" }()
		s.awaitingAnswer = storeAs
		if err := s.awaitAnyInput(handler, "> "+question); err != nil {
			<-s.exit()
		}
		return s.nextEvent(e)
	})
	return s
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

func (s *Shell) runFunc(waitFor int, message string, callback ExecFunc) error {
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

func (s *Shell) exit() <-chan struct{} {
	close(s.cancel)
	return s.quit
}

func (s *Shell) running() {
	s.greeting()
	s.runEvents()
}

func (s *Shell) runEvents() {
	e := s.flow.Events.Front()
	for {
		if e == nil {
			<-s.exit()
			return
		}
		v := e.Value
		if event, ok := v.(EventFunc); ok {
			e = event(e)
		} else {
			e = nil // propagation ended
		}
	}
}

func (s *Shell) handleCommand(command string) bool {
	switch command {
	case errorUUID:
		return false
	case QUIT, EXIT:
		<-s.exit()
		return false
	case exitUUID:
		return true
	}
	if result := s.flowCommand(command); !result {
		s.badCommand(command)
		return false
	}
	return true
}

func (s *Shell) flowCommand(command string) bool {
	flow, ok := s.flow.Flows[command]
	if !ok {
		return false
	}
	s.flow = flow
	return true
}

func (s *Shell) handleAnswer(command string) bool {
	if command == exitUUID {
		return true
	}
	if len(command) < 1 {
		return false
	}
	s.qas[s.awaitingAnswer] = command
	return true
}

func (s *Shell) handleIntAnswer(command string) bool {
	if !s.handleAnswer(command) {
		return false
	}
	// Try convert to int 64
	result, err := strconv.Atoi(command)
	if err != nil {
		s.waitForShellOutput("int_conversion", "> Please enter an integer e.g. 34", false, false)
		return false
	}
	s.intQas[s.awaitingAnswer] = result
	return true
}

func (s *Shell) handleFloatAnswer(command string) bool {
	if !s.handleAnswer(command) {
		return false
	}
	// Try convert to int 64
	result, err := strconv.ParseFloat(command, 64)
	if err != nil {
		s.waitForShellOutput("float_conversion", "> Please enter a number e.g. 3.1415", false, false)
		return false
	}
	s.floatQas[s.awaitingAnswer] = result
	return true
}

func (s *Shell) runExec(f ExecFunc, loadingMessage string, timeout uint) {
	if err := s.runFunc(int(timeout), loadingMessage, f); err != nil {
		s.bufferError(err)
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
	if len(s.LastCaptured) > 0 {
		// Drain channel
		<-s.LastCaptured
	}
	s.LastCaptured <- *command
}

func (s *Shell) waitForShellOutput(input, msg string, overwrite, hidden bool) {
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
		s.UserInput <- ""
		return
	default:
		s.waitForShellOutput(errorUUID, fmt.Sprintf("> An error occured (%s)", err.Error()), false, true)
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
	s.sanitize(&userInput)
	s.capture(&userInput)
	s.special(&userInput)
	s.emptyUserInput(&userInput)
	s.UserInput <- userInput
}

func (s *Shell) emptyUserInput(userInput *string) {
	*userInput = strings.TrimSuffix(*userInput, "\n")
	if len(s.awaitingAnswer) < 1 && len(*userInput) < 1 && len(s.flow.Default) > 0 {
		*userInput = s.flow.Default
		s.waitForShellOutput(*userInput, *userInput, false, false)
	}
}

func (s *Shell) emptyFlow() bool {
	return len(s.flow.Flows) < 1
}

func (s *Shell) instruct() {
	if s.emptyFlow() {
		return
	}
	if len(s.awaitingAnswer) > 0 {
		return
	}
	instruction := fmt.Sprintf("> %s [options: %s]", s.flow.Instruction, strings.Join(s.flow.BaseCommands, ", "))
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

func getIO() (StdIn io.Writer, reader *bufio.Reader) {
	if Testing {
		var b []byte
		in := bytes.NewBuffer(b)
		reader = bufio.NewReader(in)
		StdIn = in
		return
	}
	in := os.Stdin
	reader = bufio.NewReader(in)
	StdIn = in
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
				if len(add.In) < 1 {
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
