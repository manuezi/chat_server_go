package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"ep3-paradigmas/internal/protocol"
)

const (
	MAX_NICK_LENGTH = 32
	HANDSHAKE_LIMIT = 10 * time.Second
	CLIENT_BUFFER   = 64
)

type clientKind int

const (
	USER_CLIENT clientKind = iota
	BOT_CLIENT
)

func (k clientKind) label() string {
	if k == BOT_CLIENT {
		return "Bot"
	}
	return "Usuário"
}

func parseClientKind(value string) (clientKind, bool) {
	switch value {
	case protocol.USER_KIND:
		return USER_CLIENT, true
	case protocol.BOT_KIND:
		return BOT_CLIENT, true
	default:
		return 0, false
	}
}

type handshake struct {
	kind clientKind
	nick string
}

type client struct {
	nick      string
	kind      clientKind
	conn      net.Conn
	out       chan string
	done      chan struct{}
	closeOnce sync.Once
}

func (c *client) close() {
	c.closeOnce.Do(func() {
		close(c.done)
		if c.conn != nil {
			c.conn.Close()
		}
	})
}

func (c *client) writeLoop() {
	for {
		select {
		case message := <-c.out:
			if c.conn == nil {
				return
			}
			c.conn.SetWriteDeadline(time.Now().Add(protocol.WRITE_LIMIT))
			if _, err := fmt.Fprintln(c.conn, message); err != nil {
				c.close()
				return
			}
		case <-c.done:
			return
		}
	}
}

type joinRequest struct {
	client *client
	reply  chan error
}

type nickChangeRequest struct {
	client  *client
	newNick string
	reply   chan error
}

type publicMessage struct {
	sender *client
	text   string
}

type privateMessage struct {
	sender *client
	target string
	text   string
}

type hub struct {
	clients      map[string]*client
	joinCh       chan joinRequest
	leaveCh      chan *client
	publicCh     chan publicMessage
	privateCh    chan privateMessage
	changeNickCh chan nickChangeRequest
	console      io.Writer
}

func newHub(console io.Writer) *hub {
	return &hub{
		clients:      make(map[string]*client),
		joinCh:       make(chan joinRequest),
		leaveCh:      make(chan *client),
		publicCh:     make(chan publicMessage),
		privateCh:    make(chan privateMessage),
		changeNickCh: make(chan nickChangeRequest),
		console:      console,
	}
}

func (h *hub) run() {
	for {
		select {
		case request := <-h.joinCh:
			h.handleJoin(request)
		case c := <-h.leaveCh:
			h.handleLeave(c)
		case message := <-h.publicCh:
			h.handlePublic(message)
		case message := <-h.privateCh:
			h.handlePrivate(message)
		case request := <-h.changeNickCh:
			h.handleChangeNick(request)
		}
	}
}

func (h *hub) join(c *client) error {
	reply := make(chan error, 1)
	h.joinCh <- joinRequest{client: c, reply: reply}
	return <-reply
}

func (h *hub) leave(c *client) {
	h.leaveCh <- c
}

func (h *hub) public(message publicMessage) {
	h.publicCh <- message
}

func (h *hub) private(message privateMessage) {
	h.privateCh <- message
}

func (h *hub) changeNick(c *client, newNick string) error {
	reply := make(chan error, 1)
	h.changeNickCh <- nickChangeRequest{client: c, newNick: newNick, reply: reply}
	return <-reply
}

func (h *hub) handleJoin(request joinRequest) {
	if _, exists := h.clients[request.client.nick]; exists {
		request.reply <- errors.New("Apelido já existe")
		return
	}

	h.clients[request.client.nick] = request.client
	h.announce(fmt.Sprintf("%s @%s acabou de entrar", request.client.kind.label(), request.client.nick))
	request.reply <- nil
}

func (h *hub) handleLeave(c *client) {
	current, exists := h.clients[c.nick]
	if !exists || current != c {
		return
	}

	delete(h.clients, c.nick)
	c.close()
	h.announce(fmt.Sprintf("%s @%s saiu", c.kind.label(), c.nick))
}

