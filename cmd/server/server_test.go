package main

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestValidateNickname(t *testing.T) {
	valid := []string{
		"Alice",
		"GatoPreto",
		"usuário",
		strings.Repeat("a", MAX_NICK_LENGTH),
	}

	for _, nick := range valid {
		if err := validateNickname(nick); err != nil {
			t.Errorf("validateNickname(%q) returned unexpected error: %v", nick, err)
		}
	}

	invalid := []string{
		"",
		strings.Repeat("a", MAX_NICK_LENGTH+1),
		"Alice Smith",
		"a@b",
		"a\tb",
		"a\x00b",
	}

	for _, nick := range invalid {
		if err := validateNickname(nick); err == nil {
			t.Errorf("validateNickname(%q) returned nil, want an error", nick)
		}
	}
}

func TestParseHandshake(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		want    handshake
		wantErr bool
	}{
		{
			name: "user handshake",
			line: "START USER Alice",
			want: handshake{kind: USER_CLIENT, nick: "Alice"},
		},
		{
			name: "bot handshake",
			line: "START BOT Inversor",
			want: handshake{kind: BOT_CLIENT, nick: "Inversor"},
		},
		{
			name: "extra whitespace",
			line: "START   USER   Alice",
			want: handshake{kind: USER_CLIENT, nick: "Alice"},
		},
		{
			name:    "old user handshake",
			line:    "USER Alice",
			wantErr: true,
		},
		{
			name:    "old bot handshake",
			line:    "BOT Inversor",
			wantErr: true,
		},
		{
			name:    "wrong keyword",
			line:    "HELLO USER Alice",
			wantErr: true,
		},
		{
			name:    "missing nickname",
			line:    "START USER",
			wantErr: true,
		},
		{
			name:    "extra field",
			line:    "START USER Alice extra",
			wantErr: true,
		},
		{
			name:    "invalid kind",
			line:    "START ADMIN Alice",
			wantErr: true,
		},
		{
			name:    "invalid nickname",
			line:    "START USER Bad@Nick",
			wantErr: true,
		},
		{
			name:    "lowercase",
			line:    "start user Alice",
			wantErr: true,
		},
		{
			name:    "empty",
			line:    "",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseHandshake(test.line)
			if test.wantErr {
				if err == nil {
					t.Fatalf("parseHandshake(%q) returned nil error, want an error", test.line)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseHandshake(%q) returned unexpected error: %v", test.line, err)
			}
			if got != test.want {
				t.Fatalf("parseHandshake(%q) = %#v, want %#v", test.line, got, test.want)
			}
		})
	}
}

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name string
		line string
		want command
	}{
		{
			name: "public message",
			line: `\msg Olá pessoal`,
			want: command{kind: COMMAND_PUBLIC, text: "Olá pessoal"},
		},
		{
			name: "public message keeps internal spaces",
			line: `\msg Olá   pessoal`,
			want: command{kind: COMMAND_PUBLIC, text: "Olá   pessoal"},
		},
		{
			name: "private message",
			line: `\msg @Zezinho Oi!`,
			want: command{kind: COMMAND_PRIVATE, target: "Zezinho", text: "Oi!"},
		},
		{
			name: "private message keeps internal spaces",
			line: `\msg @Zezinho Oi   tudo bem`,
			want: command{kind: COMMAND_PRIVATE, target: "Zezinho", text: "Oi   tudo bem"},
		},
		{
			name: "change nickname",
			line: `\changenick GatoPreto`,
			want: command{kind: COMMAND_CHANGE_NICK, target: "GatoPreto"},
		},
		{
			name: "exit",
			line: `\exit`,
			want: command{kind: COMMAND_EXIT},
		},
		{
			name: "exit with arguments",
			line: `\exit now`,
			want: command{kind: COMMAND_INVALID, text: `\exit não aceita argumentos`},
		},
		{
			name: "change nickname missing argument",
			line: `\changenick`,
			want: command{kind: COMMAND_INVALID, text: "Informe o novo apelido"},
		},
		{
			name: "change nickname with extra arguments",
			line: `\changenick Gato Preto`,
			want: command{kind: COMMAND_INVALID, text: "Informe o novo apelido"},
		},
		{
			name: "message missing text",
			line: `\msg`,
			want: command{kind: COMMAND_INVALID, text: "Informe a mensagem a ser enviada"},
		},
		{
			name: "private message missing text",
			line: `\msg @Zezinho`,
			want: command{kind: COMMAND_INVALID, text: "Informe a mensagem a ser enviada"},
		},
		{
			name: "unknown command",
			line: `\foo`,
			want: command{kind: COMMAND_UNKNOWN},
		},
		{
			name: "plain text",
			line: `hello`,
			want: command{kind: COMMAND_UNKNOWN},
		},
		{
			name: "empty line",
			line: ``,
			want: command{kind: COMMAND_UNKNOWN},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := parseCommand(test.line)
			if got != test.want {
				t.Fatalf("parseCommand(%q) = %#v, want %#v", test.line, got, test.want)
			}
		})
	}
}

