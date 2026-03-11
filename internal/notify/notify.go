package notify

func NewMessage(sender, body string) {
	send("cgram — "+sender, body)
}

func ContactOnline(name string) {
	send("cgram", name+" is now online")
}

func ConnectionError(msg string) {
	send("cgram — Error", msg)
}
