# shellwrapper

### Use this library to create shell programmes ###

This library intends to provide an intuitive API for creating simple shell programmes.
I created it to build installation and configuration wizards for certain services whose setup could not be automated entirely and required some human input.

### Installation ###

Run ```go get github.com/blainemoser/shellwrapper``` to import the library.

### Approach ###

The flow of the shell programmes created using this library is governed by pre-defined user inputs. 

For example, the following code:

```
package main

import (
    "github.com/blainemoser/shellwrapper"
)

func main() {
    sh := shellwrapper.NewShell()
    sh.
        SetGreeting("Example Shell").
        FirstInstruction("ask the user to input something").
        IfUserInputs("yes", "y", "Y", "YES").
        ThenQuit("user selected 'yes'").
        IfUserInputs("no", "n", "N", "NO").
        ThenQuit("user selected 'no'")
}
```

...produces the following shell programme:

```
	_____________

	Example Shell
	_____________

> ask the user to input something [options: yes, no]
yes
> user selected 'yes'
> exiting...
```

### Branching ###

The flow of your shell programme is controlled by its branching rules, which can be set up using the function `ThenBranch` For example, the following code:

```
package main

import (
	"github.com/blainemoser/shellwrapper"
)

func main() {
	sh := shellwrapper.NewShell()
	sh.
		SetGreeting("Example Shell").
		FirstInstruction("would you like to install AWESOME PROGRAMME?").
		IfUserInputs("yes", "y", "Y", "YES").
		ThenBranch("ok, let's get started... Please select your version", func() {
			sh.
				IfUserInputs("1").ThenDisplay(func() string {
				return "installing version 1..."
			}).
				IfUserInputs("2").ThenDisplay(func() string {
				return "installing version 2..."
			}).
				Default("1")
		}).
		IfUserInputs("no", "n", "N", "NO").
		ThenQuit("OK!")
	// start the programme
	sh.Start()
}
```

...produces the following shell programme:

```
	_____________

	Example Shell
	_____________

> would you like to install AWESOME PROGRAMME? [options: yes, no]
yes
> ok, let's get started... Please select your version [options: 1, 2] (default '1')

1
installing version 1...
> exiting...
```

Note that when default is set for a branch (using the `Default` function), the user selects the default option by hitting return.

### Running Functions ###

In order to run functions in your shell programme, use the function`ThenRun`. For example, the following code:

```
package main

import (
	"context"
	"time"

	"github.com/blainemoser/shellwrapper"
)

func main() {
	sh := shellwrapper.NewShell()
	sh.
		SetGreeting("Example Shell").
		FirstInstruction("would you like to install AWESOME PROGRAMME?").
		IfUserInputs("yes", "y", "Y", "YES").
		ThenBranch("ok, let's get started... Please select your version", func() {
			sh.
				IfUserInputs("1").
				ThenRun(func(ctx context.Context, cf context.CancelFunc) error {
					time.Sleep(time.Second * 2)
					return nil
				}, "installing version 1...", 10*1000).
				ThenDisplay(func() string {
					return "> installed version 1."
				}).
				IfUserInputs("2").
				ThenRun(func(ctx context.Context, cf context.CancelFunc) error {
					time.Sleep(time.Second * 2)
					return nil
				}, "installing version 2...", 10*1000).
				ThenDisplay(func() string {
					return "> installed version 2."
				}).
				Default("1")
		}).
		IfUserInputs("no", "n", "N", "NO").
		ThenQuit("OK!")
	// start the programme
	sh.Start()
}
```
...produces the following shell programme:

```
	_____________

	Example Shell
	_____________

> would you like to install AWESOME PROGRAMME? [options: yes, no]
yes
> ok, let's get started... Please select your version [options: 1, 2] (default '1')

1
> installing version 1... /
```
... 
```
> installing version 1...  ...done
> installed version 1.
> exiting...
```
Note that the second `string` parameter for the function `ThenRun` is the loading message that is displayed when the function is executing (in the above example this is "installing version 1"). Loading messages will be displayed with a spinning effect next to them.

The third parameter is the timeout of the function in milliseconds. If the execution time of the function exceeds the specified timeout, an error will be displayed.

#### More on function timeouts ####

One should note that the callback passed to `ThenRun` must accept the parameters `ctx context.Context` and `context.CancelFunc`. This gives the callback the ability to handle its own termination on timeout (if this is required). In other words, the function is therefore able to stop its own go routing. 
This can be achieved by watching the context; for example:

```
// ...do some important stuff...
for {
	select {
	case <-ctx.Done():
		return fmt.Errorf("timed out before completion")
	default:
		sh.Display("Programme Installed", false)
		return nil
				}
}
```
Your callback may call the `context.CancelFunc` (although it should ideally just return an `error`); however it will be called on timeout by the caller.

**Note** that errors returned by your callback will be displayed to the user: therefore, if an error contains sensitive information it is the callback's responsibility to sanitise the error (or return nil), and/or implement its own error logging to ensure that sensitive errors don't pass silently but are not displayed to the user.

