# Support macOS, Linux, and Windows

The first distributable Heracles release supports macOS, Linux, and native
Windows on `amd64` and `arm64` where the platform and Go toolchain support the
architecture. Windows support is not limited to WSL.

Release CI builds, tests, checksums, and publishes supported platform binaries.
Provider adapters, permission bypasses, process control, filesystem behavior,
installation paths, and Doctor checks account for platform-specific
capabilities. Unsupported provider and platform combinations fail clearly
instead of silently degrading.
