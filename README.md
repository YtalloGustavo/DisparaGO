# DisparaGO

Documentacao base do `DisparaGO`, uma aplicacao independente dedicada ao envio de campanhas WhatsApp em escala, com estabilidade, rastreabilidade e controle operacional.

## Visao Geral

O `DisparaGO` sera publicado em dominio proprio:

- `disparador.servidoron.com.br`

Responsabilidade principal:

- receber campanhas via API REST
- organizar o processamento em fila
- executar envios com controle de concorrencia por instancia WhatsApp
- acompanhar o ciclo de vida de cada mensagem
- permitir pausa, retomada e reprocessamento com seguranca

O `DisparaGO` nasce como um produto proprio, com API, painel, banco e fila dedicados ao disparo em massa. Ele pode ser utilizado de forma independente sempre que houver necessidade de campanhas WhatsApp com maior controle operacional.

## Motivacao

Ferramentas de disparo acopladas a sistemas transacionais costumam apresentar limitacoes que afetam diretamente a operacao:

- sobrecarregam ou quebram a aplicacao principal durante campanhas
- nao possui tracking de entrega por mensagem
- nao permite pausar ou retomar disparos em tempo real
- dificulta diagnostico de falhas e reenvios

Com o `DisparaGO`, o envio em massa passa a ocorrer em uma estrutura especializada, mais estavel e escalavel, separada de qualquer sistema operacional ou comercial da empresa.

## Objetivos do Produto

- oferecer uma aplicacao propria para disparo em massa
- garantir confiabilidade no envio de ate `1.000 mensagens/dia por cliente`
- disponibilizar rastreamento completo por mensagem
- permitir controle operacional de campanhas em tempo real
- reduzir risco de bloqueios com rate limiting e delays aleatorios
- preparar a arquitetura para escala horizontal futura

## Escopo da Primeira Entrega

### Funcionalidades previstas

- API REST para criacao e gerenciamento de campanhas
- worker pool com controle de concorrencia por instancia WhatsApp
- rate limiting configuravel
- delay aleatorio entre envios para estrategia anti-ban
- tracking por mensagem com os estados:
  - `pendente`
  - `enviado`
  - `entregue`
  - `lido`
  - `falhou`
- pausa e retomada de campanhas em tempo real
- retry automatico em caso de falha
- integracao com EvolutionGO
- dashboard simples para acompanhamento operacional

### Fora de escopo inicial

- construtor visual avancado de campanhas
- segmentacao inteligente por comportamento
- testes A/B
- multi-provider de WhatsApp na primeira versao
- analytics avancado com BI embarcado

## Arquitetura Proposta

### Stack tecnica

- Backend: `Go (Golang)` + `Fiber`
- Fila e coordenacao: `Redis`
- Banco relacional: `PostgreSQL`
- Provider WhatsApp: `EvolutionGO`
- Dashboard: interface web simples consumindo a API do produto

### Componentes principais

#### 1. API Gateway / HTTP API

Responsavel por:

- receber requisicoes de usuarios, operadores ou sistemas externos autorizados
- criar e atualizar campanhas
- listar campanhas, mensagens e metricas
- executar comandos de pausa, retomada e cancelamento
- expor webhooks para atualizacao de status enviados pelo EvolutionGO

#### 2. Orquestrador de campanhas

Responsavel por:

- transformar uma campanha em itens individuais de envio
- registrar mensagens com status inicial `pendente`
- publicar jobs no Redis
- respeitar configuracoes da campanha e da instancia

#### 3. Worker Pool

Responsavel por:

- consumir jobs da fila
- aplicar controle de concorrencia por instancia WhatsApp
- aplicar rate limit e delay aleatorio entre mensagens
- chamar o EvolutionGO
- registrar tentativas, sucesso e falhas

#### 4. Rastreamento e eventos

Responsavel por:

- receber callbacks/webhooks de status
- atualizar status da mensagem no banco
- registrar historico de transicoes
- consolidar metricas da campanha

#### 5. Dashboard operacional

Responsavel por:

- exibir campanhas em andamento e finalizadas
- mostrar totais por status
- permitir pause/resume
- apoiar atendimento e suporte com visibilidade em tempo real

## Fluxo Macro

1. Um operador ou sistema autorizado cria uma campanha via HTTP no `DisparaGO`.
2. A API valida os dados, registra a campanha e persiste os destinatarios.
3. O orquestrador gera mensagens individuais com status `pendente`.
4. Os jobs sao publicados no Redis.
5. O worker pool consome os jobs respeitando a concorrencia por instancia.
6. Cada envio passa pelo EvolutionGO.
7. O resultado inicial atualiza a mensagem como `enviado` ou `falhou`.
8. Webhooks posteriores atualizam a mensagem para `entregue` ou `lido`.
9. O dashboard e a API refletem o progresso em tempo real.