func (h *hub) handlePublic(message publicMessage) {
	if h.clients[message.sender.nick] != message.sender {
		return
	}
	h.broadcast(fmt.Sprintf("@%s disse: %s", message.sender.nick, message.text), false)
}

func (h *hub) handlePrivate(message privateMessage) {
	if h.clients[message.sender.nick] != message.sender {
		return
	}

	line := fmt.Sprintf("@%s%s%s", message.sender.nick, protocol.PRIVATE_SEPARATOR, message.text)
	target, exists := h.clients[message.target]
	if !exists {
		h.sendError(message.sender, fmt.Sprintf("Usuário ou Bot @%s não encontrado", message.target))
		return
	}

	fmt.Fprintln(h.console, line)
	if !h.trySend(target, line) {
		h.sendError(message.sender, fmt.Sprintf("Destinatário @%s indisponível", message.target))
	}
}

func (h *hub) handleChangeNick(request nickChangeRequest) {
	if h.clients[request.client.nick] != request.client {
		request.reply <- errors.New("Cliente não conectado")
		return
	}

	if request.newNick == request.client.nick {
		request.reply <- errors.New("Você já está usando esse apelido")
		return
	}

	if _, exists := h.clients[request.newNick]; exists {
		request.reply <- errors.New("Apelido já existe")
		return
	}

	oldNick := request.client.nick
	delete(h.clients, oldNick)
	request.client.nick = request.newNick
	h.clients[request.newNick] = request.client
	h.announce(fmt.Sprintf("%s @%s agora é @%s", request.client.kind.label(), oldNick, request.newNick))
	request.reply <- nil
}

func (h *hub) announce(line string) {
	fmt.Fprintln(h.console, line)
	h.broadcast(line, false)
}

func (h *hub) broadcast(message string, includeBots bool) {
	for _, c := range h.clients {
		if !includeBots && c.kind == BOT_CLIENT {
			continue
		}
		h.trySend(c, message)
	}
}

func (h *hub) sendError(c *client, message string) {
	if c.kind == BOT_CLIENT {
		return
	}
	h.trySend(c, message)
}

func (h *hub) trySend(c *client, message string) bool {
	select {
	case <-c.done:
		return false
	default:
	}

	select {
	case c.out <- message:
		return true
	case <-c.done:
		return false
	default:
		c.close()
		return false
	}
}

type commandKind int

const (
	COMMAND_UNKNOWN commandKind = iota
	COMMAND_PUBLIC
	COMMAND_PRIVATE
	COMMAND_CHANGE_NICK
	COMMAND_EXIT
	COMMAND_INVALID
)

type command struct {
	kind   commandKind
	target string
	text   string
}

func parseCommand(line string) command {
	line = strings.TrimSpace(line)
	if line == "" {
		return command{kind: COMMAND_UNKNOWN}
	}

	name, args := line, ""
	if index := strings.IndexAny(line, " \t"); index >= 0 {
		name = line[:index]
		args = strings.TrimSpace(line[index+1:])
	}

	switch name {
	case protocol.EXIT_COMMAND:
		if args != "" {
			return command{kind: COMMAND_INVALID, text: protocol.EXIT_COMMAND + " não aceita argumentos"}
		}
		return command{kind: COMMAND_EXIT}

	case protocol.CHANGE_NICK_COMMAND:
		fields := strings.Fields(args)
		if len(fields) != 1 {
			return command{kind: COMMAND_INVALID, text: "Informe o novo apelido"}
		}
		return command{kind: COMMAND_CHANGE_NICK, target: fields[0]}

	case protocol.MSG_COMMAND:
		if args == "" {
			return command{kind: COMMAND_INVALID, text: "Informe a mensagem a ser enviada"}
		}

		if !strings.HasPrefix(args, "@") {
			return command{kind: COMMAND_PUBLIC, text: args}
		}

		rest := args[1:]
		index := strings.IndexAny(rest, " \t")
		if index < 0 {
			return command{kind: COMMAND_INVALID, text: "Informe a mensagem a ser enviada"}
		}

		target := rest[:index]
		text := strings.TrimSpace(rest[index+1:])
		if target == "" {
			return command{kind: COMMAND_INVALID, text: "Informe a mensagem a ser enviada"}
		}
		if text == "" {
			return command{kind: COMMAND_INVALID, text: "Informe a mensagem a ser enviada para @" + target}
		}
		return command{kind: COMMAND_PRIVATE, target: target, text: text}

	default:
		return command{kind: COMMAND_UNKNOWN}
	}
}

