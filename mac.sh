#!/usr/bin/env bash
# CodeSeek Mac Build & Package — all-in-one script
set -euo pipefail
export PATH="$HOME/go/bin:$PATH"

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_NAME="CodeSeek"
GUI_DIR="${ROOT_DIR}/cmd/codeseek-gui"
BUILD_DIR="${ROOT_DIR}/build"
DMG_STAGING="${BUILD_DIR}/dmg_staging"

# ── helpers ──────────────────────────────────────────────────────────────────

build_cli() {
	echo "▶ Building CLI..."
	CGO_ENABLED=0 go build -ldflags="-s -w" -o "${ROOT_DIR}/codeseek" ./cmd/codeseek/
	echo "  ✔  codeseek ($(du -h "${ROOT_DIR}/codeseek" | cut -f1))"
}

build_gui() {
	echo "▶ Building GUI..."

	# Icons
	pushd "${ROOT_DIR}/assets" > /dev/null
	go run gen.go
	popd > /dev/null
	cp "${ROOT_DIR}/assets/icon-c-32.png" "${GUI_DIR}/frontend/src/icon-tray.png"
	cp "${ROOT_DIR}/assets/icon-app.png"  "${GUI_DIR}/frontend/src/icon-app.png"

	# Config template for embed
	cp "${ROOT_DIR}/config.example.yml" "${GUI_DIR}/config.example.yml"

	# Wails bindings
	pushd "${GUI_DIR}" > /dev/null
	wails3 generate bindings

	# Fix runtime import
	find frontend/bindings -name "*.js" -type f | while read -r f; do
		if grep -q '@wailsio/runtime' "$f"; then
			sed -i '' 's|@wailsio/runtime|/wails/runtime.js|g' "$f"
		fi
	done

	OUTPUT="${ROOT_DIR}/codeseek-gui"
	go build -ldflags="-s -w" -o "${OUTPUT}" .
	rm -f config.example.yml
	popd > /dev/null
	echo "  ✔  codeseek-gui ($(du -h "${ROOT_DIR}/codeseek-gui" | cut -f1))"
}

