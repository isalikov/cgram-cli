package notify

import "os/exec"

func send(title, body string) {
	// PowerShell toast notification
	script := `[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] > $null; ` +
		`$template = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent([Windows.UI.Notifications.ToastTemplateType]::ToastText02); ` +
		`$text = $template.GetElementsByTagName('text'); ` +
		`$text.Item(0).AppendChild($template.CreateTextNode('` + title + `')) > $null; ` +
		`$text.Item(1).AppendChild($template.CreateTextNode('` + body + `')) > $null; ` +
		`$toast = [Windows.UI.Notifications.ToastNotification]::new($template); ` +
		`[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('cgram').Show($toast)`
	exec.Command("powershell", "-Command", script).Run()
}
