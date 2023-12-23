//go:build linux
// +build linux

package main

import (
	"errors"
	"fmt"
	"syscall"

	// Uncomment this block to pass the first stage!
	"os"
	"os/exec"
)

// Usage: your_docker.sh run <image> <command> <arg1> <arg2> ...

func main() {
	// You can use print statements as follows for debugging, they'll be visible when running tests.

	command := os.Args[3]
	args := os.Args[4:len(os.Args)]

	// create a jail dir
	JailDir, _ := createRootDir()
	// copy command to new root
	copyFunc(command, JailDir)
	// execute function in new root
	runCommandInSandBox(command, JailDir, args)
}

func runCommandInSandBox(command string, JailDir string, args []string) {
	chrootArgs := []string{JailDir, command}
	chrootArgs = append(chrootArgs, args...)
	cmd := exec.Command("chroot", chrootArgs...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID,
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		func() {
			var exitError *exec.ExitError
			errors.As(err, &exitError)
			if exitError.ExitCode() != 0 {
				os.Exit(exitError.ExitCode())
			}
		}()
	}
}

func copyFunc(command string, JailDir string) {
	mkdirP := exec.Command("mkdir", "-p", JailDir+"/usr/local/bin")
	mkdirP.Stdout = os.Stdout
	mkdirP.Stderr = os.Stderr
	pErr := mkdirP.Run()
	if pErr != nil {
		fmt.Println("mkdir -p failure", pErr)
	}
	copyCommand := exec.Command("cp", command, JailDir+"/usr/local/bin/")
	copyCommand.Stdin = os.Stdin
	copyCommand.Stderr = os.Stderr
	copyCommand.Stdout = os.Stdout
	err := copyCommand.Run()
	if err != nil {
		fmt.Println("Not able to copy command", command)
	}
}

func createRootDir() (string, bool) {
	JailDir := "jailDir"
	command := exec.Command("mkdir", "-p", JailDir)
	err := command.Run()
	if err != nil {
		fmt.Println("failed to create directory", err)
		return "", false
	}
	return JailDir, true
}
