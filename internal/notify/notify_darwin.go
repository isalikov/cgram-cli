package notify

import (
	"os"
	"os/exec"
)

func send(title, body string) {
	// Use terminal-notifier if available — click opens the terminal, not Script Editor
	if path, err := exec.LookPath("terminal-notifier"); err == nil {
		exec.Command(path, "-title", title, "-message", body, "-sender", bundleID()).Run()
		return
	}

	// Fallback: osascript with explicit activation of the terminal app
	script := `display notification "` + escapeAS(body) + `" with title "` + escapeAS(title) + `"`
	exec.Command("osascript", "-e", script).Run()
}

func bundleID() string {
	term := os.Getenv("TERM_PROGRAM")
	switch term {
	case "iTerm.app":
		return "com.googlecode.iterm2"
	case "WezTerm":
		return "com.github.wez.wezterm"
	case "Alacritty":
		return "org.alacritty"
	default:
		return "com.apple.Terminal"
	}
}

func escapeAS(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			out = append(out, '\\', '"')
		case '\\':
			out = append(out, '\\', '\\')
		default:
			out = append(out, s[i])
		}
	}
	return string(out)
}
