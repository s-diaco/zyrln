# Keep gomobile-generated classes
-keep class mobile.** { *; }
-keep class go.** { *; }

# Keep VPN service and activity (referenced by AndroidManifest)
-keep class com.zephyr.relay.RelayVpnService { *; }
-keep class com.zephyr.relay.MainActivity { *; }
