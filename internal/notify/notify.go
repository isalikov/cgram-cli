package notify

import (
	"github.com/gen2brain/beeep"
)

func NewMessage(sender, body string) {
	beeep.Notify("cgram — "+sender, body, "")
}

func ContactOnline(name string) {
	beeep.Notify("cgram", name+" is now online", "")
}

func ConnectionError(msg string) {
	beeep.Notify("cgram — Error", msg, "")
}
