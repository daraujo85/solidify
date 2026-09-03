# Solidify — Especificação de Engenharia

> **Status:** arquitetura proposta para implementação
> **Data-base:** 2026-08-19
> **Nome de trabalho:** Solidify
> **Objetivo:** ser um “Lighthouse da qualidade de uma entrega”, com SOLID como protagonista, evidência determinística e revisão por pares com IA.

## 1. O que este pacote contém

Este pacote não é um brainstorming. Ele é a especificação de implementação para um agente de software executar o projeto de forma task-driven, preservando as decisões arquiteturais já tomadas.

Arquivos:

- `AGENT_HANDOFF.md`: instrução pronta para o agente iniciar a implementação sem rediscutir a arquitetura.
- `ARCHITECTURE.md`: visão completa, arquitetura, fluxo, componentes, critérios de aplicabilidade, performance, segurança, persistência, dashboard, PDF e decisões técnicas.
- `CONTRACTS.md`: contratos de CLI, MCP, revisão por pares, scoring, artefatos e integrações externas.
- `TASKS.md`: plano de implementação em tarefas pequenas, ordenadas, com critérios de aceite e testes.
- `REPORT_SPEC.md`: contrato visual e de conteúdo do dashboard/PDF empresarial.
- `solidify.example.json`: configuração de referência para um projeto real.
- `schemas/release-report.schema.json`: contrato JSON canônico do relatório final.
- `schemas/peer-review.schema.json`: contrato JSON das análises independentes de IA.
- `schemas/arbiter.schema.json`: contrato JSON do veredito do Árbitro C.
- `prompts/peer-review.md`: contrato de comportamento do Peer A/Peer B.
- `prompts/arbiter.md`: contrato de comportamento do Árbitro C.

## 2. Proposta em uma frase

**Solidify analisa apenas o que mudou em uma entrega Git, combina métricas objetivas e testes aplicáveis com uma revisão SOLID por pares independente e gera um artefato auditável em JSON, dashboard HTML e PDF.**

## 3. Princípios que não devem ser rediscutidos durante o MVP

1. **SOLID é o centro do produto.** Cada letra recebe avaliação independente, evidências, nota quando aplicável e delta antes/depois.
2. **Evidence first.** Nenhum modelo cria fatos; commits, diffs, arquivos, testes e ferramentas são a fonte primária.
3. **Diff first.** A IA não varre o repositório inteiro. O escopo nasce de `base..head`, com expansão de contexto apenas quando necessária.
4. **Revisão por pares.** Em modo contratual: Peer A e Peer B trabalham de forma cega e independente; Árbitro C compara as duas análises e as evidências.
5. **Contextos isolados.** Peer B nunca recebe o texto do Peer A. Árbitro C é uma chamada/sessão nova e não reutiliza a janela de contexto de nenhum peer.
6. **Modelo não é autoridade.** A autoridade vem do processo, das evidências, do consenso e da arbitragem.
7. **Agnóstico de fornecedor.** O agente principal pode ser Claude Code, Codex ou outro terminal compatível com MCP. Juiz/árbitro usam um endpoint OpenAI-compatible; 9Router é o primeiro adapter de referência, não uma dependência rígida.
8. **Sem custo de API obrigatório.** O fluxo básico funciona apenas com o agente já utilizado pelo desenvolvedor. Juiz e árbitro externos são configuráveis; no cenário de referência podem aproveitar assinaturas/provedores já expostos pelo 9Router.
9. **Leve por padrão.** Não existe daemon obrigatório. O core sobe somente quando chamado. Ferramentas pesadas rodam sob demanda e encerram ao terminar.
10. **Docker-first, não Docker-heavy.** Docker serve para isolamento e portabilidade; serviços pesados não ficam residentes.
11. **Aplicabilidade automática.** Lighthouse só roda para frontend web aplicável; carga só roda em serviços/endpoints aplicáveis; migrations e envs só aparecem quando detectadas; itens não aplicáveis não reduzem nota.
12. **Um artefato canônico.** `release-report.json` é a fonte da verdade. Dashboard e PDF apenas renderizam o mesmo conteúdo.
13. **Segredos nunca entram no relatório.** Nomes de env podem ser registrados; valores, tokens e secrets não.
14. **Rastreabilidade de release.** Commits, features/bugfixes, migrations, envs, testes, segurança, performance, modelos usados e decisões de arbitragem aparecem no relatório.
15. **Falhar de forma honesta.** Em release contratual, ausência de evidência obrigatória resulta em `BLOCKED` ou `INCOMPLETE`, não em uma nota artificialmente alta.

