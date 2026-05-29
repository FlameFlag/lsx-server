package main

func isControlKeyword(name string) bool {
	switch name {
	case "if", "for", "while", "switch", "return", "sizeof":
		return true
	default:
		return false
	}
}
