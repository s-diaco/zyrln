package com.zyrln.relay

import org.json.JSONObject

data class RelayConfig(val url: String, val key: String)

object ConfigUtils {
    fun parseImportText(rawText: String): RelayConfig {
        val trimmed = rawText.trim()
        if (trimmed.isEmpty()) {
            throw IllegalArgumentException("empty config")
        }

        val jsonStart = trimmed.indexOf('{')
        val jsonEnd = trimmed.lastIndexOf('}')
        val cleanText = if (jsonStart != -1 && jsonEnd != -1 && jsonEnd > jsonStart) {
            trimmed.substring(jsonStart, jsonEnd + 1)
        } else {
            trimmed
        }

        val json = JSONObject(cleanText)
        val url = json.optString("url", json.optString("URL")).replace(Regex("[\\s]"), "")
        val key = json.optString("key", json.optString("KEY")).trim()
        if (url.isEmpty() || key.isEmpty()) {
            throw IllegalArgumentException("missing url or key")
        }
        return RelayConfig(url, key)
    }

    fun configLabel(url: String): String {
        val first = url.split(",").firstOrNull()?.trim() ?: return url
        val id = first.substringAfter("/macros/s/", "").substringBefore("/")
        if (id.length >= 6) return wordLabel(id)
        return first.substringAfter("://").substringBefore("/").removePrefix("www.")
    }

    internal fun wordLabel(seed: String): String {
        val adj = listOf("swift", "bold", "quiet", "bright", "pure", "sharp", "calm", "free")
        val noun = listOf("relay", "bridge", "tunnel", "gate", "link", "path", "pass", "line")
        var h = 0L
        for (c in seed) h = h * 31 + c.code
        val ai = ((h % adj.size) + adj.size).toInt() % adj.size
        val ni = ((h / adj.size % noun.size) + noun.size).toInt() % noun.size
        return "${adj[ai]} ${noun[ni]}"
    }
}
