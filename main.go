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
		Branch("next", func() {
			sh.IfUserInputs("yes").Default("yes").ThenQuit("bye!").IfUserInputs("no").ThenBranch("why not?", func() {
				sh.IfUserInputs("don't know", "dk").ThenQuit("sorry to hear that").
					IfUserInputs("because", "b").ThenQuit("fair enough.")
			})
		}).
		IfUserInputs("yes", "y", "YES", "ye", "Y", "YE").
		Default("yes").
		ThenRun(func(ctx context.Context, cancel context.CancelFunc) error {
			// When using timeouts the function is responsible for
			// dealing with context deadlines from the shell
			time.Sleep(time.Second * 2)
			for {
				select {
				case <-ctx.Done():
					return fmt.Errorf("no bueno")
				default:
					// sh.Quit()
					return nil
				}
			}
		}).
		WithTimeout(3500).
		WithLoadingMessage("waiting for some cows to come back").
		// ThenQuit("awesome").
		GoTo("next", "alright, it's been installed. Say yes!").
		IfUserInputs("no").
		GoTo("next", "fine, don't install it. See if I care. Say yes!").
		Start()
}
