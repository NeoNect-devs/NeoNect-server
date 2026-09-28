# Changelog

## [1.0.0]

This is the first stable release of NeoNect server, establishing the verified operational boundaries for the v1 architecture. 

### Added
- **Semantic Versioning**: Adopted SemVer framework for predictable operational deployments.
- **Operational Configuration**: Completed the transition of runtime limits to environment variables (Commit `336b6a2`).
- **Comprehensive Documentation**: Added beginner and operational guides covering deployment, environment bounds, SQLite constraints, and troubleshooting.

### Scope and Capabilities (v1)
- 1:1 encrypted messaging.
- Encrypted blind relay architecture.
- Multi-device support.
- Offline message delivery via local SQLite persistence.
- (Note: Group chats, voice channels, video calls, and native multipart file upload APIs are NOT implemented).
