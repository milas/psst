# psst 🤫
Sometimes you need to take a little peek at your Kubernetes secrets...

# Usage
You can run `psst` without any arguments to interactively pick the secret and key whose value you want to see.

```shell
$ psst
✔  Choose secret: … my-secret
✔  Choose key: … my-key
my-value
```

Alternatively, you can run `psst SECRET KEY` to get the value directly.

It's also possible to only provide the `SECRET` and interactively choose the key.

If the secret only contains a single key, it'll be chosen automatically.

Run `psst --help` for all options. (For example, you can use the standard `kubectl` flags to specify namespace and/or context.)

# Installation
If you've got Go installed:
```shell
go install 'github.com/milas/psst@latest'
```

You can also build & install in one command with Docker (you do not need to clone the repo):
```shell
DEST="${HOME}/.local/bin/" docker buildx bake \
  'https://github.com/milas/psst.git'
```
> Be sure `~/.local/bin` is in your `PATH` or pick a different directory.

## Shell Completions
### Bash
```shell
source <(psst --completion=bash)
```

To load completions for each session, execute once:
```shell
psst --completion=bash > /etc/bash_completion.d/psst
```

> NOTE: For Homebrew installs of `bash`, use `$(brew --prefix)/etc/bash_completion.d/psst`

### Zsh
```shell
source <(psst --completion=zsh); compdef _psst psst
```

To load completions for each session, execute once:
```shell
psst --completion=zsh > "${fpath[1]}/_psst"
```

### fish
```shell
psst --completion=fish | source
```

To load completions for each session, execute once:
```shell
psst --completion=fish > ~/.config/fish/completions/psst.fish
```

### PowerShell
```shell
psst --completion=powershell | Out-String | Invoke-Expression
```

# License
Licensed under the [Apache License, Version 2.0](./LICENSE)