## Requisitos Funcionais

### Campanhas

O produto deve permitir:

- criar campanha
- iniciar campanha
- pausar campanha
- retomar campanha
- cancelar campanha
- consultar progresso consolidado
- consultar mensagens individuais e erros

### Mensagens

Cada mensagem deve possuir:

- identificador unico
- referencia da campanha
- referencia do cliente
- referencia da instancia WhatsApp
- destinatario
- payload da mensagem
- status atual
- quantidade de tentativas
- ultimo erro
- timestamps de criacao, envio, entrega, leitura e falha

### Tracking

Estados suportados na primeira versao:

- `pendente`
- `processando`
- `enviado`
- `entregue`
- `lido`
- `falhou`
- `pausado` (estado operacional da campanha, nao necessariamente da mensagem)

Observacao:

- `processando` e util internamente para evitar envio duplicado em cenarios de concorrencia ou retry.

## Requisitos Nao Funcionais

- alta estabilidade mesmo sob campanhas simultaneas
- idempotencia em criacao de campanhas e webhooks
- isolamento da aplicacao em relacao a outros sistemas da operacao
- observabilidade com logs estruturados e metricas
- arquitetura preparada para escala horizontal
- configuracoes por cliente e por instancia

## Modelo de Dados Sugerido

### Tabela `campaigns`

Campos sugeridos:

- `id`
- `client_id`
- `name`
- `status` (`draft`, `running`, `paused`, `finished`, `cancelled`, `failed`)
- `instance_id`
- `total_messages`
- `sent_count`
- `delivered_count`
- `read_count`
- `failed_count`
- `scheduled_at`
- `started_at`
- `finished_at`
- `created_at`
- `updated_at`

### Tabela `campaign_messages`

Campos sugeridos:

- `id`
- `campaign_id`
- `client_id`
- `instance_id`
- `recipient_phone`
- `message_content`
- `status`
- `provider_message_id`
- `attempt_count`
- `max_attempts`
- `last_error`
- `next_retry_at`
- `sent_at`
- `delivered_at`
- `read_at`
- `failed_at`
- `created_at`
- `updated_at`

### Tabela `campaign_message_events`

Campos sugeridos:

- `id`
- `campaign_message_id`
- `event_type`
- `payload`
- `created_at`

### Tabela `campaign_rate_limits`

Campos sugeridos:

- `id`
- `client_id`
- `instance_id`
- `messages_per_minute`
- `min_delay_ms`
- `max_delay_ms`
- `max_concurrency`
- `retry_limit`
- `retry_backoff_seconds`
- `created_at`
- `updated_at`

## API REST Proposta

Base URL:

- `https://disparador.servidoron.com.br/api/v1`

### Campanhas

#### `POST /campaigns`

Cria uma nova campanha.

Exemplo de payload:

```json
{
  "client_id": "cli_123",
  "instance_id": "wa_01",
  "name": "Campanha Abril",
  "message": "Ola! Esta e uma campanha de teste.",
  "contacts": [
    { "phone": "5511999999999" },
    { "phone": "5511888888888" }
  ],
  "rate_limit": {
    "messages_per_minute": 20,
    "min_delay_ms": 4000,
    "max_delay_ms": 9000,
    "max_concurrency": 1,
    "retry_limit": 3
  }
}
```

Resposta esperada:

```json
{
  "id": "camp_001",
  "status": "draft",
  "total_messages": 2
}
```

#### `POST /campaigns/:id/start`

Inicia o processamento da campanha.

#### `POST /campaigns/:id/pause`

Pausa o processamento da campanha em tempo real.

#### `POST /campaigns/:id/resume`

Retoma o processamento da campanha.

#### `POST /campaigns/:id/cancel`

Cancela a campanha e impede novos envios.

#### `GET /campaigns`

Lista campanhas com filtros por cliente, status e periodo.

#### `GET /campaigns/:id`

Retorna detalhes da campanha e seu resumo operacional.

#### `GET /campaigns/:id/messages`

Lista mensagens da campanha com filtros por status.

### Webhooks

#### `POST /webhooks/evolution`

Recebe eventos do EvolutionGO para atualizar o ciclo de vida das mensagens.

Eventos esperados:

- enviado
- entregue
- lido
- erro

## Regras de Processamento

### Concorrencia por instancia WhatsApp

Cada `instance_id` deve possuir um limite proprio de concorrencia.

