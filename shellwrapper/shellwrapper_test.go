package shellwrapper

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
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

func TestFlowConditions(t *testing.T) {
	Testing = true
	sh := NewShell()
	var message string
	sh.SetGreeting("welcome to the test shell").FirstInstruction("run programme?").IfUserInputs("yes", "y", "Yes", "YES", "Y").Default("yes").ThenRun(func(ctx context.Context, cf context.CancelFunc) error {
		time.Sleep(time.Second * 2)
		message = "hello from shell test"
		sh.Display(message, false)
		return nil
	}).ThenQuit("thank you")
	go sh.Start()
	// bufferOutput()
	// copy the output in a separate goroutine so printing can't block indefinitely
	time.Sleep(time.Second * 2)
	// sh.UserInput <- string([]byte{27, 91, 65})
	sh.StdIn.(*bytes.Buffer).WriteString(string([]byte{27, 91, 65}) + "\n") // test special
	sh.waitForInput()
	time.Sleep(time.Second * 4)
	// fmt.Fprintln(sh.StdIn, "worse_commdng")
	// fmt.Print("this is what we've got... " + getOutput() + "ere")
	fmt.Println(message)
	fmt.Println(sh.flow.Executed)
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