func TestHubJoinAndDuplicateNickname(t *testing.T) {
	h, console := newTestHub(t)

	alice := newTestClient(USER_CLIENT, "Alice")
	if err := h.join(alice); err != nil {
		t.Fatalf("join returned unexpected error: %v", err)
	}
	expectMessage(t, alice.out, "Usuário @Alice acabou de entrar")

	if !strings.Contains(console.String(), "Usuário @Alice acabou de entrar") {
		t.Fatalf("server console does not contain the entrance message: %q", console.String())
	}

	duplicate := newTestClient(USER_CLIENT, "Alice")
	if err := h.join(duplicate); err == nil || err.Error() != "Apelido já existe" {
		t.Fatalf("join duplicate error = %v, want Apelido já existe", err)
	}
	expectNoMessage(t, duplicate.out)
}

func TestHubPublicMessagesReachUsersOnly(t *testing.T) {
	h, _ := newTestHub(t)

	alice := newTestClient(USER_CLIENT, "Alice")
	bob := newTestClient(USER_CLIENT, "Bob")
	bot := newTestClient(BOT_CLIENT, "Inversor")

	for _, c := range []*client{alice, bob, bot} {
		if err := h.join(c); err != nil {
			t.Fatalf("join %s returned unexpected error: %v", c.nick, err)
		}
	}

	expectMessage(t, alice.out, "Usuário @Alice acabou de entrar")
	expectMessage(t, alice.out, "Usuário @Bob acabou de entrar")
	expectMessage(t, alice.out, "Bot @Inversor acabou de entrar")
	expectMessage(t, bob.out, "Usuário @Bob acabou de entrar")
	expectMessage(t, bob.out, "Bot @Inversor acabou de entrar")
	expectNoMessage(t, bot.out)

	h.public(publicMessage{sender: alice, text: "Olá pessoal"})
	expectMessage(t, alice.out, "@Alice disse: Olá pessoal")
	expectMessage(t, bob.out, "@Alice disse: Olá pessoal")

	h.private(privateMessage{sender: alice, target: "Inversor", text: "ping"})
	expectMessage(t, bot.out, "@Alice disse em privado: ping")
	expectNoMessage(t, bot.out)
}

func TestHubPrivateMessageToBot(t *testing.T) {
	h, console := newTestHub(t)

	alice := newTestClient(USER_CLIENT, "Alice")
	bot := newTestClient(BOT_CLIENT, "Inversor")

	if err := h.join(alice); err != nil {
		t.Fatalf("join alice returned unexpected error: %v", err)
	}
	if err := h.join(bot); err != nil {
		t.Fatalf("join bot returned unexpected error: %v", err)
	}

	expectMessage(t, alice.out, "Usuário @Alice acabou de entrar")
	expectMessage(t, alice.out, "Bot @Inversor acabou de entrar")
	expectNoMessage(t, bot.out)

	h.private(privateMessage{sender: alice, target: "Inversor", text: "abcdef"})
	want := "@Alice disse em privado: abcdef"
	expectMessage(t, bot.out, want)
	expectNoMessage(t, alice.out)

	if !strings.Contains(console.String(), want) {
		t.Fatalf("server console does not contain the private message: %q", console.String())
	}
}

func TestHubPrivateMessageToMissingUser(t *testing.T) {
	h, console := newTestHub(t)

	alice := newTestClient(USER_CLIENT, "Alice")
	if err := h.join(alice); err != nil {
		t.Fatalf("join returned unexpected error: %v", err)
	}
	expectMessage(t, alice.out, "Usuário @Alice acabou de entrar")
	before := console.String()

	h.private(privateMessage{sender: alice, target: "Ghost", text: "hi"})
	expectMessage(t, alice.out, "Usuário ou Bot @Ghost não encontrado")

	if console.String() != before {
		t.Fatalf("server console changed after an undeliverable private message: %q", console.String())
	}
}

