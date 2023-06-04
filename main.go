package main

import (
	"context"
	"fmt"
	"time"

	"github.com/blainemoser/shellwrapper/shellwrapper"
)

func main() {
	sh := shellwrapper.NewShell()
	sh.FirstInstruction("would you like to install the programme?").
		IfUserInputs("run", "go", "now").
		Default("run").
		ThenBranch("please try something", func() {
			sh.IfUserInputs("hello", "there").ThenBranch("really though...", func() {
				sh.IfUserInputs("yes", "no").Default("no").ThenQuit("thanks for installing!")
			})
		}).
		IfUserInputs("quak").
		ThenRun(func(ctx context.Context, cancel context.CancelFunc) error {
			// When using timeouts the function is responsible for
			// dealing with context deadlines from the shell
			time.Sleep(time.Second * 4)
			for {
				select {
				case <-ctx.Done():
					return fmt.Errorf("no bueno")
				default:
					sh.Quit()
					return nil
				}
			}
		}).
		WithTimeout(3000).
		WithLoadingMessage("waiting for you to go away...").
		SetGreeting(
			"The Best WIZARD shell",
			"...Not to be confused with the Official Wizard Shell",
			"But still solid",
			"version 1.0.0",
		).
		SetBufferSize(20).
		Start()
}
