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

	secrets "github.com/ijustfool/docker-secrets"
	log "github.com/sirupsen/logrus"
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
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	dockerSecrets, err := secrets.NewDockerSecrets("")
	if err != nil {
		log.WithError(err).Warn("error loading docker secerts")
	} else {
		cmd.Env = mapToEnvList(dockerSecrets.GetAll())
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

	var status syscall.WaitStatus
	syscall.Wait4(-1, &status, 0, nil)
}
