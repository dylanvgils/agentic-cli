# Installation

Requires Git and Docker Engine 26 or later, such as Rancher Desktop. Other engines may work but are untested. The optional microVM runtime has its own [requirements](microsandbox.md#requirements).

Install directly:

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/dylanvgils/agentic-cli/main/install.sh | bash

# Windows (PowerShell)
Invoke-RestMethod https://raw.githubusercontent.com/dylanvgils/agentic-cli/main/install.ps1 | Invoke-Expression
```

Or clone the repo first and run the script from there:

```bash
git clone https://github.com/dylanvgils/agentic-cli.git
cd agentic-cli
./install.sh        # Linux / macOS
.\install.ps1       # Windows (PowerShell)
```

The installer fetches the latest release for your OS and architecture, verifies the checksum, and installs the binary.

> **Windows:** If you get an error about running scripts being disabled, set the execution policy for your user:
>
> ```powershell
> Set-ExecutionPolicy -Scope CurrentUser -ExecutionPolicy RemoteSigned
> ```
>
> Alternatively, run it directly without changing the policy:
>
> ```powershell
> powershell -ExecutionPolicy Bypass -File .\install.ps1
> ```

On Linux/macOS, the binary is installed to `~/.local/bin`. If that directory isn't in your PATH, add it to your shell config:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

On Windows, the installer adds the install directory to your user PATH. Restart your terminal afterwards.

## Uninstalling

To uninstall and remove all agentic data:

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/dylanvgils/agentic-cli/main/install.sh | bash -s -- --remove

# Windows (PowerShell)
& ([scriptblock]::Create((Invoke-RestMethod https://raw.githubusercontent.com/dylanvgils/agentic-cli/main/install.ps1))) -Remove
```

Or if you already have the repo cloned:

```bash
./install.sh --remove
.\install.ps1 -Remove
```

## Updating

Upgrade to the latest release with `agentic upgrade`:

```bash
agentic upgrade                    # update to latest
agentic upgrade --force            # reinstall even if already up to date
agentic upgrade --version v1.2.0   # install a specific release
```

agentic also checks for a new release once a day and offers to upgrade.

## Building from source

Build from source instead of downloading a release:

```bash
./install.sh --from-source    # Linux / macOS
.\install.ps1 -FromSource     # Windows (PowerShell)
```

Or, with Go installed:

```bash
make install        # builds and installs to ~/.local/bin/agentic
make uninstall      # removes the binary
```
