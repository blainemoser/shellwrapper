package main

import (
	"context"
	"fmt"
	"time"

	"github.com/blainemoser/shellwrapper/shellwrapper"
)

func main() {
	var message string
	sh := shellwrapper.NewShell()
	sh.SetGreeting("welcome to the test shell").
		ThenRun(func(ctx context.Context, cf context.CancelFunc) error {
			sh.Display("here in the land of the free...", true)
			return nil
		}, "loading once", 120).
		Ask("Are you animal, mineral or vegetable?", "animal_mineral_vegetable").
		AskForFloat("What is your weight in lbs?", "weight").
		FirstInstruction("run programme?").IfUserInputs("yes", "y", "Yes", "YES", "Y").
		Default("yes").
		ThenRun(func(ctx context.Context, cf context.CancelFunc) error {
			fmt.Println(sh.GetFloatValue("weight"))
			time.Sleep(time.Second * 3)
			message = fmt.Sprintf("you answered: %s", sh.GetValue("animal_mineral_vegetable"))
			return nil
		}, "running...", 100).
		ThenDisplay("> your weight is not good: %f", func() float64 {
			weight, _ := sh.GetFloatValue("weight")
			return weight
		}())
	sh.Start()
	fmt.Println(message)
	fmt.Println(sh.GetFloatValue("weight"))
	// sh := shellwrapper.NewShell()
	// sh.SetGreeting(
	// 	"Gandalf the WIZARD shell",
	// 	"version 1.0.0",
	// ).
	// 	SetBufferSize(120).
	// 	Ask("are you a human?", "human").
	// 	FirstInstruction("Would you like to install the programme?").
	// 	Branch("next", func() {
	// 		sh.
	// 			Ask("Are you sure you're not a robot?", "robot").
	// 			IfUserInputs("yes").
	// 			Default("yes").
	// 			ThenQuit("bye!").
	// 			IfUserInputs("no").
	// 			ThenBranch("why not?", func() {
	// 				sh.Display(sh.GetValue("human"), false)
	// 				sh.IfUserInputs("don't know", "dk").ThenQuit("sorry to hear that").
	// 					IfUserInputs("because", "b").ThenQuit(sh.GetValue("robot"))
	// 			})
	// 	}).
	// 	IfUserInputs("yes", "y", "YES", "ye", "Y", "YE").
	// 	Default("yes").
	// 	ThenRun(func(ctx context.Context, cancel context.CancelFunc) error {
	// 		// When using timeouts the function is responsible for
	// 		// dealing with context deadlines from the shell
	// 		time.Sleep(time.Second * 2)
	// 		for {
	// 			select {
	// 			case <-ctx.Done():
	// 				return fmt.Errorf("no bueno")
	// 			default:
	// 				sh.Display("Programme Installed", false)
	// 				return nil
	// 			}
	// 		}
	// 	}, "loading func...", 3500).
	// 	ThenQuit("awesome").
	// 	IfUserInputs("no", "NO", "n", "N").
	// 	GoTo("next", "ok, let's try something else").
	// 	Start()
}