func validateNickname(nick string) error {
	if nick == "" || utf8.RuneCountInString(nick) > MAX_NICK_LENGTH {
		return errors.New("Apelido deve ter entre 1 e 32 caracteres")
	}

	for _, character := range nick {
		if unicode.IsSpace(character) || character == '@' || unicode.IsControl(character) {
			return errors.New("Apelido não pode conter espaços, @ ou caracteres de controle")
		}
	}

	return nil
}

func parseHandshake(line string) (handshake, error) {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) != 3 || fields[0] != protocol.START_KEYWORD {
		return handshake{}, fmt.Errorf("formato de conexão inválido; use %s %s ou %s %s", protocol.START_KEYWORD, protocol.USER_KIND, protocol.START_KEYWORD, protocol.BOT_KIND)
	}

	kind, ok := parseClientKind(fields[1])
	if !ok {
		return handshake{}, fmt.Errorf("%s deve ser seguido de %s ou %s", protocol.START_KEYWORD, protocol.USER_KIND, protocol.BOT_KIND)
	}

	nick := fields[2]
	if err := validateNickname(nick); err != nil {
		return handshake{}, err
	}

	return handshake{kind: kind, nick: nick}, nil
}

func handleConnection(h *hub, conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, protocol.INITIAL_LINE_SIZE), protocol.MAX_LINE_SIZE)

	if err := conn.SetReadDeadline(time.Now().Add(HANDSHAKE_LIMIT)); err != nil {
		return
	}
	if !scanner.Scan() {
		return
	}

	parsedHandshake, err := parseHandshake(scanner.Text())
	if err != nil {
		fmt.Fprintln(conn, protocol.CONNECTION_REFUSED_PREFIX, err)
		return
	}
	kind := parsedHandshake.kind
	nick := parsedHandshake.nick

	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		return
	}

	c := &client{
		nick: nick,
		kind: kind,
		conn: conn,
		out:  make(chan string, CLIENT_BUFFER),
		done: make(chan struct{}),
	}

	if err := h.join(c); err != nil {
		fmt.Fprintln(conn, protocol.CONNECTION_REFUSED_PREFIX, err)
		return
	}
	defer h.leave(c)

	go c.writeLoop()

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		parsed := parseCommand(line)

		switch parsed.kind {
		case COMMAND_PUBLIC:
			h.public(publicMessage{sender: c, text: parsed.text})
		case COMMAND_PRIVATE:
			h.private(privateMessage{sender: c, target: parsed.target, text: parsed.text})
		case COMMAND_CHANGE_NICK:
			if err := validateNickname(parsed.target); err != nil {
				h.sendError(c, "Comando inválido: "+err.Error())
				continue
			}
			if err := h.changeNick(c, parsed.target); err != nil {
				h.sendError(c, "Comando inválido: "+err.Error())
			}
		case COMMAND_EXIT:
			return
		case COMMAND_INVALID:
			h.sendError(c, "Comando inválido: "+parsed.text)
		case COMMAND_UNKNOWN:
			h.sendError(c, "Comando desconhecido")
		}
	}
}

func main() {
	log.SetFlags(0)

	listener, err := net.Listen("tcp", protocol.SERVER_ADDRESS)
	if err != nil {
		log.Fatalln("Erro ao iniciar servidor:", err)
	}
	defer listener.Close()

	fmt.Printf("Servidor ouvindo em %s\n", protocol.SERVER_ADDRESS)

	h := newHub(os.Stdout)
	go h.run()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			fmt.Fprintln(os.Stderr, "Erro ao aceitar conexão:", err)
			continue
		}
		go handleConnection(h, conn)
	}
}
