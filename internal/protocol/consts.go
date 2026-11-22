package protocol

import "time"

const (
	SERVER_ADDRESS = "localhost:4444"

	START_KEYWORD = "START"
	USER_KIND     = "USER"
	BOT_KIND      = "BOT"

	MSG_COMMAND         = `\msg`
	CHANGE_NICK_COMMAND = `\changenick`
	EXIT_COMMAND        = `\exit`

	PRIVATE_SEPARATOR = " disse em privado: "

	CONNECTION_REFUSED_PREFIX = "Conexão Recusada:"

	DIAL_LIMIT        = 5 * time.Second
	WRITE_LIMIT       = 5 * time.Second
	INITIAL_LINE_SIZE = 1024
	MAX_LINE_SIZE     = 64 * 1024
)
