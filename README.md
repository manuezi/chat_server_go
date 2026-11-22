# EP03 - Chat server em Go

## Integrantes

- Emanuelly Gomes
- Gabriel Frasão

## Como executar

Execute os comandos a partir da raiz do repositório.

### Servidor

```bash
go run ./cmd/server
```

### Cliente

Com um apelido explícito:

```bash
go run ./cmd/client Alice
```

Sem argumento, o cliente pede o apelido antes de conectar:

```bash
go run ./cmd/client
# Apelido:
```

### Bot

```bash
# Apelido para o bot é opcional, o padrão já é Inversor
go run ./cmd/bot Inversor
```

## Organização

O projeto usa o módulo Go `ep3-paradigmas` e o pacote interno `internal/protocol` para compartilhar as constantes entre servidor, cliente e bot.

## Protocolo

### Handshake

O cliente e o bot se identificam com uma linha no formato `START`:

```text
START USER <apelido>
START BOT <apelido>
```

### Comandos do cliente

| Comando | Efeito |
|---|---|
| `\changenick <apelido>` | Troca o apelido do usuário. |
| `\msg <mensagem>` | Envia uma mensagem pública. |
| `\msg @<apelido> <mensagem>` | Envia uma mensagem privada para `@<apelido>`. |
| `\exit` | Encerra a sessão e fecha o cliente. |

## Testes

```bash
go test ./...
```

Com o detector de corrida:

```bash
go test -race ./...
```
