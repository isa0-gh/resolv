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
- Config validation before starting the DNS listener
- Effective config printing for deployment/debug workflows

---

## Configuration checks

Validate a config file without starting the DNS server:

```sh
resolv -config config.toml -check-config
```

Print the effective TOML config after defaults and file values are loaded:

```sh
resolv -config config.toml -print-config
```

---

## Installation
- [For Linux](docs/install_linux.md)

## License

MIT License
