package shell

import "os/exec"

func shellCommand(shellPath, dir string, env []string) (*exec.Cmd, error) {
	cmd := exec.Command(shellPath, shellArgs(shellPath)...)
	cmd.Dir = dir
	cmd.Env = env
	return cmd, nil
}
