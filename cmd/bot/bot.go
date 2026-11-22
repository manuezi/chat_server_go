package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"ep3-paradigmas/internal/protocol"
)

const DEFAULT_NICKNAME = "Inversor"

func main() {
	log.SetFlags(0)

	nick := DEFAULT_NICKNAME
	if len(os.Args) > 1 {
		nick = strings.TrimSpace(os.Args[1])
	}

	conn, err := net.DialTimeout("tcp", protocol.SERVER_ADDRESS, protocol.DIAL_LIMIT)
	if err != nil {
		log.Fatalln("Erro ao conectar:", err)
	}
	defer conn.Close()

	conn.SetWriteDeadline(time.Now().Add(protocol.WRITE_LIMIT))
	if _, err := fmt.Fprintf(conn, "%s %s %s\n", protocol.START_KEYWORD, protocol.BOT_KIND, nick); err != nil {
		log.Fatalln("Erro ao enviar apelido:", err)
	}

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, protocol.INITIAL_LINE_SIZE), protocol.MAX_LINE_SIZE)

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")

		if strings.HasPrefix(line, protocol.CONNECTION_REFUSED_PREFIX) {
			log.Fatalln(line)
		}

		sender, text, ok := parsePrivateMessage(line)
		if !ok {
			continue
		}

		reversed := reverse(text)
		fmt.Printf("Mensagem de: @%s\n", sender)
		fmt.Printf("Recebi: %s\n", text)
		fmt.Printf("Resposta: %s\n", reversed)

		conn.SetWriteDeadline(time.Now().Add(protocol.WRITE_LIMIT))
		if _, err := fmt.Fprintf(conn, "%s @%s %s\n", protocol.MSG_COMMAND, sender, reversed); err != nil {
			log.Fatalln("Erro ao enviar resposta:", err)
		}
	}

	if err := scanner.Err(); err != nil && !errors.Is(err, net.ErrClosed) {
		fmt.Fprintln(os.Stderr, "Erro de leitura:", err)
	}
}

func parsePrivateMessage(line string) (sender, text string, ok bool) {
	if !strings.HasPrefix(line, "@") {
		return "", "", false
	}

	sender, text, ok = strings.Cut(line[1:], protocol.PRIVATE_SEPARATOR)
	if !ok || sender == "" || text == "" {
		return "", "", false
	}
	return sender, text, true
}

func reverse(text string) string {
	characters := []rune(text)
	for i, j := 0, len(characters)-1; i < j; i, j = i+1, j-1 {
		characters[i], characters[j] = characters[j], characters[i]
	}
	return string(characters)
}
