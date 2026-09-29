# Estudo Diário

Um sistema web minimalista para criar o hábito de estudar 25 minutos por dia, focado em aprendizado contínuo através da pesquisa, leitura, escrita e avaliação automática de resumos.

## O Projeto

O objetivo do Estudo Diário é simples: incentivar o aprendizado de um tema novo todos os dias de maneira focada. A plataforma funciona com uma dinâmica direta:

1. Uma vez por dia, um assunto aleatório da Wikipédia é sorteado.
2. O relógio de 25 minutos começa a contar automaticamente.
3. O usuário deve pesquisar e escrever um resumo autoral de 100 a 2000 caracteres antes que o tempo acabe.
4. O texto é avaliado e recebe uma nota de 0 a 10 e um feedback qualitativo, listando pontos técnicos que possam ter faltado.
5. Os pontos são somados em um ranking global entre todos os estudantes.

## Stack Tecnológica

O projeto foi construído focando em performance, baixo acoplamento e dependências mínimas.

* **Backend**: Go (Golang) estruturado no modelo de pacotes internos (Standard Go Project Layout). Uso do servidor e roteador HTTP nativos da linguagem (`net/http`).
* **Frontend**: SPA construída com HTML, CSS e JS puros em um único arquivo, injetada no binário final. O estilo é utilitário, com suporte nativo a light/dark mode fornecido pelo sistema operacional (`color-scheme: light dark`).
* **Banco de Dados**: PostgreSQL, acessado nativamente via `database/sql` com o driver `pgx`.
* **Segurança e Regulação**: Rate Limiter em memória por IP (global e severo), hash de senhas feito com algoritmo `bcrypt` e gerenciamento de sessão via `JWT` validado em rotas estritas.

## Estrutura do Código

A base de código se concentra em manter uma separação clara de responsabilidades:

* `cmd/api`: Ponto de entrada (main) do servidor e configuração de dependências cruzadas (wiring/mux).
* `internal/`: Lógica de negócio e configurações protegidas do acesso externo (não importáveis por outros módulos).
  * `ai`: Integração e montagem estrita dos payloads JSON de prompt enviados para a avaliação via API.
  * `auth`: Controle de criptografia e middlewares que extraem e protegem contextos de requisição.
  * `handler`: Roteadores HTTP, processamento dos DTOs e devolução estruturada das respostas para o frontend.
  * `middleware`: Rate limiters criados via `x/time/rate` aplicando restrições granulares de acesso.
  * `model`: Tipos base (DTOs/Entities) que circulam globalmente pela aplicação.
  * `repository`: Camada de consultas e transações diretas em SQL puro ao banco de dados.
  * `study`: Motor de regras: controle temporal, estados (in_progress/expired/submitted) e consulta randômica da Wikipédia.
* `web`: Assets estáticos (frontend) que utilizam do recurso `embed.FS` para tornarem-se intrínsecos ao executável final no ato do build.

## Execução Local

### Pré-requisitos
* Go 1.22 ou superior
* PostgreSQL em funcionamento
* Chave de acesso da API OpenAI

### Instalação

1. Clone o repositório.
2. Na raiz do diretório do projeto, forneça um arquivo `.env` para a inicialização do ambiente:

```env
DATABASE_URL=postgres://usuario:senha@localhost:5432/estudodiario
JWT_SECRET=chave_muito_segura_de_no_minimo_32_caracteres_de_comprimento
OPENAI_API_KEY=sk-sua-chave
OPENAI_MODEL=gpt-4o-mini
```

3. Baixe as dependências:
```bash
go mod tidy
```

4. Realize a compilação e inicie a execução:
```bash
go run ./cmd/api
```

O serviço iniciará na porta padrão `8080`. Acesse `http://localhost:8080` pelo navegador.

## Regras de Conduta

Como o código fonte é de conhecimento público, confiamos estritamente na comunidade. O uso da plataforma trata de avaliar a própria capacidade mental para compreender e discorrer sobre um contexto desconhecido em um período espremido de tempo. 

Gerar o resumo por intermédio de Inteligência Artificial somente para computar os pontos no placar público ofende, fundamentalmente, o próprio progresso individual de aprendizado.

## Licença

Este projeto é distribuído sob a [Licença MIT](LICENSE). Sinta-se livre para usar, modificar e distribuir o código conforme os termos da licença.
