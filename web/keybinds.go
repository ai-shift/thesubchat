package web

type KeybindsTable struct {
	SendMessage Keybind
	ToggleGraph Keybind
	NewChat     Keybind
}

type Keybind struct {
	Value string
}

var Keybinds = KeybindsTable{
	SendMessage: Keybind{Value: "keydown[ctrlKey&&key=='Enter']"},
	ToggleGraph: Keybind{Value: "keyup[key=='g']"},
	NewChat:     Keybind{Value: "keyup[key=='n']"},
}
