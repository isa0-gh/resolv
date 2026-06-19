# resolv

**resolv** is a simple DNS-over-HTTPS (DoH) server written in Go. It allows you to run a local DoH server that forwards DNS queries to popular public resolvers.  

[GitHub Repository](https://github.com/isa0-gh/resolv)

---

## Features

- Lightweight and easy to configure
- Supports multiple upstream resolvers:
  - Cloudflare
  - Google
  - Quad9
  - AdGuard
  - Cisco
- Configurable TTL and bind address
- Graceful UDP listener shutdown on SIGINT and SIGTERM

---

## Installation
- [For Linux](docs/install_linux.md)

## Shutdown

When running `resolv`, press `Ctrl+C` or send `SIGTERM` to stop the UDP listener gracefully. The server closes its UDP socket, stops the cache flusher, and exits cleanly instead of continuing to log read errors.

## License

MIT License
