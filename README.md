# Echo

Secure video conferencing by [Facile Studio](https://facile.studio). Built on Jitsi Meet.

## Features

- Support for all current browsers
- Mobile applications (iOS & Android)
- Web and native SDKs for integration
- HD audio and video
- Content sharing
- End-to-end encryption
- Chat with private conversations
- Virtual backgrounds

## Development

### Building

- `make dev` - Start development server with webpack-dev-server
- `make compile` - Build production bundles
- `make clean` - Clean build directory
- `make all` - Full build (compile + deploy)

### Code Quality

- `npm run lint-fix` - Automatically fix linting issues
- `npm run tsc:ci` - Run TypeScript checks for both web and native platforms
- `npm run lint:ci` - Run ESLint without type checking

### Testing

- `npm test` - Run full test suite
- `npm run test-single -- <spec-file>` - Run single test file

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Security

See [SECURITY.md](SECURITY.md) for reporting vulnerabilities.

## License

See [LICENSE](LICENSE) for details.
