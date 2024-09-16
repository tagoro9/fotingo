# Shell Completion

## Bash (Linux)

```bash
# Add to ~/.bashrc
source <(fotingo completion bash)

# Or install system-wide
fotingo completion bash > /etc/bash_completion.d/fotingo
```

## Bash (macOS with Homebrew)

```bash
brew install bash-completion
source <(fotingo completion bash)
```

## Zsh

```bash
# Add before compinit in ~/.zshrc
source <(fotingo completion zsh)

# Or generate file in your fpath
fotingo completion zsh > "${fpath[1]}/_fotingo"

# Refresh completion cache if needed
rm -f ~/.zcompdump; compinit
```

## Fish

```bash
# Current session
fotingo completion fish | source

# Persist
fotingo completion fish > ~/.config/fish/completions/fotingo.fish
```

## PowerShell

```powershell
fotingo completion powershell | Out-String | Invoke-Expression
```
