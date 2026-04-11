# Backend Scaffold

Estrutura inicial do backend do `DisparaGO`.

## Pastas principais

- `cmd/api`: ponto de entrada da aplicacao HTTP
- `internal/app`: bootstrap da aplicacao
- `internal/config`: carregamento de configuracao
- `internal/httpserver`: inicializacao do Fiber, handlers e rotas
- `internal/platform`: integracoes de infraestrutura
- `internal/evolutiongo`: client base do provider
- `internal/worker`: worker inicial de disparo
- `internal/domain`: entidades centrais do dominio
- `migrations`: scripts SQL iniciais

## Estado atual

O scaffold inclui:

- healthcheck
- conexao com PostgreSQL
- conexao com Redis
- client base do EvolutionGO
- rotas iniciais de campanhas
- worker stub
- migracao inicial de campanhas e mensagens

## Proximo passo

- instalar Go no ambiente
- executar `go mod tidy`
- adicionar repositorios e servicos de aplicacao
- persistir campanhas no banco
- publicar jobs no Redis
- integrar envio real com o EvolutionGO

## Docker local

Arquivos adicionados para ambiente local:

- `Dockerfile`
- `docker-compose.yml`
- `.dockerignore`

Servicos previstos no compose:

- `app`: API do DisparaGO
- `postgres`: banco local
- `redis`: fila local

Observacao importante:

- no `docker-compose.yml`, o `EVOLUTIONGO_BASE_URL` esta configurado como `http://host.docker.internal:8081`
- isso assume que o EvolutionGO esta rodando fora do compose, no host Windows
- se futuramente o EvolutionGO entrar no mesmo compose, basta trocar a URL para o nome do servico correspondente
