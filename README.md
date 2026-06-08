# ZapTerm

**Um cliente de WhatsApp para o terminal** — envie e receba mensagens sem abrir o navegador, direto na linha de comando.

> 🎓 **Projeto universitário.** O ZapTerm é uma adaptação/estudo construído sobre o
> projeto open-source [whatscli](https://github.com/normen/whatscli), usando as
> bibliotecas [whatsmeow](https://github.com/tulir/whatsmeow) (conexão com o WhatsApp)
> e [tview](https://github.com/rivo/tview) (interface no terminal).

![screenshot](/doc/screenshot.png?raw=true "ZapTerm")

## O que ele faz

- Envia e recebe mensagens de WhatsApp dentro de um app de terminal
- Conecta pela API do WhatsApp Web, sem precisar de navegador
- Login simples por QR Code
- Baixa e abre anexos (imagem, vídeo, áudio, documento)
- Envia imagens, vídeos, áudios e documentos
- Gerenciamento básico de grupos
- Notificações no desktop
- Cores personalizáveis

### Limitações (importante saber)

- O histórico depende do que o WhatsApp sincroniza com aparelhos conectados; às vezes é preciso usar `/backlog`.
- A Meta não endossa apps assim — eles podem parar de funcionar quando o WhatsApp muda o app web.

---

## Como executar (passo a passo)

Há duas formas. Escolha a que combina com você.

### ✅ Opção A — Usar o programa pronto (NÃO precisa do Go)

Esta é a forma mais simples: baixe a pasta pronta para o seu sistema e use. Não precisa
instalar Go, compilador, nada de programação.

#### Windows

A pasta `ZapTerm-windows/` contém:

| Arquivo | Para que serve |
| --- | --- |
| `ZapTerm.exe` | O programa (já vem com ícone) |
| `Instalar-no-menu-iniciar.bat` | Cria o atalho **ZapTerm** no Menu Iniciar |
| `Abrir-ZapTerm.bat` | Abre o programa na hora, sem instalar |

1. Dê dois cliques em **`Instalar-no-menu-iniciar.bat`**.
2. Abra o **Menu Iniciar** e procure por **ZapTerm**.
3. Clique no atalho — abre uma janela de terminal com o programa.
4. **Escaneie o QR Code** com o WhatsApp do celular
   (*WhatsApp → Aparelhos conectados → Conectar um aparelho*).

> Só quer testar sem instalar? Dê dois cliques em `Abrir-ZapTerm.bat`.
> Na primeira execução o Windows pode mostrar o aviso do SmartScreen: clique em
> **Mais informações → Executar assim mesmo**.

#### Linux

A pasta `ZapTerm-linux/` contém:

| Arquivo | Para que serve |
| --- | --- |
| `zapterm` | O programa já compilado |
| `Instalar-no-menu.sh` | Coloca o **ZapTerm** no menu de aplicativos (tela inicial) |
| `Abrir-ZapTerm.sh` | Lançador inteligente: abre o programa em uma janela de terminal |

1. Abra um terminal na pasta e rode:
   ```sh
   bash Instalar-no-menu.sh
   ```
2. Abra o menu de aplicativos e procure por **ZapTerm**.
3. Clique no ícone — abre um terminal com o programa.
4. **Escaneie o QR Code** com o WhatsApp do celular
   (*WhatsApp → Aparelhos conectados → Conectar um aparelho*).

> Só quer testar sem instalar? Rode `bash Abrir-ZapTerm.sh`.

> **Como a janela de terminal é aberta (sem depender da sua configuração):**
> em vez do tradicional `Terminal=true` do `.desktop` — que depende do
> `x-terminal-emulator` estar configurado e às vezes falha — o atalho chama um
> **lançador inteligente** (`Abrir-ZapTerm.sh`) que detecta um terminal instalado
> (`kitty`, `gnome-terminal`, `konsole`, `alacritty`, `wezterm`, `xfce4-terminal`,
> `tilix`, `xterm`...) e abre o ZapTerm dentro dele. Se você rodar o lançador de
> dentro de um terminal, ele executa direto, sem abrir outra janela.

---

### 🛠️ Opção B — Compilar a partir do código (precisa do Go)

Use se quiser a versão mais recente ou contribuir com o projeto.

1. **Instale o Go** (versão 1.25 ou mais nova) em https://go.dev/dl e confira:
   ```sh
   go version
   ```
2. **Instale o Git** (https://git-scm.com/downloads).
3. **Clone e entre na pasta**:
   ```sh
   git clone <url-do-repositorio> zapterm
   cd zapterm
   ```
4. **Compile e rode** (para o seu próprio sistema):
   ```sh
   go run .          # roda direto
   # ou
   go build          # gera o executável e depois rode-o
   ```
   Também há um `Makefile`: `make run` ou `make build`.

> **Observação técnica:** o ZapTerm usa `go-sqlite3`, que depende de **CGO**, então é
> necessário ter um compilador C (no Linux, o `gcc`).

#### Gerar o executável do Windows (`ZapTerm.exe`) com ícone

O ícone é embutido via um arquivo de recurso `zapterm_windows_amd64.syso` (gerado a
partir de `assets/zapterm.ico`). Para regenerar os artefatos e compilar para Windows
a partir do Linux, é preciso o cross-compilador **mingw**:

```sh
# 1. compilador C para Windows (Debian/Ubuntu)
sudo apt-get install -y gcc-mingw-w64-x86-64

# 2. (opcional) regenerar ícone e recurso embutido
python3 assets/make_icon.py     # gera assets/zapterm.ico e .png
python3 assets/make_syso.py     # gera zapterm_windows_amd64.syso

# 3. compilar o .exe com o ícone embutido
CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc \
  go build -o dist/ZapTerm-windows/ZapTerm.exe .
```

---

## Usando o ZapTerm

A ajuda completa, comandos e atalhos estão dentro do app: digite `/help` (ou tecle `F1`).

### Login

Ao iniciar, o ZapTerm tenta conectar e mostra um **QR Code**. Escaneie com o WhatsApp do
celular. Se o QR não couber na tela, diminua a fonte do terminal ou aumente a janela.
Depois da primeira vez, ele reconecta sozinho. Para sair da conta, digite `/logout`.

### Mensagens e comandos

Selecione uma conversa à esquerda e digite no campo de baixo para enviar. Use `Tab` para
alternar entre a lista de conversas e o campo de digitação. Comandos usam o prefixo `/`
(ex.: `/sendimage /caminho/para/foto.jpg`). Em caminhos, não precisa aspas nem barras
mesmo com espaços.

### Seleção de mensagens

`Ctrl-w` (padrão) entra no modo de seleção de mensagens. Com uma mensagem selecionada,
`o` abre anexos em um programa externo.

#### Exibir imagens

É possível mostrar imagens no terminal usando programas externos que convertem para
caracteres, como `jp2a` ou [PIXterm](https://github.com/eliukblau/pixterm). Configure o
comando em `show_command` no arquivo `whatscli.config` (veja a localização em `/help`).

#### Copiar IDs

Comandos como `/add` e `/remove` pedem um "user id". Copie o id de uma conversa/mensagem
selecionada com `Ctrl-c` e cole no campo com `Ctrl-v` (mapeamentos padrão).

### Notificações

Notificações de desktop usam a biblioteca `gen2brain/beeep`. Ative com
`enable_notifications = true` no `whatscli.config`. Para usar o "bell" do terminal,
`use_terminal_bell = true`.

### Configuração

Atalhos, cores e outras opções ficam no arquivo `whatscli.config`; o comando `/help`
mostra onde ele está.

---

## Estrutura do código (visão geral)

- `main.go` — elementos da interface (app `tview` na rotina principal), mapeamento de
  teclas (`tslocum/cbind`), seleção de mensagens e exibição da lista de conversas.
- `messages/session_manager.go` — roda em uma goroutine separada recebendo mensagens do
  `whatsmeow` (que mantém o websocket com o WhatsApp) e comandos da UI via canais,
  garantindo acesso "thread-safe" à conexão e aos dados.
- `messages/storage.go` — banco de mensagens (`MessageDatabase`).
- `messages/messages.go` — interfaces e estruturas de dados de comunicação.
- `config/settings.go` — singleton `Config` carregado via `gopkg.in/ini.v1` na inicialização.
- `assets/` — gerador do ícone (`make_icon.py`) e do recurso embutido no `.exe` (`make_syso.py`).
- `dist/` — pacotes prontos para distribuição (Windows e Linux).

## Créditos

ZapTerm é baseado no projeto [whatscli](https://github.com/normen/whatscli) (licença MIT).
Bibliotecas principais: [whatsmeow](https://github.com/tulir/whatsmeow) e
[tview](https://github.com/rivo/tview).

## Licença

Distribuído sob a licença MIT, herdada do projeto original.
