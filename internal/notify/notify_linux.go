package notify

import "os/exec"

func send(title, body string) {
	// notify-send is available on most Linux desktops
	if path, err := exec.LookPath("notify-send"); err == nil {
		exec.Command(path, title, body).Run()
	}
}
