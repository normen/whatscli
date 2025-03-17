## PR: Library Migration & Feature Improvements

### Description
This PR migrates the WhatsApp CLI application from the deprecated `go-whatsapp` library to the newer `whatsmeow` library. It also adds several new features and improvements to enhance the user experience.

### Changes
- Migrated from `go-whatsapp` to `whatsmeow` library
- Implemented automatic loading of recent chats and contacts
- Added improved message history retrieval with multiple strategies
- Fixed various bugs and type mismatches
- Added a CHANGELOG.md file to track changes

### Testing
- Tested login and session restoration
- Verified chat and contact loading
- Tested message sending and receiving
- Verified backlog command with various strategies

### Documentation
- Added detailed CHANGELOG.md
- Updated code documentation 
