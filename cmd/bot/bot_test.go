package main

import "testing"

func TestParsePrivateMessage(t *testing.T) {
	tests := []struct {
		line   string
		sender string
		text   string
		ok     bool
	}{
		{
			line:   "@Zezinho disse em privado: Oi!",
			sender: "Zezinho",
			text:   "Oi!",
			ok:     true,
		},
		{
			line:   "@Zezinho disse em privado: Oi   tudo bem",
			sender: "Zezinho",
			text:   "Oi   tudo bem",
			ok:     true,
		},
		{
			line: "@Zezinho disse: Oi!",
		},
		{
			line: "@Zezinho disse em privado: ",
		},
		{
			line: "@ disse em privado: Oi",
		},
		{
			line: "Conexão Recusada: Apelido já existe",
		},
		{
			line: "",
		},
	}

	for _, test := range tests {
		sender, text, ok := parsePrivateMessage(test.line)
		if sender != test.sender || text != test.text || ok != test.ok {
			t.Errorf(
				"parsePrivateMessage(%q) = (%q, %q, %v), want (%q, %q, %v)",
				test.line, sender, text, ok, test.sender, test.text, test.ok,
			)
		}
	}
}

func TestReverse(t *testing.T) {
	tests := map[string]string{
		"abcdef": "fedcba",
		"Olá":    "álO",
		"a":      "a",
		"ab":     "ba",
		"":       "",
	}

	for input, want := range tests {
		if got := reverse(input); got != want {
			t.Errorf("reverse(%q) = %q, want %q", input, got, want)
		}
	}
}
