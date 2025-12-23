package cli

import (
	"flag"

	"fortio.org/cli"
	"fortio.org/log"
	"grol.io/vm/asm"
	"grol.io/vm/cpu"
)

func Main() int {
	cli.CommandBeforeFlags = true
	cli.MinArgs = 1
	cli.MaxArgs = -1
	cli.ArgsHelp = "files...\nwhere command is one of: compile, run"
	cli.Main()
	log.Debugf("Command: %s, Args: %v", cli.Command, flag.Args())
	switch cli.Command {
	case "compile":
		return asm.Compile(flag.Args()...)
	case "run":
		return cpu.Run(flag.Args()...)
	default:
		return log.FErrf("invalid command %q", cli.Command)
	}
}
