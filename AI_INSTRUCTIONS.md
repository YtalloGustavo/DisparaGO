# Guia para IAs (DisparaGO)

Este documento foi criado para fornecer contexto, regras arquiteturais e diretrizes de desenvolvimento para assistentes de Inteligência Artificial que forem trabalhar no projeto **DisparaGO**. Sempre leia este guia antes de iniciar tarefas de refatoração, criação ou diagnóstico neste repositório.

## 1. Visão Geral do Projeto
O **DisparaGO** é uma aplicação focada no envio de campanhas do WhatsApp em grande escala. Ele funciona como um sistema satélite, independente de sistemas comerciais, assegurando estabilidade, rastreabilidade avançada e controle sobre grandes volumes de disparos. 

Utiliza as seguintes tecnologias core:
- **Backend**: Go (Golang) + Fiber (framework HTTP).
- **Banco de Dados**: PostgreSQL (gerenciado através da pasta de `migrations/`).
- **Filas e Concorrência**: Redis.
- **Provider do WhatsApp**: EvolutionGO.
- **Frontend**: Aplicação SPA React utilizando Vite (Pasta `web/`).

## 2. Estrutura e Padrões do Backend
O backend não utiliza apenas padrões de "scripts soltos", mas simenta-se numa arquitetura em camadas focada em modularidade:

- `cmd/api`: Ponto de entrada ("entry-point") da API HTTP.
- `internal/app`: Bootstrap comum e injeção de dependências.
- `internal/config`: Parsing e variáveis de ambiente (EnvVars), a segurança deve pautar os segredos aqui não preenchidos no código.
- `internal/domain`: Entidades e interfaces de negócio centrais, evite carregar frameworks web aqui dentro.
- `internal/evolutiongo`: Clientes que se integram via HTTP estritamente com o provedor EvolutionGO.
- `internal/httpserver`: Servidor HTTP Fiber e definições de handlers/rotas. Os handlers não devem possuir regras de negócio pesadas; eles despacham para o *service*.
- `internal/platform`: Configurações e conexões de infraestrutura (PostgreSQL, Redis, Logger).
- `internal/repository`: Operações diretas com o banco de dados (SQL bruto, libs ou ORMs definidos na especificação).
- `internal/service`: Contém toda a regra de operações, processamento, retries e orquestração.
- `internal/worker`: Workers designados para consumir jobs (mensagens engatilhadas) do Redis.
- `migrations/`: Nunca edite migrações passadas que já rodaram no banco. Sempre adicione novos arquivos SQL incrementais `UP` / `DOWN`.

## 3. Estrutura e Padrões do Frontend
A pasta `web/` é separada por um `package.json` base utilizando React com JavaScript (`tipo module`) e o `Vite`.

- Não adicione bibliotecas pesadas de styling (como TailwindCSS) e tipagem profunda (TypeScript) sem perguntar primeiro ao usuário; na dúvida mantenha com Vanilla CSS ou padrão atual.
- Siga as regras arquiteturais e visuais solicitadas: O design deve parecer premium, moderno, focar em glassmorphism, micro-interações, cores ricas e ter "Dark Mode" onde cabível se as decisões base apontarem a isso (evitar apenas vermelho/azul padrão).

## 4. Regras e Lógicas Cruciais

### Concorrência e Rate Limiting Anti-Ban
Essa aplicação tem como premissa ser escalável. Em todo código feito nos workers ou handlers (`DisparaGO` backend), as IAs devem assegurar que **taxas de envio e delays aleatórios** sejam injetadas para evitar congestionamento e os bans do WhatsApp. O worker não emite nada ao EvolutionGO direto de forma assíncrona louca: respeita as travas do banco/redis.

### Manipulação do EvolutionGO
A integração é por consumo de HTTP + Callbacks/Webhooks.
Quando criamos rotas que recebem status (como mensagens `enviado`, `entregue`, `lido`), não processe lógicas duras no contexto síncrono do request de webhook se houver IO intensivo; despache em async ou responda `200 OK` agilmente.

### Tratamento de Falhas (Retries) 
Toda operação que envolva a rede remota (envio pra API externa, conexão redis) pode cair. Crie cenários em GO que mapeiem falhas transientes e joguem de volta em filas de retry, deixando falhas definitivas marcadas e atualizadas no banco de dados.

## 5. Diretrizes Comportamentais da IA
- **Entendimento e Verificação**: Sempre se localize no diretório correto. Os comandos Node/Npm devem ocorrer ativamente dentro da pasta `/web` ou de container frontend. Os comandos GO devem ser chamados do root folder do repositório onde se localiza o `go.mod`.
- **Containers**: Para infraestrutura local foi montado o `docker-compose.yml`. As URLs no código ou `.env.example` apontam via host network `host.docker.internal` para se comunicar externamente. Respeite essa premissa.
- **Evite Respostas e Mudanças Genéricas**: Sempre atenda às regras listadas, fornecendo código funcional, evitando placeholders onde se exija visual. Crie artefatos, quando solicitado, com foco nesses padrões.
