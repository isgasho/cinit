package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/creack/pty"
	secrets "github.com/ijustfool/docker-secrets"
	isatty "github.com/mattn/go-isatty"
	log "github.com/sirupsen/logrus"
	"golang.org/x/term"
)

func envKey(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return strings.ToUpper(s)
}

func mapToEnvList(kv map[string]string) []string {
	var envList []string
	for k, v := range kv {
		envList = append(envList, fmt.Sprintf("%s=%s", envKey(k), v))
	}
	return envList
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s COMMAND [args...]\n", filepath.Base(os.Args[0]))
		os.Exit(1)
	}

	cmd := exec.Command(os.Args[1], os.Args[2:]...)

	dockerSecrets, err := secrets.NewDockerSecrets("")
	if err != nil {
		log.WithError(err).Warn("error loading docker secerts")
	} else {
		cmd.Env = mapToEnvList(dockerSecrets.GetAll())
	}

	if isatty.IsTerminal(os.Stdout.Fd()) {
		ptmx, err := pty.Start(cmd)
		if err != nil {
			log.WithError(err).Warn("error allocating pty")
		}
		// Make sure to close the pty at the end.
		defer func() { _ = ptmx.Close() }() // Best effort.

		// Handle pty size.
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGWINCH)
		go func() {
			for range ch {
				if err := pty.InheritSize(os.Stdin, ptmx); err != nil {
					log.Printf("error resizing pty: %s", err)
				}
			}
		}()
		ch <- syscall.SIGWINCH                        // Initial resize.
		defer func() { signal.Stop(ch); close(ch) }() // Cleanup signals when done.

		// Set stdin in raw mode.
		oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			panic(err)
		}
		defer func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }() // Best effort.

		// Copy stdin to the pty and the pty to stdout.
		// NOTE: The goroutine will keep reading until the next keystroke before returning.
		go func() { _, _ = io.Copy(ptmx, os.Stdin) }()
		_, _ = io.Copy(os.Stdout, ptmx)
	} else {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		stdin, err := cmd.StdinPipe()
		if err != nil {
			log.WithError(err).Fatal("error creating stdin pipe")
		}

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			log.WithError(err).Fatal("error creating stdout pipe")
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			log.WithError(err).Fatal("error creating stderr pipe")
		}

		err = cmd.Start()
		if err != nil {
			log.WithError(err).Fatal("error starting process")
		}

		child, err := syscall.Getpgid(cmd.Process.Pid)
		if err != nil {
			log.WithError(err).Fatal("error getting process group")
		}

		go func() {
			_, err := io.Copy(stdin, os.Stdin)
			if err != nil && err != io.EOF {
				log.WithError(err).Error("error reading stdin pipe")
			}
			cmd.Process.Kill()
		}()

		go io.Copy(os.Stdout, stdout)
		go io.Copy(os.Stderr, stderr)

		sig := make(chan os.Signal)
		signal.Notify(sig)
		signal.Ignore(syscall.SIGCHLD)

		// Handle zombies
		go func() {
			for {
				syscall.Kill(-(child), (<-sig).(syscall.Signal))
			}
		}()
	}

	var status syscall.WaitStatus
	syscall.Wait4(-1, &status, 0, nil)
}
