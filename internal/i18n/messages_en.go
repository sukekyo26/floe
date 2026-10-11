package i18n

func init() {
	register(English, messagesEN)
}

// messagesEN is the reference catalog: every other language has exactly its keys.
var messagesEN = map[string]string{}
