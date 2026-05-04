# Keep gomobile-generated classes
-keep class mobile.** { *; }
-keep class go.** { *; }

# Keep VPN service and activity (referenced by AndroidManifest)
-keep class com.zyrln.relay.RelayVpnService { *; }
-keep class com.zyrln.relay.MainActivity { *; }
