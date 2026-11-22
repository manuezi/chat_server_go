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

func main() {
	log.SetFlags(0)

	nick, input, ok := readNickname(os.Args)
	if !ok {
		return
	}

	conn, err := net.DialTimeout("tcp", protocol.SERVER_ADDRESS, protocol.DIAL_LIMIT)
	if err != nil {
		log.Fatalln("Erro ao conectar:", err)
	}
	defer conn.Close()

	conn.SetWriteDeadline(time.Now().Add(protocol.WRITE_LIMIT))
	if _, err := fmt.Fprintf(conn, "%s %s %s\n", protocol.START_KEYWORD, protocol.USER_KIND, nick); err != nil {
		log.Fatalln("Erro ao enviar apelido:", err)
	}

	lines := make(chan string)
	go readInput(input, lines)

	done := make(chan struct{})
	go readServer(conn, done)

	for {
		select {
		case line, open := <-lines:
			if !open {
				return
			}

			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			if line == protocol.EXIT_COMMAND {
				conn.SetWriteDeadline(time.Now().Add(protocol.WRITE_LIMIT))
				fmt.Fprintln(conn, protocol.EXIT_COMMAND)
				return
			}

			conn.SetWriteDeadline(time.Now().Add(protocol.WRITE_LIMIT))
			if _, err := fmt.Fprintln(conn, line); err != nil {
				log.Fatalln("Erro ao enviar comando:", err)
			}
		case <-done:
			return
		}
	}
}

func readNickname(args []string) (string, *bufio.Scanner, bool) {
	input := bufio.NewScanner(os.Stdin)

	if len(args) > 1 {
		nick := strings.TrimSpace(args[1])
		if nick == "" {
			log.Fatalln("Apelido não pode ser vazio")
		}
		return nick, input, true
	}

	fmt.Print("Apelido: ")
	if !input.Scan() {
		return "", input, false
	}

	nick := strings.TrimSpace(input.Text())
	if nick == "" {
		log.Fatalln("Apelido não pode ser vazio")
	}
	return nick, input, true
}

func readInput(input *bufio.Scanner, lines chan<- string) {
	defer close(lines)
	for input.Scan() {
		lines <- strings.TrimRight(input.Text(), "\r")
	}
}

func readServer(conn net.Conn, done chan<- struct{}) {
	defer close(done)

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, protocol.INITIAL_LINE_SIZE), protocol.MAX_LINE_SIZE)

	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}

	if err := scanner.Err(); err != nil && !errors.Is(err, net.ErrClosed) {
		fmt.Fprintln(os.Stderr, "Erro de leitura:", err)
	}
}