func TestHubChangeNickname(t *testing.T) {
	h, console := newTestHub(t)

	alice := newTestClient(USER_CLIENT, "Alice")
	bob := newTestClient(USER_CLIENT, "Bob")

	if err := h.join(alice); err != nil {
		t.Fatalf("join alice returned unexpected error: %v", err)
	}
	if err := h.join(bob); err != nil {
		t.Fatalf("join bob returned unexpected error: %v", err)
	}

	expectMessage(t, alice.out, "Usuário @Alice acabou de entrar")
	expectMessage(t, alice.out, "Usuário @Bob acabou de entrar")
	expectMessage(t, bob.out, "Usuário @Bob acabou de entrar")

	if err := h.changeNick(alice, "Alicia"); err != nil {
		t.Fatalf("changeNick returned unexpected error: %v", err)
	}

	announcement := "Usuário @Alice agora é @Alicia"
	expectMessage(t, alice.out, announcement)
	expectMessage(t, bob.out, announcement)

	if !strings.Contains(console.String(), announcement) {
		t.Fatalf("server console does not contain the nickname change: %q", console.String())
	}

	if err := h.changeNick(alice, "Bob"); err == nil || err.Error() != "Apelido já existe" {
		t.Fatalf("changeNick to an existing nickname error = %v, want Apelido já existe", err)
	}

	if err := h.changeNick(alice, "Alicia"); err == nil || err.Error() != "Você já está usando esse apelido" {
		t.Fatalf("changeNick to the same nickname error = %v, want Você já está usando esse apelido", err)
	}

	h.public(publicMessage{sender: alice, text: "depois"})
	expectMessage(t, alice.out, "@Alicia disse: depois")
	expectMessage(t, bob.out, "@Alicia disse: depois")
}

func TestHubLeave(t *testing.T) {
	h, console := newTestHub(t)

	alice := newTestClient(USER_CLIENT, "Alice")
	bob := newTestClient(USER_CLIENT, "Bob")

	if err := h.join(alice); err != nil {
		t.Fatalf("join alice returned unexpected error: %v", err)
	}
	if err := h.join(bob); err != nil {
		t.Fatalf("join bob returned unexpected error: %v", err)
	}

	expectMessage(t, alice.out, "Usuário @Alice acabou de entrar")
	expectMessage(t, alice.out, "Usuário @Bob acabou de entrar")
	expectMessage(t, bob.out, "Usuário @Bob acabou de entrar")

	h.leave(alice)
	expectMessage(t, bob.out, "Usuário @Alice saiu")

	select {
	case <-alice.done:
	default:
		t.Fatal("alice done channel was not closed")
	}

	if !strings.Contains(console.String(), "Usuário @Alice saiu") {
		t.Fatalf("server console does not contain the exit message: %q", console.String())
	}

	bot := newTestClient(BOT_CLIENT, "Inversor")
	if err := h.join(bot); err != nil {
		t.Fatalf("join bot returned unexpected error: %v", err)
	}
	expectMessage(t, bob.out, "Bot @Inversor acabou de entrar")

	h.leave(bot)
	expectMessage(t, bob.out, "Bot @Inversor saiu")

	if !strings.Contains(console.String(), "Bot @Inversor saiu") {
		t.Fatalf("server console does not contain the bot exit message: %q", console.String())
	}
}

func newTestHub(t *testing.T) (*hub, *bytes.Buffer) {
	t.Helper()

	console := &bytes.Buffer{}
	h := newHub(console)
	go h.run()
	return h, console
}

func newTestClient(kind clientKind, nick string) *client {
	return &client{
		nick: nick,
		kind: kind,
		out:  make(chan string, CLIENT_BUFFER),
		done: make(chan struct{}),
	}
}

func expectMessage(t *testing.T, messages <-chan string, want string) {
	t.Helper()

	select {
	case got := <-messages:
		if got != want {
			t.Fatalf("got message %q, want %q", got, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for message %q", want)
	}
}

func expectNoMessage(t *testing.T, messages <-chan string) {
	t.Helper()

	select {
	case got := <-messages:
		t.Fatalf("got unexpected message %q", got)
	case <-time.After(50 * time.Millisecond):
	}
}
