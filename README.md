# shellwrapper

### Use this library to create shell programmes ###

### Example ###

```
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/blainemoser/shellwrapper"
)

func main() {
	sh := shellwrapper.NewShell()
	sh.SetGreeting(
		"Gandalf the WIZARD shell",
		"version 1.0.0",
	).
		SetBufferSize(120).
		FirstInstruction("Would you like to install the programme?").
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
					sh.Display("Programme Installed", false)
					return nil
				}
			}
		}).
		WithTimeout(3500).
		WithLoadingMessage("waiting for some cows to come back").
		ThenQuit("awesome").
		IfUserInputs("no", "NO", "n", "N").GoTo("next", "Alright, what else can we do for you then?").
		Start()
}
```
The Result:
```
	________________________

	Gandalf the WIZARD shell
	version 1.0.0
	________________________

> Would you like to install the programme? [options: yes, no] (default 'yes')
```
```
	________________________

	Gandalf the WIZARD shell
	version 1.0.0
	________________________

> Would you like to install the programme? [options: yes, no] (default 'yes')
yes
> waiting for some cows to come back -
> waiting for some cows to come back  ...done
> awesome
> exiting...
```
```
	________________________

	Gandalf the WIZARD shell
	version 1.0.0
	________________________

> Would you like to install the programme? [options: yes, no] (default 'yes')
no
> Alright, what else can we do for you then? [options: yes, no] (default 'yes')
no
> why not? [options: don't know, because]
because
> fair enough.
> exiting...
```
### TODO: ###
Write documentation on the API.
