<!-- Use this file to provide workspace-specific custom instructions to Copilot. For more details, visit https://code.visualstudio.com/docs/copilot/copilot-customization#_use-a-githubcopilotinstructionsmd-file -->

# Network - Distributed P2P Database System

This is a distributed peer-to-peer network project built with Go and LibP2P. The system provides decentralized database capabilities with consensus and replication.

## Key Components

- **LibP2P Network Layer**: Core networking built on LibP2P for P2P communication
- **Distributed Database**: RQLite-based distributed SQLite with Raft consensus
- **Client Library**: Go API for applications to interact with the network
- **Application Isolation**: Each app gets isolated namespaces for data and messaging

## Development Guidelines

1. **Architecture Patterns**: Follow the client-server pattern where applications use the client library to interact with the distributed network
2. **Namespacing**: All data (database, storage, pub/sub) is namespaced per application to ensure isolation
3. **Error Handling**: Always check for connection status before performing operations
4. **Async Operations**: Use context.Context for cancellation and timeouts
5. **Logging**: Use structured logging with appropriate log levels

## Code Style

- Use standard Go conventions and naming
- Implement interfaces for testability
- Include comprehensive error messages
- Add context parameters to all network operations
- Use dependency injection for components

## Testing Strategy

- Unit tests for individual components
- Integration tests for client library
- E2E tests for full network scenarios
- Mock external dependencies (LibP2P, database)

## Future Applications

This network is designed to support applications like:

- Anchat (encrypted messaging)
- Distributed file storage
- IoT data collection
- Social networks

When implementing applications, they should use the client library rather than directly accessing network internals.
