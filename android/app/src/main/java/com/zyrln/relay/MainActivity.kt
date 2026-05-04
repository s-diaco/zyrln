package com.zyrln.relay

import android.app.AlertDialog
import android.content.BroadcastReceiver
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.content.SharedPreferences
import android.graphics.Typeface
import android.net.VpnService
import android.os.Bundle
import android.os.Environment
import android.provider.Settings
import android.util.Log
import android.view.Gravity
import android.view.View
import android.widget.LinearLayout
import android.widget.TextView
import android.widget.Toast
import org.json.JSONArray
import org.json.JSONException
import org.json.JSONObject
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.cardview.widget.CardView
import androidx.core.content.ContextCompat
import com.zyrln.relay.databinding.ActivityMainBinding
import mobile.Mobile
import java.io.File

class MainActivity : AppCompatActivity() {

    private lateinit var binding: ActivityMainBinding
    private lateinit var prefs: SharedPreferences
    private var activeUrl: String? = null
    private var activeKey: String? = null

    private val vpnPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult()
    ) { result ->
        if (result.resultCode == RESULT_OK) {
            launchVpnService()
        } else {
            activeUrl = null
            activeKey = null
            Toast.makeText(this, R.string.error_vpn_permission, Toast.LENGTH_SHORT).show()
            refreshList(running = false)
        }
    }

    private val startedReceiver = object : BroadcastReceiver() {
        override fun onReceive(context: Context?, intent: Intent?) {
            updateUI(running = true)
        }
    }

    private val stopReceiver = object : BroadcastReceiver() {
        override fun onReceive(context: Context?, intent: Intent?) {
            if (activeUrl == null) {
                // genuine stop — no config switch in progress
                activeKey = null
                updateUI(running = false)
            }
            // activeUrl != null means connectConfig() already set the next config; don't clear it
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        binding = ActivityMainBinding.inflate(layoutInflater)
        setContentView(binding.root)

        prefs = getSharedPreferences("config", Context.MODE_PRIVATE)

        binding.btnImportConfig.setOnClickListener { importConfig() }
        binding.btnInstallCA.setOnClickListener { installCACert() }

        if (Mobile.isRunning()) {
            activeUrl = prefs.getString("url", null)
            activeKey = prefs.getString("key", null)
        }
        updateUI(running = Mobile.isRunning())
    }

    override fun onResume() {
        super.onResume()
        registerReceiver(startedReceiver, IntentFilter("com.zyrln.relay.STARTED"), RECEIVER_NOT_EXPORTED)
        registerReceiver(stopReceiver, IntentFilter("com.zyrln.relay.STOPPED"), RECEIVER_NOT_EXPORTED)
        if (Mobile.isRunning() && activeUrl == null) {
            activeUrl = prefs.getString("url", null)
            activeKey = prefs.getString("key", null)
        }
        updateUI(running = Mobile.isRunning())
    }

    override fun onPause() {
        super.onPause()
        unregisterReceiver(startedReceiver)
        unregisterReceiver(stopReceiver)
    }

    private fun updateUI(running: Boolean) {
        runOnUiThread {
            binding.statusDot.backgroundTintList = ContextCompat.getColorStateList(this,
                if (running) R.color.dot_active else R.color.dot_inactive)
            binding.statusText.setText(if (running) R.string.status_running else R.string.status_stopped)
            binding.btnImportConfig.isEnabled = !running
            refreshList(running)
        }
    }

    private fun refreshList(running: Boolean = Mobile.isRunning()) {
        val configs = loadConfigs()
        binding.configList.removeAllViews()

        if (configs.isEmpty()) {
            binding.configScroll.visibility = View.GONE
            binding.emptyState.visibility = View.VISIBLE
            return
        }

        binding.configScroll.visibility = View.VISIBLE
        binding.emptyState.visibility = View.GONE

        val dp = resources.displayMetrics.density

        val hostnames = configs.map { (url, _) -> configLabel(url) }

        configs.forEach { (url, key) ->
            val isActive = url == activeUrl && key == activeKey

            val card = CardView(this).apply {
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                ).apply { bottomMargin = (10 * dp).toInt() }
                radius = 12 * dp
                cardElevation = 2 * dp
            }

            val row = LinearLayout(this).apply {
                orientation = LinearLayout.HORIZONTAL
                gravity = Gravity.CENTER_VERTICAL
                val p = (16 * dp).toInt()
                setPadding(p, p, p, p)
            }

            val dot = View(this).apply {
                layoutParams = LinearLayout.LayoutParams(
                    (10 * dp).toInt(), (10 * dp).toInt()
                ).apply { marginEnd = (14 * dp).toInt() }
                background = ContextCompat.getDrawable(this@MainActivity, R.drawable.status_dot)
                backgroundTintList = ContextCompat.getColorStateList(this@MainActivity,
                    if (isActive) R.color.dot_active else R.color.dot_inactive)
            }

            val baseLabel = configLabel(url)
            val displayLabel = if (hostnames.count { it == baseLabel } > 1)
                "$baseLabel …${key.takeLast(4)}" else baseLabel

            val label = TextView(this).apply {
                layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
                text = displayLabel
                textSize = 16f
                maxLines = 1
                ellipsize = android.text.TextUtils.TruncateAt.END
                if (isActive) setTypeface(null, Typeface.BOLD)
            }

            val urlList = url.split(",").map { it.trim() }.filter { it.isNotEmpty() }
            val infoBtn = android.widget.ImageButton(this).apply {
                visibility = if (urlList.size > 1) View.VISIBLE else View.GONE
                setImageDrawable(ContextCompat.getDrawable(this@MainActivity, android.R.drawable.ic_menu_info_details))
                background = null
                layoutParams = LinearLayout.LayoutParams(
                    (32 * dp).toInt(), (32 * dp).toInt()
                ).apply { marginEnd = (4 * dp).toInt() }
            }
            infoBtn.setOnClickListener {
                val lines = urlList.mapIndexed { i, u ->
                    val id = u.substringAfter("/macros/s/", "").substringBefore("/")
                    val short = if (id.length >= 6) "…${id.takeLast(10)}" else u.substringAfter("://").substringBefore("/")
                    "${i + 1}. $short"
                }.joinToString("\n")
                AlertDialog.Builder(this@MainActivity)
                    .setTitle("${urlList.size} Apps Script URLs")
                    .setMessage(lines)
                    .setPositiveButton("OK", null)
                    .show()
            }

            val action = TextView(this).apply {
                text = if (isActive && running) "Disconnect" else "Connect"
                textSize = 13f
                setTextColor(ContextCompat.getColor(this@MainActivity,
                    if (isActive && running) R.color.dot_active else android.R.color.darker_gray))
            }

            val deleteBtn = android.widget.ImageButton(this).apply {
                layoutParams = LinearLayout.LayoutParams(
                    (36 * dp).toInt(), (36 * dp).toInt()
                ).apply { marginStart = (8 * dp).toInt() }
                setImageDrawable(ContextCompat.getDrawable(this@MainActivity, android.R.drawable.ic_menu_delete))
                background = null
            }

            row.addView(dot)
            row.addView(label)
            row.addView(infoBtn)
            row.addView(action)
            row.addView(deleteBtn)
            card.addView(row)

            card.setOnClickListener {
                if (isActive && running) {
                    stopVpn()
                } else {
                    if (running) stopVpn()
                    connectConfig(url, key)
                }
            }

            deleteBtn.setOnClickListener {
                AlertDialog.Builder(this)
                    .setTitle("Remove Config")
                    .setMessage(if (isActive && running)
                        "\"$displayLabel\" is currently connected. Disconnect and remove it?"
                    else
                        "Remove \"$displayLabel\"?")
                    .setPositiveButton("Remove") { _, _ ->
                        if (isActive && running) stopVpn()
                        deleteConfig(url, key)
                        refreshList(running = Mobile.isRunning())
                    }
                    .setNegativeButton("Cancel", null)
                    .show()
            }

            binding.configList.addView(card)
        }
    }

    private fun connectConfig(url: String, key: String) {
        activeUrl = url
        activeKey = key
        prefs.edit().putString("url", url).putString("key", key).apply()
        refreshList(running = false)
        val vpnIntent = VpnService.prepare(this)
        if (vpnIntent != null) vpnPermissionLauncher.launch(vpnIntent) else launchVpnService()
    }

    private fun launchVpnService() {
        val url = prefs.getString("url", "") ?: ""
        val key = prefs.getString("key", "") ?: ""
        updateUI(running = true)
        ContextCompat.startForegroundService(this, Intent(this, RelayVpnService::class.java).apply {
            action = RelayVpnService.ACTION_START
            putExtra(RelayVpnService.EXTRA_URL, url)
            putExtra(RelayVpnService.EXTRA_KEY, key)
        })
    }

    private fun stopVpn() {
        startService(Intent(this, RelayVpnService::class.java).apply { action = RelayVpnService.ACTION_STOP })
        activeUrl = null
        activeKey = null
        updateUI(running = false)
    }

    private fun importConfig() {
        val clipboard = getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
        val text = clipboard.primaryClip?.getItemAt(0)?.getText()?.toString()?.trim()
        if (text.isNullOrEmpty()) {
            Toast.makeText(this, "Clipboard is empty", Toast.LENGTH_SHORT).show()
            return
        }
        try {
            val json = JSONObject(text)
            val url = json.getString("url").replace(Regex("[\\s]"), "")
            val key = json.getString("key").trim()
            if (url.isEmpty() || key.isEmpty()) throw JSONException("empty fields")
            if (saveConfig(url, key)) {
                refreshList()
                Toast.makeText(this, "Config saved — tap it to connect", Toast.LENGTH_SHORT).show()
            } else {
                Toast.makeText(this, "Already in list", Toast.LENGTH_SHORT).show()
            }
        } catch (e: JSONException) {
            Toast.makeText(this, "Invalid config — copy the JSON from ./zyrln -export-config", Toast.LENGTH_LONG).show()
        }
    }

    private fun loadConfigs(): List<Pair<String, String>> {
        val raw = prefs.getString("configs", "[]") ?: "[]"
        return try {
            val arr = JSONArray(raw)
            (0 until arr.length()).map { i ->
                val o = arr.getJSONObject(i)
                Pair(o.getString("url"), o.getString("key"))
            }
        } catch (e: JSONException) { emptyList() }
    }

    private fun saveConfig(url: String, key: String): Boolean {
        val existing = loadConfigs()
        if (existing.any { it.first == url && it.second == key }) return false
        val arr = JSONArray()
        existing.forEach { (u, k) -> arr.put(JSONObject().put("url", u).put("key", k)) }
        arr.put(JSONObject().put("url", url).put("key", key))
        prefs.edit().putString("configs", arr.toString()).apply()
        return true
    }

    private fun deleteConfig(url: String, key: String) {
        val arr = JSONArray()
        loadConfigs().filter { it.first != url || it.second != key }
            .forEach { (u, k) -> arr.put(JSONObject().put("url", u).put("key", k)) }
        prefs.edit().putString("configs", arr.toString()).apply()
    }

    private fun configLabel(url: String): String {
        val first = url.split(",").firstOrNull()?.trim() ?: return url
        val id = first.substringAfter("/macros/s/", "").substringBefore("/")
        if (id.length >= 6) return wordLabel(id)
        return first.substringAfter("://").substringBefore("/").removePrefix("www.")
    }

    private fun wordLabel(seed: String): String {
        val adj = listOf("swift","bold","quiet","bright","pure","sharp","calm","free")
        val noun = listOf("relay","bridge","tunnel","gate","link","path","pass","line")
        var h = 0L
        for (c in seed) h = h * 31 + c.code
        val ai = ((h % adj.size) + adj.size).toInt() % adj.size
        val ni = ((h / adj.size % noun.size) + noun.size).toInt() % noun.size
        return "${adj[ai]} ${noun[ni]}"
    }

    private fun installCACert() {
        val certDir = File(filesDir, "certs")
        certDir.mkdirs()
        val certFile = File(certDir, "ca.pem")
        val keyFile = File(certDir, "ca.key")

        if (!certFile.exists()) {
            val err = Mobile.generateCA(certFile.absolutePath, keyFile.absolutePath)
            if (err.isNotEmpty()) {
                Toast.makeText(this, "CA generation failed: $err", Toast.LENGTH_LONG).show()
                return
            }
        }

        try {
            val downloads = Environment.getExternalStoragePublicDirectory(Environment.DIRECTORY_DOWNLOADS)
            downloads.mkdirs()
            certFile.copyTo(File(downloads, "zyrln-ca.pem"), overwrite = true)
        } catch (e: Exception) {
            Log.w("MainActivity", "copy to Downloads failed: ${e.message}")
        }

        AlertDialog.Builder(this)
            .setTitle("Install CA Certificate")
            .setMessage(
                "The certificate has been saved to:\n\nDownloads/zyrln-ca.pem\n\n" +
                "Steps:\n" +
                "1. Tap \"Open Settings\" below\n" +
                "2. Go to Biometrics & security\n" +
                "3. Tap \"Other security settings\"\n" +
                "4. Tap \"Install from device storage\"\n" +
                "5. Browse to Downloads folder\n" +
                "6. Select zyrln-ca.pem\n" +
                "7. Choose \"CA certificate\"\n\n" +
                "Do this once — HTTPS sites will then work through the relay."
            )
            .setPositiveButton("Open Settings") { _, _ ->
                startActivity(Intent(Settings.ACTION_SECURITY_SETTINGS))
            }
            .setNegativeButton("Later", null)
            .show()
    }
}
