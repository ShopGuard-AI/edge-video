# API Registration

O Edge Video pode registrar-se automaticamente em uma API externa ao iniciar, enviando informações de configuração para facilitar o gerenciamento centralizado de múltiplas instâncias.

## Visão Geral

A funcionalidade de registro permite que cada instância do Edge Video:

- ✅ **Auto-registro**: Envia dados de configuração ao iniciar
- ✅ **Retry Automático**: Tenta novamente a cada 1 minuto em caso de falha
- ✅ **Background Process**: Não bloqueia a captura de vídeo
- ✅ **Informações Completas**: Envia câmeras, namespace, RabbitMQ, exchange, routing key

## Configuração

### Habilitando o Registro

```toml
# config.toml

[registration]
enabled = true
api_url = "http://api.example.com/register"
```

### Parâmetros

| Parâmetro | Tipo | Descrição |
|-----------|------|-----------|
| `enabled` | `bool` | Habilita/desabilita o registro na API |
| `api_url` | `string` | URL completa do endpoint de registro |

## Payload Enviado

A API receberá um POST com o seguinte JSON:

```json
{
  "cameras": [
    {
      "id": "cam1",
      "url": "rtsp://admin:pass@192.168.1.100:554/stream1"
    },
    {
      "id": "cam2",
      "url": "rtsp://admin:pass@192.168.1.101:554/stream1"
    }
  ],
  "namespace": "supermercado_vhost",
  "rabbitmq_url": "amqp://user:pass@rabbitmq:5672/supermercado_vhost",
  "routing_key": "camera.",
  "exchange": "cameras",
  "vhost": "supermercado_vhost"
}
```

### Campos do Payload

| Campo | Tipo | Descrição |
|-------|------|-----------|
| `cameras` | `array` | Lista de câmeras com ID e URL RTSP |
| `namespace` | `string` | Identificador do cliente (extraído do vhost) |
| `rabbitmq_url` | `string` | URL completa do RabbitMQ |
| `routing_key` | `string` | Prefixo da routing key (ex: "camera.") |
| `exchange` | `string` | Nome do exchange RabbitMQ |
| `vhost` | `string` | Virtual host do RabbitMQ |

## Comportamento

### Fluxo de Registro

```mermaid
sequenceDiagram
    participant EV as Edge Video
    participant API as API de Registro
    
    EV->>EV: Carrega configuração
    EV->>EV: Extrai vhost
    EV->>API: POST /register (payload)
    
    alt Sucesso
        API-->>EV: 200 OK
        EV->>EV: Continua operação normal
    else Falha
        API-->>EV: 500 Error
        EV->>EV: Aguarda 1 minuto
        EV->>API: POST /register (retry)
    end
```

### Retry Logic

1. **Primeira tentativa**: Imediata ao iniciar o sistema
2. **Falha detectada**: Registra erro no log
3. **Retry automático**: Tenta novamente a cada 1 minuto
4. **Background**: Executa em goroutine separada
5. **Sucesso**: Para os retries automaticamente

**Importante**: O sistema continua capturando frames mesmo durante os retries. O registro não bloqueia a operação principal.

## Exemplo de API Receptora

### Python (Flask)

```python
from flask import Flask, request, jsonify

app = Flask(__name__)

@app.route('/register', methods=['POST'])
def register():
    data = request.json
    
    # Validar dados
    if not data.get('cameras') or not data.get('vhost'):
        return jsonify({"error": "Missing required fields"}), 400
    
    # Salvar no banco de dados
    save_camera_instance(data)
    
    print(f"Registered {len(data['cameras'])} cameras for vhost: {data['vhost']}")
    
    return jsonify({"message": "Registration successful"}), 200

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=8080)
```

### Go (Gin)

```go
package main

import (
    "github.com/gin-gonic/gin"
    "net/http"
)

type RegistrationPayload struct {
    Cameras     []CameraInfo `json:"cameras"`
    Namespace   string       `json:"namespace"`
    RabbitMQURL string       `json:"rabbitmq_url"`
    RoutingKey  string       `json:"routing_key"`
    Exchange    string       `json:"exchange"`
    Vhost       string       `json:"vhost"`
}

type CameraInfo struct {
    ID  string `json:"id"`
    URL string `json:"url"`
}

func main() {
    r := gin.Default()
    
    r.POST("/register", func(c *gin.Context) {
        var payload RegistrationPayload
        
        if err := c.BindJSON(&payload); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }
        
        // Salvar no banco de dados
        // saveToDatabase(payload)
        
        c.JSON(http.StatusOK, gin.H{
            "message": "Registration successful",
            "cameras_count": len(payload.Cameras),
        })
    })
    
    r.Run(":8080")
}
```

