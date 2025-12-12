# 🛡️ Edge Video - Configuração de Resiliência (Windows)

Este guia configura o Edge Video para ser **100% resiliente** e iniciar automaticamente.

## 📋 O que será configurado

✅ **RabbitMQ** - Auto-start no boot
✅ **Redis/Memurai** - Auto-start no boot
✅ **Edge Video** - Watchdog que monitora e reinicia automaticamente
✅ **Logs** - Registro de todas as atividades

---

## 🚀 Instalação Rápida (Recomendada)

### **1️⃣ Configurar Serviços de Infraestrutura**

Execute no PowerShell como **Administrador**:

```powershell
# Configurar RabbitMQ e Redis para auto-start
Set-Service -Name RabbitMQ -StartupType Automatic
Set-Service -Name Memurai -StartupType Automatic

# Verificar
Get-Service RabbitMQ, Memurai | Select-Object Name, StartType, Status
```

**Resultado esperado:**
```
Name     StartType Status
----     --------- ------
RabbitMQ Automatic Running
Memurai  Automatic Running
```

### **2️⃣ Instalar Edge Video Watchdog**

**ANTES DE EXECUTAR**: Edite o arquivo `edge-video-watchdog.ps1` e ajuste o caminho:

```powershell
$EdgeVideoPath = "C:\Users\maisfluxo\Desktop\edge-video-1.2"  # ✅ Ajuste aqui!
```

Agora execute no PowerShell como **Administrador**:

```powershell
cd D:\Users\rafa2\OneDrive\Desktop\edge-video\deployment\windows

# Executar instalador
.\install-watchdog-service.ps1
```

**Pronto!** O sistema agora está 100% resiliente! 🎉

---

## 🔍 Verificar se Está Funcionando

```powershell
# Ver status da tarefa agendada
Get-ScheduledTask -TaskName EdgeVideoWatchdog

# Ver processos rodando
Get-Process edge-video -ErrorAction SilentlyContinue

# Ver logs do watchdog
Get-Content "C:\Users\maisfluxo\Desktop\edge-video-1.2\logs\watchdog.log" -Tail 20

# Ver logs do Edge Video
Get-Content "C:\Users\maisfluxo\Desktop\edge-video-1.2\logs\service-stdout.log" -Tail 20
```

---

## 🛠️ Comandos Úteis

### **Gerenciar Watchdog**

```powershell
# Parar watchdog
Stop-ScheduledTask -TaskName EdgeVideoWatchdog

# Iniciar watchdog
Start-ScheduledTask -TaskName EdgeVideoWatchdog

# Desinstalar watchdog
Unregister-ScheduledTask -TaskName EdgeVideoWatchdog -Confirm:$false
```

### **Gerenciar Serviços**

```powershell
# RabbitMQ
Stop-Service RabbitMQ
Start-Service RabbitMQ
Restart-Service RabbitMQ

# Redis/Memurai
Stop-Service Memurai
Start-Service Memurai
Restart-Service Memurai
```

---

## 📊 Monitoramento

### **Logs**

Localização: `C:\Users\maisfluxo\Desktop\edge-video-1.2\logs\`

- `watchdog.log` - Log do sistema de monitoramento
- `service-stdout.log` - Saída padrão do Edge Video
- `service-stderr.log` - Erros do Edge Video

### **Ver logs em tempo real**

```powershell
# PowerShell
Get-Content "C:\Users\maisfluxo\Desktop\edge-video-1.2\logs\watchdog.log" -Wait

# CMD
tail -f "C:\Users\maisfluxo\Desktop\edge-video-1.2\logs\watchdog.log"
```

---

## 🧪 Testar Resiliência

### **Teste 1: Matar processo manualmente**

```powershell
# Matar Edge Video
Get-Process edge-video | Stop-Process -Force

# Aguardar 10-15 segundos e verificar se reiniciou
Get-Process edge-video
```

✅ **Esperado**: Processo reinicia automaticamente

### **Teste 2: Reiniciar máquina**

```powershell
Restart-Computer
```

Após o boot:
```powershell
# Verificar se tudo subiu
Get-Service RabbitMQ, Memurai
Get-ScheduledTask -TaskName EdgeVideoWatchdog
Get-Process edge-video
```

✅ **Esperado**: Todos os serviços rodando

---

## 🆘 Troubleshooting

### **Watchdog não inicia**

```powershell
# Ver detalhes da tarefa
Get-ScheduledTaskInfo -TaskName EdgeVideoWatchdog

# Ver logs do Event Viewer
Get-WinEvent -LogName "Microsoft-Windows-TaskScheduler/Operational" -MaxEvents 20 | Where-Object {$_.Message -like "*EdgeVideoWatchdog*"}
```

### **Edge Video não reinicia após falha**

1. Verifique os logs: `watchdog.log`
2. Verifique permissões da pasta
3. Teste execução manual do script:

```powershell
cd C:\Users\maisfluxo\Desktop\edge-video-1.2
.\edge-video-watchdog.ps1
```

### **Serviços não sobem no boot**

```powershell
# Reconfigurar auto-start
Set-Service -Name RabbitMQ -StartupType Automatic
Set-Service -Name Memurai -StartupType Automatic

# Verificar dependências
Get-Service RabbitMQ | Select-Object -ExpandProperty DependentServices
```

---

## 🎯 Alternativa: NSSM (Mais Simples)

Se preferir uma solução ainda mais robusta:

```powershell
# Instalar NSSM
choco install nssm -y

# Criar serviço
nssm install EdgeVideoService "C:\Users\maisfluxo\Desktop\edge-video-1.2\edge-video.exe"
nssm set EdgeVideoService AppDirectory "C:\Users\maisfluxo\Desktop\edge-video-1.2"
nssm set EdgeVideoService AppExit Default Restart
nssm set EdgeVideoService AppRestartDelay 5000

# Iniciar
nssm start EdgeVideoService

# Verificar
nssm status EdgeVideoService
```

**Vantagens do NSSM:**
- Mais robusto para serviços nativos
- Interface GUI: `nssm edit EdgeVideoService`
- Restart automático nativo

**Desvantagens:**
- Requer instalação adicional (NSSM)

---

## ✅ Checklist Final

Após instalação, verifique:

- [ ] RabbitMQ configurado para auto-start
- [ ] Memurai configurado para auto-start
- [ ] Tarefa EdgeVideoWatchdog criada e rodando
- [ ] Edge Video processo ativo
- [ ] Logs sendo gravados em `logs/`
- [ ] Teste de kill manual (processo reinicia)
- [ ] Teste de reboot (tudo sobe automaticamente)

---

## 📞 Suporte

Se algo der errado, verifique:

1. **Logs**: `C:\Users\maisfluxo\Desktop\edge-video-1.2\logs\`
2. **Event Viewer**: Windows Logs → Application
3. **Task Scheduler**: Procure por `EdgeVideoWatchdog`
4. **Serviços**: `services.msc` (RabbitMQ e Memurai)

---

**🎉 Sistema 100% Resiliente e Pronto para Produção!**
