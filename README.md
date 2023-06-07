# shellwrapper

### Use this library to create shell programmes ###

This library intends to provide an intuitive API for creating simple shell programmes.
I created it to build installation and configuration wizards for certain services whose setup could not be automated entirely and required some human input.

### Installation ###

Run ```go get github.com/blainemoser/shellwrapper``` to import the library.

### Approach ###

The flow of shell programmes created using this library is governed by pre-defined user inputs, using the function:

#### `func (s *Shell) IfUserInputs(input ...string) *Shell` 

**Description**: IfUserInputs Sets up the behaviour for your programme to respond to a user's input. `input` here is variadic so that you can set alternatives, for example `yes`, `YES`, `y` and `Y`

- `input` (...string): The inputs that trigger the behaviour defined hereafter; note that only the first input will be displayed as an option

**Returns**:
- `_` *Shell (self)

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

The flow of your shell programme is controlled by its branching rules, which can be set up using the function: 

#### `func (s *Shell) ThenBranch(instruction string, f FlowFunc) *Shell`

**Description**: ThenBranch runs the callback function f after a condition has been met. The function f should contain further branching rules

- `instruction` (string): The instruction/prompt that will be displayed to the user when the branch is entered
- `f` (FlowFunc|func()): Callback containing the logic for the branch

**Returns**:
- `_` *Shell (self)

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

Note that when default is set for a branch using the function, the user selects the default option by hitting return:

#### `func (s *Shell) Default(def string) *Shell`

**Description**: Default Sets a default command to be run for when the user hits return without providing an input

- `def` (string): The default selection

**Returns**:
- `_` *Shell (self)

### Running Functions ###

In order to run functions in your shell programme, use the function: 

#### `func (s *Shell) ThenRun(f ExecFunc, loadingMessage string, timeout uint) *Shell`

**Description**: ThenRun runs the passed function f after a condition has been met

- `f` (shellwrapper.ExecFunc|func (context.Context, context.CancelFunc) error): The callback that will be executed when the conditions trigger `ExecFunc`

- `loadingMessage` (string): the loading message that is displayed when the function is executing (in the below example this is "installing version 1") Loading messages will be displayed with a spinning effect next to them.

- `timeout` (uint): The timeout limit of the callback `f` (in milliseconds). If the execution time of the callback `f` exceeds the specified timeout, an error will be displayed.

**Returns**:
- `_` *Shell (self)

For example, the following code:

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
					return "installed version 1."
				}).
				IfUserInputs("2").
				ThenRun(func(ctx context.Context, cf context.CancelFunc) error {
					time.Sleep(time.Second * 2)
					return nil
				}, "installing version 2...", 10*1000).
				ThenDisplay(func() string {
					return "installed version 2."
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

#### More on function timeouts ####

One should note that the callback `f` passed to `ThenRun` must accept the parameters `ctx context.Context` and `context.CancelFunc`. This gives the callback the ability to handle its own termination on timeout (if this is required). In other words, the function is therefore able to stop its own go routine. 

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
Your callback may call the `context.CancelFunc` (although it should ideally just return an `error`); however `context.CancelFunc` will be called on timeout by the caller of `f`.

**Note** that errors returned by your callback will be displayed to the user: therefore, if an error contains sensitive information it is the callback's responsibility to sanitise the error (or return nil), and/or implement its own error logging to ensure that sensitive errors don't pass silently but are not displayed to the user.

### Using GoTos ###

For some shell programmes, branches can be visited from different flows. This can be achieved using the following functions:

#### `func (s *Shell) Branch(name string, f FlowFunc) *Shell`

**Description**: Branch lets the programmer create a branch in memory that can be visited at a later stage using the function `GoTo`

- `name` (string): Then name of the branch

- `f` (shellwrapper.FlowFunc|func()): The callback that contains the branching rules

**Returns**:
`_` *Shell (self)

#### `func (s *Shell) GoTo(name string, instruction string) *Shell` 

**Description**: GoTo lets the programmer specify a "go to" on saved Branches, so that branching rules can be applied in multiple contexts after a condition has been met

- `name` (string): The name of the branch to go to (specified in the `name` parameter in `Branch`)

- `instruction` (string): The message/instruction that will be displayed to the user when the GoTo event is executed.

**Returns**:
`_` *Shell (self)

These functions allow you to create branches once and visit them in different contexts. 

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

### Asking the User for information ###

The shell programme can ask the user for information using the following functions:

#### `func (s *Shell) Ask(question, storeAs string) *Shell`

**Description**: Ask promps the user for any string

- `question` (string): The question that will be posed to the user

- `storeAs` (string): The index that the user's answer will be stored as for later retrieval 

**Returns**:
`_` *Shell (self)

#### `func (s *Shell) AskForInt(question, storeAs string) *Shell` 

**Description**: AskForInt promps the user for an integer value. If the user's input is unacceptable then they will be prompted again

- `question` (string): The question that will be posed to the user

- `storeAs` (string): The index that the user's answer will be stored as for later retrieval 

**Returns**:
`_` *Shell (self)

#### `func (s *Shell) AskForFloat(question, storeAs string) *Shell ` 

**Description**: AskForFloat promps the user for a float value. If the user's input is unacceptable then they will be prompted again

- `question` (string): The question that will be posed to the user

- `storeAs` (string): The index that the user's answer will be stored as for later retrieval 

**Returns**:
`_` *Shell (self)

For example, the following code:

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
			return fmt.Sprintf("installed version '%d'", version)
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

The function `Ask` accepts any `string` input from the user. The function `AskForInt` accepts input that can be formatted as an integer and `AskForFloat` accepts input that can be formatted as a float. 

#### Retrieving a user's answers ####

To retrieve answers to questions created using `Ask`, use: 

#### `func (s *Shell) GetValue(storedAs string) string`

**Description**: GetValue is used to retrieve strings inputted by the user in the function Ask

- `storedAs` (string): The index set in the `storeAs` parameter in `Ask`

**Returns**:
`_` string (will be empty if not found)

To retrieve answers to `AskForInt`and `AskForFloat`, use: 

#### `func (s *Shell) GetIntValue(storedAs string) (result int, found bool)`

**Description**: GetIntValue is used to retrieve strings inputted by the user in the function Ask

- `storedAs` (string): The index set in the `storeAs` parameter in `AskForInt`

**Returns** (result int, found bool):
- `result` int
- `found` bool (will be false if no value is found, since users might legitimately input zero values)

#### `func (s *Shell) GetFloatValue(storedAs string) (result float64, found bool) ` 

**Description**: GetFloatValue is used to retrieve floats inputted by the user in the function AskForFloat

- `storedAs` (string): The index set in the `storeAs` parameter in `AskForFloat`

**Returns**:
- `result` int
- `found` bool (will be false if no value is found, since users might legitimately input zero values)

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

### Displaying messages ###

Use this function to display messages to the console during runtime:

#### `func (s *Shell) ThenDisplay(display DisplayFunc) *Shell`

**Description**: ThenDisplay schedules a display event

- `display` (shellwrapper.DisplayFunc|func() string): callback function that returns the string to be displayed

**Returns**:
- `_` *Shell

### Exiting the programme

When there are no more events to propagate the shell programme will automatically terminate. 

To set a terminate event expressly then use this function:

#### `func (s *Shell) ThenQuit(message string) *Shell` 

**Description**: ThenQuit quits the programme after some condition has been met

- `message` (string): The message to be displayed prior to exiting

**Returns**:
`_` *Shell (self)


### Licence ###
MIT