### Node.js (Express)

```javascript
const express = require('express');
const app = express();

app.use(express.json());

app.post('/register', (req, res) => {
    const { cameras, namespace, vhost, rabbitmq_url, exchange, routing_key } = req.body;
    
    if (!cameras || !vhost) {
        return res.status(400).json({ error: 'Missing required fields' });
    }
    
    // Salvar no banco de dados
    // saveToDatabase(req.body);
    
    console.log(`Registered ${cameras.length} cameras for vhost: ${vhost}`);
    
    res.json({ 
        message: 'Registration successful',
        cameras_count: cameras.length 
    });
});

app.listen(8080, () => {
    console.log('API listening on port 8080');
});
```

## Logs

### Sucesso

```
INFO    Successfully registered with API
        api_url=http://api.example.com/register
        vhost=supermercado_vhost
        cameras_count=5
```

### Falha com Retry

```
WARN    Failed to register with API, will retry every 1 minute
        error=failed to send registration request: connection refused
        api_url=http://api.example.com/register

WARN    Registration retry failed, will try again in 1 minute
        error=registration failed with status code: 500

INFO    Successfully registered with API after retry
```

## Casos de Uso

### 1. Dashboard Centralizado

Use o registro para popular um dashboard que mostra todas as instâncias ativas:

- Quais câmeras estão capturando
- Qual vhost cada instância usa
- Exchange e routing keys configuradas

### 2. Provisionamento Automático

Configure consumers automaticamente baseado nos dados registrados:

```python
# Ao receber registro, criar consumer para o vhost
def on_registration(payload):
    vhost = payload['vhost']
    cameras = payload['cameras']
    
    # Criar consumer para este vhost
    create_consumer(vhost, cameras)
```

### 3. Monitoramento de Saúde

Compare registros com heartbeats para detectar instâncias offline:

```python
# Se não houver heartbeat por 5 minutos após registro
if time.now() - last_heartbeat > 5_minutes:
    alert(f"Instance {vhost} may be offline")
```

### 4. Auditoria e Compliance

Mantenha histórico de todas as câmeras e configurações:

```sql
CREATE TABLE registrations (
    id SERIAL PRIMARY KEY,
    vhost VARCHAR(255),
    cameras JSONB,
    rabbitmq_url VARCHAR(255),
    registered_at TIMESTAMP DEFAULT NOW()
);
```

## Best Practices

!!! success "Recomendações"
    
    1. **Endpoint Idempotente**: A API deve aceitar múltiplos registros do mesmo vhost
    2. **Validação de Dados**: Valide todos os campos antes de persistir
    3. **Autenticação**: Adicione token de autenticação para segurança
    4. **Rate Limiting**: Proteja a API contra retry excessivo
    5. **Logs Estruturados**: Use logging estruturado para facilitar análise
    6. **Database Indexes**: Indexe pelo vhost para queries rápidas
    7. **Webhook Events**: Envie eventos quando novas instâncias se registram
    8. **Health Endpoint**: Exponha endpoint de health na API

## Troubleshooting

### Erro: "Registration API URL is empty"

**Causa**: `api_url` não foi configurado no `config.toml`

**Solução**:
```toml
[registration]
enabled = true
api_url = "http://api.example.com/register"  # Adicione esta linha
```

### Erro: "Failed to send registration request: connection refused"

**Causa**: API não está acessível na URL configurada

**Soluções**:
1. Verificar se a API está rodando
2. Testar conectividade: `curl http://api.example.com/register`
3. Verificar firewall/rede
4. Confirmar URL no config.toml

### Erro: "Registration failed with status code: 500"

**Causa**: API retornou erro interno

**Soluções**:
1. Verificar logs da API
2. Validar formato do payload
3. Verificar se API aceita application/json
4. Testar endpoint manualmente com curl

### Retry Infinito

**Comportamento**: Sistema continua tentando registrar indefinidamente

**Explicação**: Isso é esperado! O retry continua até ter sucesso ou o sistema ser desligado.

**Para desabilitar**:
```toml
[registration]
enabled = false
```

## Integração com MkDocs

Para mais informações sobre integração:

<div class="grid cards" markdown>

-   :material-api:{ .lg } __Configuration__
    
    Configuração completa do sistema
    
    [:octicons-arrow-right-24: Config Guide](../getting-started/configuration.md)

-   :material-rabbit:{ .lg } __RabbitMQ__
    
    Multi-tenancy com vhosts
    
    [:octicons-arrow-right-24: RabbitMQ Guide](message-queue.md)

-   :material-camera:{ .lg } __Camera Capture__
    
    Captura de câmeras RTSP
    
    [:octicons-arrow-right-24: Camera Guide](camera-capture.md)

</div>
