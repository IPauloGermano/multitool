# Guia por Plataforma: Linux, Windows e macOS

O **Multitool** foi concebido para fornecer uma experiência consistente independente da plataforma hospedeira. Este guia aborda o comportamento específico em cada sistema operacional.

---

## 🐧 1. Linux

### Distribuições Suportadas e Gerenciadores Nativos

O Multitool identifica a distribuição inspecionando `/etc/os-release`:

| Família / Distro | IDs Reconhecidos | Gerenciador Primário | Comando de Instalação |
| :--- | :--- | :--- | :--- |
| **Ubuntu / Debian** | `ubuntu`, `debian`, `deepin` | `apt` | `sudo apt-get install -y <pkg>` |
| **Fedora / RHEL** | `fedora`, `rhel`, `centos`, `almalinux`, `rocky` | `dnf` | `sudo dnf install -y <pkg>` |
| **Arch Linux** | `arch`, `artix`, `manjaro`, `cachyos` | `pacman` | `sudo pacman -S --noconfirm --needed <pkg>` |
| **openSUSE** | `opensuse-tumbleweed`, `opensuse-leap`, `sles` | `zypper` | `sudo zypper install -y <pkg>` |
| **Solus** | `solus` | `eopkg` | `sudo eopkg install -y <pkg>` |

### Gerenciamento de Privilégios (`sudo`)
* Se o usuário atual **não for root** (`os.Geteuid() != 0`) e o binário `sudo` estiver disponível, o Multitool eleva comandos nativos de sistema de forma automática.
* Ferramentas instaladas via `npm -g`, `pipx` ou `git skill` utilizam o diretório do usuário e **não** exigem privilégios de root.

### Sandboxing e Flatpak
Para ferramentas gráficas empacotadas via Flatpak (ex: `obs`, `dbeaver`, `kdenlive`), o Multitool despacha `flatpak install -y flathub <id>`. A suíte `@flatpakfull` configura o repositório Flathub automaticamente.

---

## 🪟 2. Windows

### Binário Único e Execução Direta
Diferente de scripts PowerShell tradicionais, o executável `multitool.exe` é um binário compilado que:
* **Não requer alterar `ExecutionPolicy`** do PowerShell (`Restricted`, `RemoteSigned`, etc.).
* Não depende de runtimes do .NET ou Node instalados previamente.
* Opera no CMD, Windows PowerShell, PowerShell 7 ou Windows Terminal.

### Suporte Nativo a ARM64 (Copilot+ PCs)
Dispositivos com processadores Qualcomm Snapdragon X Elite e arquitetura ARM64 possuem um binário nativo dedicado:
```cmd
multitool-arm64.exe
```
Isso garante execução direta sem overhead de emulação x86.

### Integração com `winget`
No Windows, os pacotes nativos são instalados via Windows Package Manager:
```cmd
winget install --exact --id <PackageID> --accept-source-agreements --accept-package-agreements --silent
```

---

## 🍎 3. macOS

* Suporta tanto chips Apple Silicon (`arm64` - M1/M2/M3/M4) quanto Intel (`amd64`).
* Utiliza o **Homebrew** (`brew`) como gerenciador de pacotes nativo para ferramentas CLI e utilitários.
* Skills de agentes são vinculadas automaticamente em `~/.claude/skills`, `~/.config/opencode/skills` e `~/.gemini/skills`.