### Using GoTos ###

For some shell programmes, branches can be visited from different flows. The functions `Branch` and `GoTo` allow you to create branches once and visit them in different contexts. 

For example, the following code:

```
package main

import (
	"context"

	"github.com/blainemoser/shellwrapper"
)

func main() {
	sh := shellwrapper.NewShell()
	sh.SetGreeting("GOTO example", "this will demonstrate pre-defined branching")
	sh.Branch("programme_one", func() {
		sh.IfUserInputs("1").
			ThenRun(func(ctx context.Context, cf context.CancelFunc) error {
				// ... do something else
				return nil
			}, "installing programme one (version 1)...", 5*1000).
			GoTo("programme_two", "Programme One (version 1) has been installed. Should we install programme two?")
		sh.IfUserInputs("2").
			ThenRun(func(ctx context.Context, cf context.CancelFunc) error {
				// ... do something else
				return nil
			}, "installing programme one (version 2)...", 5*1000).
			GoTo("programme_two", "Programme One (version 2) has been installed. Should we install programme two?")
	})
	sh.Branch("programme_two", func() {
		sh.IfUserInputs("yes", "y", "Y", "YES").
			ThenRun(func(ctx context.Context, cf context.CancelFunc) error {
				// ... do something
				return nil
			}, "installing programme two...", 5*1000)
		sh.IfUserInputs("no", "NO", "n", "N").
			ThenQuit("done")
	})
	sh.FirstInstruction("do you need to install Programme One?").
		IfUserInputs("yes", "y", "YES", "Y").
		GoTo("programme_one", "What version of Programme One should we install?").
		IfUserInputs("no", "n", "NO", "n").
		GoTo("programme_two", "OK, we won't install Programme One. Should we install Programme Two")
	sh.Start()
}

```

...produces the following shell programme:

```
	___________________________________________

	GOTO example
	this will demonstrate pre-defined branching
	___________________________________________

> do you need to install Programme One? [options: yes, no]
no
> OK, we won't install Programme One. Should we install Programme Two [options: yes, no]
yes
> installing programme two...  ...done
> exiting...
```

You should note that the function `GoTo` accepts two `string` parameters: the first is the pre-defined Branch to go to, the second is the message that will be displayed to the user once there.

### Asking the User for information ###

The shell programme can ask the user for information using the `Ask`, `AskForInt` and/or `AskForFloat` functions. For example, the following code:

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
	sh.
		SetGreeting("Example Shell").
		FirstInstruction("would you like to install AWESOME PROGRAMME?").
		IfUserInputs("yes", "y", "Y", "YES").
		AskForInt("What version?", "version").
		ThenRun(func(ctx context.Context, cf context.CancelFunc) error {
			_, ok := sh.GetIntValue("version")
			if !ok {
				return fmt.Errorf("user-provided version not found")
			}
			time.Sleep(time.Second * 2)
			return nil
		}, "installing selected version...", 5*1000).
		ThenDisplay(func() string {
			version, _ := sh.GetIntValue("version")
			return fmt.Sprintf("> installed version '%d'", version)
		}).
		IfUserInputs("no", "n", "N", "NO").
		ThenQuit("OK!")
	// start the programme
	sh.Start()
}
```

... produces the following shell programme:

```
	_____________

	Example Shell
	_____________

> would you like to install AWESOME PROGRAMME? [options: yes, no]
yes
> What version?
one
> Please enter an integer e.g. 34
> What version?
1
> installing selected version...  ...done
> installed version '1'
> exiting...
```

One should note that the functions `Ask`, `AskForInt` and `AskForFloat` accept two `string` parameters: the former is the text for the question itself, the latter is the name to assign to the user's answer so that the answer can be retrieved later.

The function `Ask` accepts any `string` input from the user. The function `AskForInt` accepts input that can be formatted as an integer. `AskForFloat` accepts input that can be formatted as a float. 

#### Retrieving a user's answers ####

To retrieve answers given to questions created using the `Ask` function, use `GetValue`. `GetValue` accepts one `string` parameter: use the string you chose for `storeAs` parameter for `Ask` to retrieve the user's answers. `GetValue` returns an empty `string` if no answer is found for `storeAs`. 

To retrieve answers to `AskForInt`and `AskForFloat`, use `GetIntValue` and `GetFloatValue` respectively. Note that the latter two functions have two returns; the first being the result and the second being a `boolean` to indiecate whether an answer was actually found (because the user could legitimately have inputted zero values to `AskForInt` `and `AskForFloat`). 

Please note that user's answers can only be retrieved once they have been inputted; therefore they must be retrieved in event callbacks that occur thereafter. For example:

```
sh := shellwrapper.NewShell()
sh.
	SetGreeting("Example Shell").
	Ask("how is the weather today?", "forecast").
	ThenDisplay(func() string {
		return "Looks like " + sh.GetValue("forecast") + " weather..."
	})
```

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
