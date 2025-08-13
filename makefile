# 应用名称
APP_NAME := GoDownload

# 版本
VERSION := 1.0.0

# 输出目录
BIN_DIR := bin

# Go 源码主文件
MAIN_FILE := main.go

# 默认任务：编译全部平台
all: clean mac_amd64 mac_arm64 win_amd64

# 清理输出
clean:
	@echo "==> Cleaning..."
	@rm -rf $(BIN_DIR)

# 只编译Mac amd64
mac_amd64:
	@mkdir -p $(BIN_DIR)/darwin_amd64
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 CC=clang go build -ldflags="-s -w" -o $(BIN_DIR)/darwin_amd64/$(APP_NAME) $(MAIN_FILE)

# 只编译Mac arm64
mac_arm64:
	@mkdir -p $(BIN_DIR)/darwin_arm64
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 CC=clang go build -ldflags="-s -w" -o $(BIN_DIR)/darwin_arm64/$(APP_NAME) $(MAIN_FILE)

# 合并成 universal（fat binary） - 需要在 macOS 上运行 lipo
universal: mac_amd64 mac_arm64
	@echo ">>> Creating universal binary..."
	@command -v lipo >/dev/null 2>&1 || (echo "ERROR: lipo not found. Run on macOS with Xcode CLI tools." && exit 1)
	@mkdir -p $(BIN_DIR)/universal
	@lipo -create \
		$(BIN_DIR)/darwin_amd64/$(APP_NAME) \
		$(BIN_DIR)/darwin_arm64/$(APP_NAME) \
		-output $(BIN_DIR)/universal/$(APP_NAME)
	@chmod +x $(BIN_DIR)/universal/$(APP_NAME)
	@echo ">>> Universal binary created at $(BIN_DIR)/universal/$(APP_NAME)"
	@echo ">>> Verification:"
	@file $(BIN_DIR)/universal/$(APP_NAME) || true
	@lipo -info $(BIN_DIR)/universal/$(APP_NAME) || true

# 只编译Windows amd64
win_amd64:
	@mkdir -p $(BIN_DIR)/windows_amd64
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(BIN_DIR)/windows_amd64/$(APP_NAME).exe $(MAIN_FILE)

mac_dmg_build:
	@cp -r $(BIN_DIR)/universal/GoDownload build/mac/GoDownload.app/Contents/MacOS/
	@cd build/mac && create-dmg \
      --volname "GoDownload" \
      --background "dmg.png" \
      --window-pos 400 200 \
      --window-size 660 400 \
      --icon-size 100 \
      --icon "GoDownload.app" 160 185 \
      --hide-extension "GoDownload.app" \
      --app-drop-link 500 185 \
      "GoDownload.dmg" \
      "GoDownload.app/"


.PHONY: all clean mac_amd64 mac_arm64 win_amd64 mac_dmg_build