## 4. Experiência-alvo

Fluxo esperado em um terminal com agente:

```text
Usuário: faça a revisão Solidify desta release entre origin/main e HEAD

Agente -> MCP Solidify: begin_review(base="origin/main", head="HEAD", profile="contractual")
Solidify -> coleta Git + detecta componentes + roda analisadores objetivos aplicáveis
Agente -> MCP Solidify: obtém Evidence Bundle
Agente -> produz Peer A em JSON
Agente -> MCP Solidify: submit_peer_review(...)
Solidify -> executa Peer B isolado via provider configurado
Solidify -> executa Árbitro C isolado via provider configurado
Solidify -> consolida scores determinísticos
Solidify -> gera release-report.json + dashboard + PDF
```

O usuário também pode iniciar pelo CLI e depois pedir ao agente para continuar via MCP.

## 5. Saídas de uma execução

```text
.solidify/
  solidify.db
  runs/
    01J.../
      run.json
      evidence/
        manifest.json
        git.json
        changes.json
        migrations.json
        env-changes.json
        test-results.json
        sonar.json
        lighthouse.json
        security.json
        load.json
      reviews/
        peer-a.json
        peer-b.json
        arbiter.json
        robustness.json         # se executado
      release-report.json       # fonte canônica
      report.html
      report.pdf
      logs/
```

## 6. Resultado visual mínimo

O dashboard e o PDF devem destacar, nesta ordem:

- resultado do Quality Gate;
- nota geral da release;
- **S · O · L · I · D** com uma nota/estado por letra;
- delta de cada princípio entre o código anterior e o código entregue;
- confiança da análise e grau de consenso entre peers;
- resumo executivo do que foi entregue;
- features, bugfixes, refactors e breaking changes detectados;
- qualidade estática/Sonar;
- testes;
- segurança;
- performance/carga;
- Lighthouse, quando aplicável;
- migrations;
- novas/alteradas/removidas envs;
- riscos e recomendações;
- transparência da revisão por IA: modelo, papel, provider, latência, prompt version e divergências;
- apêndice de rastreabilidade Git.

## 7. Perfis operacionais

### `quick`

Uso durante desenvolvimento. Coleta Git, stack, mudanças, checks rápidos e Peer A. Não exige segundo modelo nem PDF.

### `release`

Uso antes de merge/release. Executa todos os analisadores aplicáveis configurados e pode usar Peer B + Árbitro C.

### `contractual`

Uso para gerar um laudo técnico que acompanha uma entrega. Exige revisão por pares configurada, árbitro, artefato canônico validado, PDF e cumprimento das regras de Quality Gate declaradas no projeto.

### `strict-plus`

Inclui experimento de robustez com troca de papéis dos modelos externos e comparação de estabilidade do veredito.

## 8. Ordem recomendada de leitura para o agente implementador

1. `AGENT_HANDOFF.md`.
2. `ARCHITECTURE.md` inteiro.
3. `CONTRACTS.md` inteiro.
4. `REPORT_SPEC.md`.
5. `schemas/*.json`.
6. `TASKS.md`.
7. Executar as tarefas em ordem, sem antecipar features de fases posteriores.

## 9. Regra de mudança arquitetural

Se durante a implementação uma decisão desta especificação ficar inviável por incompatibilidade técnica comprovada, o agente deve:

1. registrar o problema em um ADR curto;
2. demonstrar a incompatibilidade com teste ou documentação;
3. propor a alternativa mais leve;
4. preservar os invariantes: diff-first, evidence-first, SOLID-first, revisão independente, artefato canônico, baixo consumo e execução sob demanda.

Não trocar tecnologia apenas por preferência do agente.
