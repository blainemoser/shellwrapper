package shellwrapper

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

var (
	r    *os.File
	w    *os.File
	outC chan string
	old  *os.File
)

func TestMain(m *testing.M) {
	old = os.Stdout
	defer func() {
		os.Stdout = old
	}()
	code := m.Run()
	os.Exit(code)
}

func TestNew(t *testing.T) {
	sh := NewShell()
	if sh.flow == nil {
		t.Errorf("expected non-nil flow")
	}
	sh.SetGreeting("Hello Test!", "version 1.0.0").SetBufferSize(40)
	if sh.bufferSize != 40 {
		t.Errorf("expected shell buffer size to be 40, got %d", sh.bufferSize)
	}
	if len(sh.greeter) != 2 {
		t.Errorf("expected 2 lines in greeter, got %d", len(sh.greeter))
	}
	for _, line := range sh.greeter {
		if line != "Hello Test!" && line != "version 1.0.0" {
			t.Errorf("expected greeting messages to be either 'Hello Test!' or 'version 1.0.0'")
		}
	}
}

func TestBadCommand(t *testing.T) {
	Testing = true
	sh := NewShell()
	sh.FirstInstruction("run programme?").IfUserInputs("yes", "y", "Yes", "YES", "Y").Default("yes").ThenQuit("thank you")
	go sh.Start()
	bufferOutput()
	write(sh, "bad_command\n")
	getOutput()
	if err := checkShellBuffer(sh, []string{"unrecognised command 'bad_command'"}, false); err != nil {
		t.Error(err)
	}
}

func TestQuit(t *testing.T) {
	Testing = true
	sh := NewShell()
	sh.
		FirstInstruction("run programme?").
		IfUserInputs("yes", "y", "Yes", "YES", "Y").
		Default("yes").
		ThenQuit("thank you")
	go sh.Start()
	bufferOutput()
	write(sh, "exit\n")
	getOutput()
	if err := checkShellBuffer(sh, []string{"exiting..."}, false); err != nil {
		t.Error(err)
	}
}

func TestBranching(t *testing.T) {
	Testing = true
	sh := NewShell()
	sh.
		FirstInstruction("run programme?").
		SetBufferSize(1000).
		IfUserInputs("yes", "y", "Yes", "YES", "Y").
		Default("yes").
		ThenQuit("thank you").
		IfUserInputs("no", "n", "NO", "N").
		ThenBranch("would you like to do anything else?", func() {
			sh.
				IfUserInputs("yes").
				ThenQuit("that's all we can do though.").
				Default("yes").
				IfUserInputs("no").ThenQuit("OK")
		})
	go sh.Start()
	bufferOutput()
	write(sh, "n\n")
	write(sh, "yes\n")
	getOutput()
	if err := checkShellBuffer(sh, []string{"that's all we can do though.", "exiting..."}, false); err != nil {
		t.Error(err)
	}
}

func TestGotos(t *testing.T) {
	Testing = true
	sh := NewShell()
	sh.FirstInstruction("run programme?").Branch("branch_one", func() {
		sh.IfUserInputs("hello world!").ThenQuit("hello")
	}).Branch("branch_two", func() {
		sh.IfUserInputs("goodbye").ThenQuit("goodbye")
	}).IfUserInputs("one").Default("one").GoTo("branch_one", "you've entered branch one").
		IfUserInputs("branch_two").GoTo("branch_two", "you've entered branch two")
	go sh.Start()
	bufferOutput()
	write(sh, "one\n")
	write(sh, "hello world!\n")
	getOutput()
	if err := checkShellBuffer(sh, []string{"you've entered branch one", "hello"}, false); err != nil {
		t.Error(err)
	}
}

func TestNoBranch(t *testing.T) {
	Testing = true
	sh := NewShell()
	sh.FirstInstruction("run programme?").IfUserInputs("one").Default("one").GoTo("branch_one", "you've entered branch one").
		IfUserInputs("branch_two").GoTo("branch_two", "you've entered branch two")
	go sh.Start()
	bufferOutput()
	write(sh, "one\n")
	getOutput()
	if err := checkShellBuffer(sh, []string{"branch 'branch_one' not found"}, false); err != nil {
		t.Error(err)
	}
}

func TestFunc(t *testing.T) {
	Testing = true
	sh := NewShell()
	var message string
	sh.SetGreeting("welcome to the test shell").
		SetBufferSize(10000).
		FirstInstruction("run programme?").IfUserInputs("yes", "y", "Yes", "YES", "Y").Default("yes").ThenRun(func(ctx context.Context, cf context.CancelFunc) error {
		for {
			select {
			case <-ctx.Done():
				return fmt.Errorf("timeout (not expected)")
			default:
				message = "ran function"
				return nil
			}
		}
	}, "function loading", 10000).ThenQuit("thank you")
	go sh.Start()
	bufferOutput()
	write(sh, "\n")
	getOutput()
	if message != "ran function" {
		t.Errorf("expected message from function to be 'ran function', got '%s'", message)
	}
}

// One might see some weird terminal artifacts with this function
// This is when the goroutine for the function is run
func TestFuncTimeout(t *testing.T) {
	Testing = true
	sh := NewShell()
	var message string
	sh.SetGreeting("welcome to the test shell").
		FirstInstruction("run programme?").IfUserInputs("yes", "y", "Yes", "YES", "Y").Default("yes").
		ThenRun(func(ctx context.Context, cf context.CancelFunc) error {
			time.Sleep(time.Second * 2)
			for {
				select {
				case <-ctx.Done():
					return fmt.Errorf("timeout (expected)")
				default:
					message = "ran function"
					return nil
				}
			}
		}, "running...", 100).ThenQuit("thank you")
	go sh.Start()
	bufferOutput()
	write(sh, string([]byte{27, 91, 65})+"\n")
	time.Sleep(time.Second * 2) // should have timed out after three seconds
	getOutput()
	if len(message) > 0 {
		t.Errorf("expected message from function to be blank (expected timeout), got '%s'", message)
	}
	if err := checkShellBuffer(sh, []string{"timeout (expected)"}, false); err != nil {
		t.Error(err)
	}
}

