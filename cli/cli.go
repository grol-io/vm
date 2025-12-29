package cli

import (
	"flag"
	"os"
	"runtime/pprof"

	"fortio.org/cli"
	"fortio.org/log"
	"grol.io/vm/asm"
	"grol.io/vm/cpu"
)

func memProfile(fname string) {
	f, err := os.Create(fname)
	if err != nil {
		log.Errf("can't open file for memory profile: %v", err)
		return
	}
	err = pprof.WriteHeapProfile(f)
	if err != nil {
		log.Errf("can't write memory profile: %v", err)
	}
	f.Close()
}

func Main() int {
	cli.CommandBeforeFlags = true
	cli.MinArgs = 0 // no arg to genh
	cli.MaxArgs = -1
	cli.ArgsHelp = "[<files>...]\nwhere command is one of: compile, genh, run"
	cpuProf := flag.String("profile-cpu", "", "write CPU profile to file")
	memProf := flag.String("profile-mem", "", "write memory profile to file")
	cli.Main()
	log.Debugf("Command: %s, Args: %v", cli.Command, flag.Args())
	if *cpuProf != "" {
		f, err := os.Create(*cpuProf)
		if err != nil {
			return log.FErrf("can't open file for cpu profile: %v", err)
		}
		err = pprof.StartCPUProfile(f)
		if err != nil {
			return log.FErrf("can't start cpu profile: %v", err)
		}
		log.Infof("Writing cpu profile to %s", *cpuProf)
		defer pprof.StopCPUProfile()
	}
	if *memProf != "" {
		defer memProfile(*memProf)
	}
	switch cli.Command {
	case "compile":
		return asm.Compile(flag.Args()...)
	case "run":
		return cpu.Run(flag.Args()...)
	case "genh":
		return asm.GenHeader()
	default:
		return log.FErrf("invalid command %q", cli.Command)
	}
}
