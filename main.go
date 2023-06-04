package main

import (
	"context"
	"fmt"
	"time"

	"github.com/blainemoser/shellwrapper/shellwrapper"
)

func main() {
	sh := shellwrapper.NewShell()
	sh.SetGreeting(
		"The Best WIZARD shell",
		"...Not to be confused with the Official Wizard Shell",
		"But still solid",
		"version 1.0.0",
	).
		SetBufferSize(20).
		FirstInstruction("would you like to install the programme?").
		IfUserInputs("yes", "y", "YES", "ye", "Y", "YE").
		ThenBranch("please try something", func() {
			sh.IfUserInputs("hello", "HELLO", "h").
				ThenBranch("really though...", func() {
					sh.IfUserInputs("yes", "no").
						Default("no").
						ThenQuit("thanks for installing hello!")
				}).
				IfUserInputs("goodbye", "GOODBYE", "gb").
				Default("hello").
				ThenQuit("thanks for installing goodbye!")
		}).
		IfUserInputs("no", "NO", "N", "n").
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
		Default("yes").
		Start()
}
