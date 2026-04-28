# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added
- **Windows platform support**: Added PowerShell build script (`build.ps1`) and batch wrapper (`build.bat`)
- **Windows documentation**: Created comprehensive `WINDOWS.md` with installation, usage, and troubleshooting guide
- **Build verification**: Added `test-build.sh` script to verify all platform builds
- **Cross-platform testing**: Automated testing of all platform builds (darwin, linux, windows)

### Changed
- **Updated README**: Added Windows-specific build instructions and link to WINDOWS.md
- **Build scripts**: Enhanced `build.sh` for consistency with PowerShell script

### Technical Details
- Code already uses cross-platform Go APIs (`filepath.Join`, `filepath.ToSlash`, `os.ReadDir`, etc.)
- Windows builds tested and verified on macOS via cross-compilation
- All 5 platforms (darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, windows/amd64) build successfully

## [1.1.0] - 2026-04-XX

### Added
- Initial release
- Spring MVC source scanning
- OpenAPI JSON import
- MCP server for AI assistant integration
- Multi-platform support (macOS, Linux)

## [1.0.0] - Initial Release

### Features
- Backend API route scanning
- DTO/VO schema extraction
- Configurable project conventions
- Safe defaults (127.0.0.1 only, auto-port selection)