Regra inicial sugerida:

- `1 worker ativo por instancia` por padrao
- expansao futura permitida para mais workers apenas em instancias com homologacao segura

Objetivo:

- reduzir risco de bloqueio
- evitar sobrecarga no provider
- manter previsibilidade na ordem dos disparos

### Rate limiting e anti-ban

Cada instancia deve respeitar configuracoes como:

- mensagens por minuto
- intervalo minimo entre mensagens
- intervalo maximo entre mensagens
- delay aleatorio entre envios

Exemplo de estrategia:

- antes de cada envio, aguardar um tempo aleatorio entre `min_delay_ms` e `max_delay_ms`
- impedir picos abruptos de throughput

### Retry automatico

Falhas transientes devem gerar nova tentativa automatica.

Sugestao de politica inicial:

- maximo de `3` tentativas por mensagem
- backoff progressivo
- retry apenas para erros recuperaveis
- erros permanentes devem finalizar a mensagem como `falhou`

Erros tipicamente recuperaveis:

- timeout
- indisponibilidade temporaria do EvolutionGO
- falha de rede

Erros tipicamente nao recuperaveis:

- numero invalido
- instancia desconectada por longo periodo
- payload rejeitado por validacao

## Integracao com Sistemas Externos

O `DisparaGO` pode ser consumido por outros sistemas quando houver necessidade de automacao, sincronizacao ou acionamento remoto de campanhas.

Responsabilidades dos sistemas externos:

- montar a requisicao de campanha quando necessario
- enviar dados via HTTP para a API do `DisparaGO`
- consultar status quando fizer sentido para o fluxo de origem
- decidir como apresentar progresso, falhas ou resultados em seus proprios contextos

Responsabilidades do `DisparaGO`:

- processar a campanha de ponta a ponta
- controlar fila, workers, retries e tracking
- concentrar a observabilidade operacional
- manter sua propria logica de disparo independente da aplicacao chamadora

Beneficio dessa separacao:

- o `DisparaGO` pode evoluir como produto proprio
- sistemas externos nao absorvem a carga do processamento de campanha
- o time ganha reuso, isolamento e rastreabilidade

## Integracao com EvolutionGO

O `DisparaGO` dependera do EvolutionGO para:

- envio de mensagens
- consulta ou recebimento de status
- identificacao da instancia remetente

Pontos importantes da integracao:

- mapear `provider_message_id` para correlacionar callbacks
- validar autenticidade dos webhooks recebidos
- tratar indisponibilidade do provider com retry
- registrar payload bruto dos eventos para auditoria
- gerenciar instancias WhatsApp com pareamento via QR Code
- receber eventos em tempo real via webhook e, se necessario, WebSocket

Pontos confirmados pela documentacao oficial do EvolutionGO:

- e uma API WhatsApp escrita em Go
- expoe API RESTful
- suporta webhooks e eventos em tempo real
- utiliza `whatsmeow` internamente como biblioteca de integracao com WhatsApp
- possui persistencia opcional com PostgreSQL

Decisao arquitetural:

- o `DisparaGO` nao vai integrar com `whatsmeow` diretamente na primeira versao
- o produto vai consumir o `EvolutionGO` como provider HTTP
- isso reduz complexidade de implementacao e acelera a entrega do MVP

Responsabilidade do provider:

- conexao com WhatsApp
- pareamento e manutencao de sessao
- envio das mensagens
- emissao de eventos tecnicos e operacionais

Responsabilidade do `DisparaGO` acima do provider:

- criacao e gerenciamento de campanhas
- fila, concorrencia e rate limiting
- pausa e retomada
- retries e consolidacao de estados
- dashboard e visibilidade operacional

## Dashboard Simples

Escopo inicial do dashboard:

- lista de campanhas
- status atual de cada campanha
- total de mensagens por estado
- percentual de progresso
- acao de pausar e retomar
- visualizacao de falhas recentes

Objetivo do dashboard:

- dar visibilidade operacional sem depender de acesso ao banco
- apoiar suporte e acompanhamento durante campanhas ativas

## Observabilidade

Recomendacoes iniciais:

- logs estruturados em JSON
- correlacao por `campaign_id` e `message_id`
- metricas de throughput, falha, retry e latencia
- health checks para API, Redis e PostgreSQL

Metricas uteis:

- campanhas ativas
- mensagens pendentes
- mensagens enviadas por minuto
- taxa de falha por instancia
- tempo medio ate envio
- tempo medio ate entrega

## Escalabilidade

Volume alvo inicial:

- ate `1.000 mensagens/dia por cliente`

A arquitetura deve permitir evolucao para:

