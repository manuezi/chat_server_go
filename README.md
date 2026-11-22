# EP03 — Chat Server em Go

Implementação de servidor de chat concorrente em Go via TCP (`localhost:4444`), composto por servidor, cliente e bot reativo.

**Integrantes:** Emanuelly Gomes e Gabriel Frasão

## Estrutura do Projeto

- `cmd/server/server.go`: servidor TCP com hub centralizado via goroutines e channels.
- `cmd/client/client.go`: cliente de terminal interativo.
- `cmd/bot/bot.go`: bot reativo que inverte mensagens privadas recebidas.
- `internal/protocol/consts.go`: constantes e configurações compartilhadas do protocolo.

## Como Executar

Abra terminais separados na raiz do projeto e execute:

1. **Servidor:**
   ```bash
   go run ./cmd/server
