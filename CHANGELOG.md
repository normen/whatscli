# Changelog

All notable changes to the WhatsApp CLI application will be documented in this file.

## [Unreleased] - 2023-03-17

### Changed
- Migrated from deprecated `go-whatsapp` library to the newer `whatsmeow` library
- Complete rewrite of the session management system for better reliability
- Enhanced message handling with improved error handling and feedback
- Updated UI interactions to work with the new backend

### Added
- Automatic loading of recent chats after successful login or session restoration
- Loading of the 20 most recent messages for each chat, including text and image messages with captions
- Loading of WhatsApp contacts from the address book to ensure proper name display
- Improved backlog command with multiple strategies for retrieving message history:
  - Read receipt trigger method
  - Chat presence (typing status) trigger method
  - Direct history sync notification method
- Better error handling with user-friendly messages
- More robust session restoration and QR code login process

### Fixed
- Nil pointer dereference errors in message history retrieval
- Type mismatches in the backlog command implementation
- Issues with contact name display in messages and chat list
- Improved session reconnection after disconnect

### Security
- Updated to the latest WhatsApp protocol implementation
- Better handling of encrypted sessions 
