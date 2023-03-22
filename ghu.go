package ghu

import "os/exec"

func RefreshSSHAgent() error {
	cmd := exec.Command("bash", "-c", "eval \"$(ssh-agent -s)\"")
	return cmd.Run()
}