func TestAsk(t *testing.T) {
	Testing = true
	sh := NewShell()
	var message string
	sh.SetGreeting("welcome to the test shell").
		FirstInstruction("run programme?").IfUserInputs("yes", "y", "Yes", "YES", "Y").
		Ask("Are you animal, mineral or vegetable?", "animal_mineral_vegetable").
		Default("yes").
		ThenRun(func(ctx context.Context, cf context.CancelFunc) error {
			message = fmt.Sprintf("you answered: %s", sh.GetValue("animal_mineral_vegetable"))
			return nil
		}, "running...", 100) // should quit when the propagation stops
	go sh.Start()
	bufferOutput()
	// Spam with some empty answers
	write(sh, string("\n"))
	write(sh, string("\n"))
	write(sh, string("\n"))
	write(sh, string("\n"))
	write(sh, string("\n"))
	write(sh, string("\n"))
	write(sh, string("robot\n"))
	getOutput()
	if len(message) < 1 || message != "you answered: robot" {
		t.Errorf("expected message from function to be 'you answered: robot', got '%s'", message)
	}
}

func TestAskInt(t *testing.T) {
	Testing = true
	sh := NewShell()
	sh.SetGreeting("welcome to the test shell").SetBufferSize(100).AskForInt("how many apples do you want?", "apples")
	go sh.Start()
	bufferOutput()
	write(sh, "I don't want any apples\n")
	write(sh, "17\n")
	getOutput()

	apples, _ := sh.GetIntValue("apples")
	if apples != 17 {
		t.Errorf("expected user input to be 17, got %d", apples)
	}

	if err := checkShellBuffer(sh, []string{"Please enter an integer e.g. 34"}, false); err != nil {
		t.Error(err)
	}
}

func TestAskFloat(t *testing.T) {
	Testing = true
	sh := NewShell()
	sh.SetGreeting("welcome to the test shell").SetBufferSize(100).AskForFloat("how tall are you (cm)?", "height")
	go sh.Start()
	bufferOutput()
	write(sh, "I don't know\n")
	write(sh, "180.2\n")
	getOutput()
	height, _ := sh.GetFloatValue("height")
	if height != 180.2 {
		t.Errorf("expected user input to be 180.2, got %f", height)
	}

	if err := checkShellBuffer(sh, []string{"Please enter a number e.g. 3.1415"}, false); err != nil {
		t.Error(err)
	}
}

func TestThenDisplay(t *testing.T) {
	Testing = true
	sh := NewShell()
	sh.SetGreeting("welcome to the test shell").SetBufferSize(100).ThenDisplay(func() string {
		return "> guess this programme was pretty pointless huh?"
	})
	go sh.Start()
	bufferOutput()
	time.Sleep(time.Second * 1)
	getOutput()
	if err := checkShellBuffer(sh, []string{"guess this programme was pretty pointless huh?"}, false); err != nil {
		t.Error(err)
	}
}

func TestSmallBuffer(t *testing.T) {
	Testing = true
	sh := NewShell()
	sh.SetGreeting("welcome to the test shell").SetBufferSize(1).ThenDisplay(func() string {
		return "> This should be the only buffer item, and it will soon be replaced..."
	}).ThenDisplay(func() string {
		return "> guess this programme was pretty pointless huh?"
	})
	go sh.Start()
	bufferOutput()
	time.Sleep(time.Second * 1)
	getOutput()
	e := sh.Buffer.Front()
	if e == nil {
		t.Fatalf("expected one buffer item, got none")
	}
	if e.Next() != nil {
		t.Errorf("expected only one buffer item")
	}
}

func TestInterrupt(t *testing.T) {
	Testing = true
	sh := NewShell()
	sh.SetGreeting("welcome to the test shell").
		FirstInstruction("run programme?").IfUserInputs("yes", "y", "Yes", "YES", "Y").
		Default("yes").ThenQuit("done with programme")
	go sh.Start()
	bufferOutput()
	time.Sleep(time.Second * 2) // should have timed out after three seconds
	sh.OsInterrupt <- os.Interrupt
	getOutput()
}

func bufferOutput() {
	r, w, _ = os.Pipe()
	os.Stdout = w
	outC = make(chan string)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outC <- buf.String()
	}()
}

func getOutput() string {
	w.Close()
	os.Stdout = old
	return <-outC
}

func checkShellBuffer(sh *Shell, messages []string, notIn bool) error {
	expects := make(map[string]struct{})
	for _, message := range messages {
		expects[message] = struct{}{}
	}
	for e := sh.Buffer.Front(); e != nil; e = e.Next() {
		v := e.Value
		bufferObject, ok := v.(*BufferObject)
		if !ok {
			continue
		}
		for message := range expects {
			if strings.Contains(bufferObject.Out, message) {
				delete(expects, message)
			}
		}
	}
	errs := make([]string, 0)
	if len(expects) > 0 {
		for message := range expects {
			errs = append(errs, message)
		}
		return fmt.Errorf("expected the following messages to be in the shell buffer (not found): '%s'", strings.Join(errs, "', '"))
	}
	return nil
}

func write(sh *Shell, input string) {
	sh.StdIn.(*bytes.Buffer).WriteString(input)
	sh.waitForInput()
	time.Sleep(time.Second * 1)
}
