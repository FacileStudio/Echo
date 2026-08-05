# Echo — Documentation

This set documents what Facile changed in the fork and how Facile runs it. For upstream Jitsi
Meet behavior — conferencing features, Prosody modules, `lib-jitsi-meet`, the iframe API — use
the [Jitsi handbook](https://jitsi.github.io/handbook/), which is where the old `doc/*.md`
redirect stubs pointed.

| Page | What's in it |
|---|---|
| [Architecture](architecture.md) | Service topology, the Facile delta, transcript and summary flow |
| [Configuration](configuration.md) | Every environment variable, `config.js`, `interface_config.js` |
| [Development](development.md) | Build targets, the lint gate, tests, working with the fork |
| [Deployment](deployment.md) | The image, Compose, Dokploy and Traefik, ports, upgrades |
| [API](api.md) | The AI summary proxy, the nginx routes, the iframe API |

`doc/` (singular) is not documentation. It holds upstream Jitsi packaging payload —
`doc/debian/` and `doc/jaas/` are referenced by exact path from `debian/*.install`, so they
cannot move.

Back to the [README](../README.md).
