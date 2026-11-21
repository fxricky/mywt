package shell

import "os/exec"

func shellCommand(shellPath, dir string, args []string, env []string) (*exec.Cmd, error) {
	cmd := exec.Command(shellPath, args...)
	cmd.Dir = dir
	cmd.Env = env
	return cmd, nil
}