- multiplos workers
- distribuicao horizontal da API
- particionamento por cliente ou instancia
- filas separadas por prioridade

## Seguranca

Itens minimos recomendados:

- autenticacao entre clientes da API e o `DisparaGO`
- chave ou token por ambiente
- validacao de origem dos webhooks
- auditoria de operacoes de pause/resume/cancel
- mascaramento de dados sensiveis em logs

## Roadmap Tecnico Sugerido

### Fase 1 - Fundacao

- estrutura inicial do projeto em Go + Fiber
- conexao com PostgreSQL e Redis
- modelos principais
- endpoints basicos de campanha

### Fase 2 - Processamento

- publicacao em fila
- worker pool
- controle de concorrencia por instancia
- rate limiting e delay aleatorio

### Fase 3 - Tracking

- webhooks do EvolutionGO
- consolidacao de status por mensagem
- historico de eventos
- retries automaticos

### Fase 4 - Operacao

- dashboard simples
- metricas e logs
- ajustes de performance e robustez

## MVP v1

O primeiro passo concreto de construcao do `DisparaGO` sera a entrega de um MVP funcional ponta a ponta, priorizando a espinha dorsal do produto antes de recursos avancados.

### Objetivo do MVP

Entregar uma primeira versao capaz de:

- criar campanhas
- registrar destinatarios
- enfileirar mensagens
- processar envios com worker
- enviar via EvolutionGO
- consultar progresso basico por API

### Escopo do MVP

Itens que entram na v1 inicial:

- projeto backend em `Go + Fiber`
- configuracao de ambiente
- conexao com `PostgreSQL`
- conexao com `Redis`
- integracao inicial com `EvolutionGO`
- tabela de campanhas
- tabela de mensagens
- endpoint para criar campanha
- endpoint para listar campanha
- endpoint para listar mensagens da campanha
- worker simples para consumo da fila
- atualizacao de status inicial `pendente`, `processando`, `enviado`, `falhou`

Itens que ficam para a sequencia:

- pause/resume
- dashboard
- retries avancados
- tracking completo ate `entregue` e `lido`
- metricas detalhadas
- controles administrativos mais completos

### Ordem de Implementacao

#### Etapa 1 - Fundacao do servico

- criar estrutura do projeto
- configurar `Fiber`, `PostgreSQL`, `Redis` e variaveis de ambiente
- adicionar healthcheck
- definir padrao de logs

#### Etapa 2 - Modelo de dados

- criar migracoes iniciais
- modelar `campaigns`
- modelar `campaign_messages`
- definir status iniciais

#### Etapa 3 - API basica

- `POST /campaigns`
- `GET /campaigns/:id`
- `GET /campaigns/:id/messages`

#### Etapa 4 - Processamento

- publicar jobs no `Redis`
- criar worker de consumo
- integrar envio com `EvolutionGO`
- registrar resultado do envio

#### Etapa 5 - Validacao operacional

- testar fluxo completo com campanha pequena
- validar persistencia dos estados
- confirmar comportamento de falha e logs

### Entregaveis do Proximo Passo

O proximo passo de desenvolvimento deve gerar:

- scaffold do backend do `DisparaGO`
- estrutura de pastas do projeto
- arquivo de configuracao por ambiente
- conexao funcional com banco e redis
- primeiras migracoes
- primeiros endpoints de campanha

### Definicao de pronto do MVP inicial

Consideraremos esse MVP inicial pronto quando for possivel:

- criar uma campanha via API
- persistir seus destinatarios
- publicar as mensagens na fila
- consumir os jobs com worker
- enviar pelo EvolutionGO
- consultar quais mensagens foram enviadas ou falharam
- repetir o fluxo sem depender de outro sistema externo alem do provider WhatsApp

## Criterios de Sucesso

Consideraremos a primeira entrega bem-sucedida quando o produto:

- processar campanhas de forma estavel e isolada
- permitir pause/resume em tempo real
- registrar status confiavel por mensagem
- reprocessar falhas transientes automaticamente
- oferecer visibilidade operacional suficiente para suporte e atendimento

## Proximos Passos Recomendados

1. validar este documento com produto e operacao
2. confirmar o contrato HTTP da API publica do `DisparaGO`
3. definir o payload exato esperado pelo EvolutionGO
4. fechar o modelo de dados inicial
5. iniciar o scaffold do servico em Go

## Observacoes

Este documento funciona como especificacao inicial do produto. Durante o desenvolvimento, ele pode ser desdobrado em:

- documento de arquitetura
- contrato de API em OpenAPI/Swagger
- backlog tecnico
- plano de deploy e operacao
