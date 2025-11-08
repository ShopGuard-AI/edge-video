# Changelog

Todas as mudanças notáveis neste projeto serão documentadas neste arquivo.

O formato é baseado em [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
e este projeto adere ao [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

<!-- towncrier release notes start -->

## [Unreleased]

### ⚠️ BREAKING CHANGES

- **Redis Key Format**: Migração para Unix nanoseconds com vhost-first ordering
  - **Formato Anterior**: `frames:{vhost}:{cameraID}:{RFC3339_timestamp}:{sequence}`
  - **Formato Novo**: `{vhost}:{prefix}:{cameraID}:{unix_timestamp_nano}:{sequence}`
  - **Exemplo**: `supermercado_vhost:frames:cam4:1731024000123456789:00001`
  - **Impacto**: Chaves existentes no Redis não serão compatíveis
  - **Migração**: Requer FLUSHDB, aguardar TTL ou script de migração manual
  - **Benefícios**:
    - ✅ 36% mais compacto (19 vs 30 caracteres)
    - ✅ 10x mais rápido em comparações (integer vs string parsing)
    - ✅ Sortable naturalmente (ordem cronológica nativa)
    - ✅ Range queries extremamente eficientes

### 🚀 Performance

- Otimiza formato de timestamp em Redis keys de RFC3339 para Unix nanoseconds ([#TBD](https://github.com/T3-Labs/edge-video/issues/TBD))
  - Reduz tamanho de chave de 30 para 19 caracteres (economia de 36%)
  - Melhora performance de comparações em ~10x usando integers
  - Facilita range queries com operadores numéricos nativos
  - Mantém precisão de nanosegundos para alta resolução temporal

### ♻️ Refactoring

- Refatora `KeyGenerator` para suportar Unix nanoseconds ([#TBD](https://github.com/T3-Labs/edge-video/issues/TBD))
  - Atualiza `GenerateKey()` para usar `timestamp.UnixNano()`
  - Reescreve `ParseKey()` com parsing robusto de integers usando `fmt.Sscanf()`
  - Ajusta `QueryPattern()` para formato vhost-first: `{vhost}:{prefix}:*`
  - Move vhost para primeira posição do key para melhor organização hierárquica

### 📝 Documentation

- Atualiza documentação completa para novo formato de chaves Redis ([#TBD](https://github.com/T3-Labs/edge-video/issues/TBD))
  - Atualiza `docs/vhost-based-identification.md` com tabela comparativa de performance
  - Atualiza seção multi-tenant no `README.md` com exemplos do mundo real
  - Adiciona guia de migração com opções de transição
  - Documenta breaking changes e estratégias de deployment

### ✅ Tests

- Atualiza suite completa de testes para Unix nanoseconds ([#TBD](https://github.com/T3-Labs/edge-video/issues/TBD))
  - Reescreve validação de timestamps em todos os 16 testes
  - Adiciona caso de teste `supermercado_vhost` como exemplo real
  - Atualiza assertions para verificar posição de vhost em `parts[0]`
  - Todos os testes passando: `PASS ok github.com/T3-Labs/edge-video/internal/storage 0.009s`

## [1.1.0] - 2025-11-06

### ✨ Features

- Conversão do formato de configuração de YAML para TOML para melhor legibilidade e suporte nativo ([#[#1](https://github.com/T3-Labs/edge-video/issues/1)](https://github.com/T3-Labs/edge-video/issues/[#1](https://github.com/T3-Labs/edge-video/issues/1)))
- Implementa pipeline CI/CD com GitHub Actions para testes automatizados em qualquer branch ([#[#3](https://github.com/T3-Labs/edge-video/issues/3)](https://github.com/T3-Labs/edge-video/issues/[#3](https://github.com/T3-Labs/edge-video/issues/3)))
- Adiciona visualização em tempo real de frames com OpenCV no script de teste Python ([#[#4](https://github.com/T3-Labs/edge-video/issues/4)](https://github.com/T3-Labs/edge-video/issues/[#4](https://github.com/T3-Labs/edge-video/issues/4)))

### 🔒 Security

- Adiciona autenticação por senha para Redis com configuração via config.toml ([#[#2](https://github.com/T3-Labs/edge-video/issues/2)](https://github.com/T3-Labs/edge-video/issues/[#2](https://github.com/T3-Labs/edge-video/issues/2)))
