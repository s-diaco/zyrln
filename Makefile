GOCACHE     ?= /tmp/go-build-cache
ANDROID_HOME ?= $(HOME)/Android/Sdk
GOTOOLCHAIN  ?= go1.25.0
GOFLAGS      ?= -buildvcs=false
AAR_OUT       = android/app/libs/mobile.aar
APK_VERSION   = 1.2
APK_RELEASE   = android/app/build/outputs/apk/release/zyrln-$(APK_VERSION).apk
APK_DEBUG     = android/app/build/outputs/apk/debug/zyrln-$(APK_VERSION).apk

export ANDROID_HOME
export GOTOOLCHAIN
export GOFLAGS

.PHONY: all desktop proxy test android android-debug keystore clean

all: desktop

## Build the desktop CLI binary.
desktop:
	GOCACHE=$(GOCACHE) go build -o zyrln ./platforms/desktop/

## Start the desktop relay proxy (reads config.env).
proxy:
	@if [ ! -f certs/zyrln-ca.pem ]; then \
		echo "CA certificate not found. Run this first:"; \
		echo "  make desktop && ./zyrln -init-ca"; \
		exit 1; \
	fi
	GOCACHE=$(GOCACHE) go run ./platforms/desktop/ -serve-proxy

## Smoke test the full relay chain.
test:
	GOCACHE=$(GOCACHE) go run ./platforms/desktop/ -relay-fetch-url 'https://www.gstatic.com/generate_204'

## Generate a release signing keystore (run once before `make android`).
## Requires: keytool (comes with the JDK)
keystore:
	@if [ -f android/keystore.properties ]; then \
		echo "Keystore already exists. Delete android/zyrln.jks and android/keystore.properties to regenerate."; \
		exit 1; \
	fi
	keytool -genkeypair -v \
		-keystore android/zyrln.jks \
		-alias zyrln \
		-keyalg RSA -keysize 2048 -validity 10000 \
		-storepass zyrln123 -keypass zyrln123 \
		-dname "CN=Zyrln, O=Zyrln, C=US"
	@printf 'storeFile=../zyrln.jks\nstorePassword=zyrln123\nkeyAlias=zyrln\nkeyPassword=zyrln123\n' \
		> android/keystore.properties
	@echo "Keystore → android/zyrln.jks"
	@echo "Properties → android/keystore.properties"

## Build the release APK (requires keystore first).
android:
	@echo "Building gomobile AAR..."
	@mkdir -p android/app/libs
	PATH=$(PATH):$(HOME)/go/bin GOCACHE=$(GOCACHE) gomobile bind \
		-target android \
		-androidapi 21 \
		-o $(AAR_OUT) \
		zyrln/platforms/mobile
	cd android && ./gradlew assembleRelease
	@echo "APK → $(APK_RELEASE)"

## Build a debug APK (no keystore needed).
android-debug:
	@echo "Building gomobile AAR..."
	@mkdir -p android/app/libs
	PATH=$(PATH):$(HOME)/go/bin GOCACHE=$(GOCACHE) gomobile bind \
		-target android \
		-androidapi 21 \
		-o $(AAR_OUT) \
		zyrln/platforms/mobile
	cd android && ./gradlew assembleDebug
	@echo "APK → $(APK_DEBUG)"


clean:
	rm -f zyrln $(AAR_OUT)
	cd android && ./gradlew clean 2>/dev/null || true
