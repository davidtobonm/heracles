# Self-install as a user or system command

The Heracles binary will manage explicit installation scopes through `heracles install --user`, `sudo heracles install --system`, and matching uninstall commands. User installation targets `~/.local/bin/heracles` and reports any required `PATH` change; system installation targets `/usr/local/bin/heracles`. This lets a downloaded release become a normal command without a language toolchain, while contributors may continue using `make install`.

`heracles update check` reports whether a newer stable GitHub Release exists, and `heracles update` downloads the correct release binary, verifies its published checksum, and atomically replaces the current user or system installation. System updates require elevated permission. Heracles never updates itself during a Labor.

Heracles does not require cron jobs for update awareness. Ordinary interactive commands trigger a silent, non-blocking cached update check and report an available version after the command finishes. Update checks never delay or fail the requested command and emit no notices during JSON, quiet, MCP, or active Labor output.