package_dmg() {
	echo "▶ Creating DMG..."
	rm -rf "${BUILD_DIR}"
	mkdir -p "${BUILD_DIR}"

	local APP_BUNDLE="${BUILD_DIR}/${APP_NAME}.app"
	mkdir -p "${APP_BUNDLE}/Contents/MacOS"
	mkdir -p "${APP_BUNDLE}/Contents/Resources"

	cp "${ROOT_DIR}/codeseek-gui" "${APP_BUNDLE}/Contents/MacOS/codeseek-gui"
	chmod 755 "${APP_BUNDLE}/Contents/MacOS/codeseek-gui"

	# ICNS
	local ICON_PNG="${GUI_DIR}/frontend/src/icon-app.png"
	if [ -f "${ICON_PNG}" ]; then
		local ICONSET="${BUILD_DIR}/app.iconset"
		mkdir -p "${ICONSET}"
		for s in 16 32 128 256 512; do
			sips -z $s $s "${ICON_PNG}" --out "${ICONSET}/icon_${s}x${s}.png" >/dev/null 2>&1
		done
		sips -z 32 32   "${ICON_PNG}" --out "${ICONSET}/icon_16x16@2x.png" >/dev/null 2>&1
		sips -z 64 64   "${ICON_PNG}" --out "${ICONSET}/icon_32x32@2x.png" >/dev/null 2>&1
		sips -z 256 256 "${ICON_PNG}" --out "${ICONSET}/icon_128x128@2x.png" >/dev/null 2>&1
		sips -z 512 512 "${ICON_PNG}" --out "${ICONSET}/icon_256x256@2x.png" >/dev/null 2>&1
		sips -z 1024 1024 "${ICON_PNG}" --out "${ICONSET}/icon_512x512@2x.png" >/dev/null 2>&1
		iconutil -c icns -o "${APP_BUNDLE}/Contents/Resources/app.icns" "${ICONSET}" 2>/dev/null || true
		rm -rf "${ICONSET}"
	fi

	# Info.plist
	cat > "${APP_BUNDLE}/Contents/Info.plist" << 'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key>         <string>codeseek-gui</string>
	<key>CFBundleIconFile</key>           <string>app</string>
	<key>CFBundleIdentifier</key>         <string>com.codeseek.app</string>
	<key>CFBundleName</key>               <string>CodeSeek</string>
	<key>CFBundleDisplayName</key>        <string>CodeSeek</string>
	<key>CFBundleVersion</key>            <string>1.0.0</string>
	<key>CFBundleShortVersionString</key> <string>1.0.0</string>
	<key>CFBundlePackageType</key>        <string>APPL</string>
	<key>CFBundleInfoDictionaryVersion</key><string>6.0</string>
	<key>LSMinimumSystemVersion</key>     <string>13.0</string>
	<key>NSHighResolutionCapable</key>    <true/>
</dict>
</plist>
PLIST

	codesign --force --deep --sign - "${APP_BUNDLE}" 2>/dev/null || true

	# Stage for DMG
	rm -rf "${DMG_STAGING}"
	mkdir -p "${DMG_STAGING}"
	cp -R "${APP_BUNDLE}" "${DMG_STAGING}/"
	ln -s /Applications "${DMG_STAGING}/Applications"

	# Detach old volume, build read-write image
	hdiutil detach "/Volumes/${APP_NAME}" -force 2>/dev/null || true
	local TMP_DMG="${BUILD_DIR}/tmp.dmg"
	rm -f "${TMP_DMG}"
	hdiutil create -volname "${APP_NAME}" -srcfolder "${DMG_STAGING}" \
		-ov -format UDRW -fs HFS+ "${TMP_DMG}" 2>&1

	# Mount, layout, clean
	local DEV=$(hdiutil attach -readwrite -noverify -noautoopen "${TMP_DMG}" 2>&1 | awk '/Apple_HFS/{print $1}')
	if [ -n "$DEV" ]; then
		local VOL="/Volumes/${APP_NAME}"
		sleep 1
		open "${VOL}"

		cat > "${BUILD_DIR}/layout.applescript" << 'ASEOF'
tell application "Finder"
	activate
	tell disk "CodeSeek"
		open
		set current view of container window to icon view
		set toolbar visible of container window to false
		set statusbar visible of container window to false
		set bounds of container window to {400, 200, 880, 520}
		set theViewOptions to the icon view options of container window
		set arrangement of theViewOptions to not arranged
		set icon size of theViewOptions to 80
		set position of item "CodeSeek.app" to {140, 130}
		set position of item "Applications" to {360, 130}
		close
	end tell
end tell
ASEOF
		osascript "${BUILD_DIR}/layout.applescript" 2>/dev/null || true

		sleep 2; sync
		rm -rf "${VOL}/.fseventsd" 2>/dev/null || true
		rm -rf "${VOL}/.Spotlight-V100" 2>/dev/null || true
		rm -rf "${VOL}/.Trashes" 2>/dev/null || true

		sleep 1
		hdiutil detach "$DEV" -force 2>/dev/null
	fi

	local DMG_OUTPUT="${ROOT_DIR}/${APP_NAME}.dmg"
	rm -f "${DMG_OUTPUT}"
	hdiutil convert "${TMP_DMG}" -format UDZO -o "${DMG_OUTPUT}" 2>&1
	rm -f "${TMP_DMG}"
	rm -rf "${DMG_STAGING}" "${BUILD_DIR}/dmg"
	echo "  ✔  ${APP_NAME}.dmg ($(du -h "${DMG_OUTPUT}" | cut -f1))"
}

clean() {
	rm -rf "${ROOT_DIR}/codeseek" "${ROOT_DIR}/codeseek-gui" \
	       "${ROOT_DIR}/${APP_NAME}.dmg" "${BUILD_DIR}"
	echo "✔ Cleaned"
}

# ── main ─────────────────────────────────────────────────────────────────────

case "${1:-all}" in
	cli)   build_cli ;;
	gui)   build_gui ;;
	dmg)   build_gui && package_dmg ;;
	clean) clean ;;
	all)   build_cli && build_gui && package_dmg ;;
	*)
		echo "Usage: $0 {cli|gui|dmg|all|clean}"
		echo "  cli    Build CLI binary only"
		echo "  gui    Build GUI app only"
		echo "  dmg    Build GUI + DMG installer"
		echo "  all    Build CLI + GUI + DMG (default)"
		echo "  clean  Remove build artifacts"
		exit 1
		;;
esac
