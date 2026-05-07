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
import android.widget.EditText
import android.text.InputType
import android.widget.Toast
import org.json.JSONArray
import org.json.JSONException
import org.json.JSONObject
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.appcompat.app.AppCompatDelegate
import androidx.cardview.widget.CardView
import androidx.core.content.ContextCompat
import androidx.core.os.LocaleListCompat
import com.zyrln.relay.databinding.ActivityMainBinding
import mobile.Mobile
import android.net.Uri
import java.io.File
import java.io.FileOutputStream
import java.util.Locale

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

    private val createDocumentLauncher = registerForActivityResult(
        ActivityResultContracts.CreateDocument("application/x-x509-ca-cert")
    ) { uri ->
        uri?.let { saveCertToUri(it) }
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
        binding.versionTag.text = "v${BuildConfig.VERSION_NAME}"

        binding.btnImportConfig.setOnClickListener { importConfig() }
        binding.btnInstallCA.setOnClickListener { installCACert() }
        binding.btnLanguage.setOnClickListener { toggleLanguage() }
        binding.btnTheme.setOnClickListener { toggleTheme() }

        if (Mobile.isRunning()) {
            activeUrl = prefs.getString("url", null)
            activeKey = prefs.getString("key", null)
        }
        updateUI(running = Mobile.isRunning())
        updateLanguageButton()
        updateThemeButton()

        // Apply saved theme
        val savedTheme = prefs.getInt("theme_mode", AppCompatDelegate.MODE_NIGHT_FOLLOW_SYSTEM)
        if (AppCompatDelegate.getDefaultNightMode() != savedTheme) {
            AppCompatDelegate.setDefaultNightMode(savedTheme)
        }
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

    private fun toggleLanguage() {
        val currentLocale = AppCompatDelegate.getApplicationLocales()[0]?.language ?: Locale.getDefault().language
        val newLocale = if (currentLocale == "fa") "en" else "fa"
        
        val appLocale: LocaleListCompat = LocaleListCompat.forLanguageTags(newLocale)
        AppCompatDelegate.setApplicationLocales(appLocale)
    }

    private fun toggleTheme() {
        val isNight = (resources.configuration.uiMode and android.content.res.Configuration.UI_MODE_NIGHT_MASK) == android.content.res.Configuration.UI_MODE_NIGHT_YES
        val nextMode = if (isNight) AppCompatDelegate.MODE_NIGHT_NO else AppCompatDelegate.MODE_NIGHT_YES
        AppCompatDelegate.setDefaultNightMode(nextMode)
        prefs.edit().putInt("theme_mode", nextMode).apply()
        updateThemeButton()
    }

    private fun updateLanguageButton() {
        val currentLocale = AppCompatDelegate.getApplicationLocales()[0]?.language ?: Locale.getDefault().language
        binding.btnLanguage.text = if (currentLocale == "fa") "EN" else "FA"
    }

    private fun updateThemeButton() {
        val isNight = (resources.configuration.uiMode and android.content.res.Configuration.UI_MODE_NIGHT_MASK) == android.content.res.Configuration.UI_MODE_NIGHT_YES
        binding.btnTheme.setImageResource(if (isNight) R.drawable.ic_sun else R.drawable.ic_moon)
        binding.btnTheme.imageTintList = ContextCompat.getColorStateList(this, R.color.icon)
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
        val iconButtonSize = (40 * dp).toInt()
        val iconButtonPadding = (8 * dp).toInt()

        val hostnames = configs.map { (url, _) -> configLabel(url) }

        configs.forEach { (url, key) ->
            val isActive = url == activeUrl && key == activeKey

            val card = CardView(this).apply {
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                ).apply { bottomMargin = (12 * dp).toInt() }
                radius = 12 * dp
                cardElevation = 0f
                setCardBackgroundColor(ContextCompat.getColor(this@MainActivity, R.color.surface))
            }

            val row = LinearLayout(this).apply {
                orientation = LinearLayout.HORIZONTAL
                gravity = Gravity.CENTER_VERTICAL
                layoutDirection = View.LAYOUT_DIRECTION_LTR
                val p = (16 * dp).toInt()
                setPadding(p, p, p, p)
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
                textAlignment = View.TEXT_ALIGNMENT_VIEW_START
                textDirection = View.TEXT_DIRECTION_LTR
                setTextColor(ContextCompat.getColor(this@MainActivity, R.color.title))
                if (isActive) setTypeface(null, Typeface.BOLD)
            }

            val urlList = url.split(",").map { it.trim() }.filter { it.isNotEmpty() }
            val infoBtn = android.widget.ImageButton(this).apply {
                visibility = if (urlList.size > 1) View.VISIBLE else View.GONE
                setImageDrawable(ContextCompat.getDrawable(this@MainActivity, R.drawable.ic_info))
                background = null
                imageTintList = ContextCompat.getColorStateList(this@MainActivity, R.color.icon)
                scaleType = android.widget.ImageView.ScaleType.CENTER
                setPadding(iconButtonPadding, iconButtonPadding, iconButtonPadding, iconButtonPadding)
                layoutParams = LinearLayout.LayoutParams(
                    iconButtonSize, iconButtonSize
                ).apply { marginEnd = (4 * dp).toInt() }
            }
            infoBtn.setOnClickListener {
                val lines = urlList.mapIndexed { i, u ->
                    val id = u.substringAfter("/macros/s/", "").substringBefore("/")
                    val short = if (id.length >= 6) "…${id.takeLast(10)}" else u.substringAfter("://").substringBefore("/")
                    "${i + 1}. $short"
                }.joinToString("\n")
                AlertDialog.Builder(this@MainActivity)
                    .setTitle(R.string.btn_ok) // Reusing OK as placeholder for title
                    .setMessage(lines)
                    .setPositiveButton(R.string.btn_ok, null)
                    .show()
            }

            val actionBtn = android.widget.ImageButton(this).apply {
                layoutParams = LinearLayout.LayoutParams(
                    iconButtonSize, iconButtonSize
                ).apply { marginEnd = (12 * dp).toInt() }
                setImageDrawable(ContextCompat.getDrawable(this@MainActivity,
                    if (isActive && running) R.drawable.ic_pause else R.drawable.ic_play))
                background = null
                imageTintList = ContextCompat.getColorStateList(this@MainActivity, R.color.icon)
                scaleType = android.widget.ImageView.ScaleType.CENTER
                setPadding(iconButtonPadding, iconButtonPadding, iconButtonPadding, iconButtonPadding)
                isClickable = false
                isFocusable = false
            }

            val deleteBtn = android.widget.ImageButton(this).apply {
                layoutParams = LinearLayout.LayoutParams(
                    iconButtonSize, iconButtonSize
                ).apply { marginStart = (8 * dp).toInt() }
                setImageDrawable(ContextCompat.getDrawable(this@MainActivity, R.drawable.ic_delete))
                background = null
                imageTintList = ContextCompat.getColorStateList(this@MainActivity, R.color.icon)
                scaleType = android.widget.ImageView.ScaleType.CENTER
                setPadding(iconButtonPadding, iconButtonPadding, iconButtonPadding, iconButtonPadding)
            }

            row.addView(actionBtn)
            row.addView(label)
            row.addView(infoBtn)
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
                    .setTitle(R.string.dialog_remove_title)
                    .setMessage(if (isActive && running)
                        getString(R.string.dialog_remove_active, displayLabel)
                    else
                        getString(R.string.dialog_remove_inactive, displayLabel))
                    .setPositiveButton(R.string.btn_remove) { _, _ ->
                        if (isActive && running) stopVpn()
                        deleteConfig(url, key)
                        refreshList(running = Mobile.isRunning())
                    }
                    .setNegativeButton(R.string.btn_cancel, null)
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
        val rawText = clipboard.primaryClip?.getItemAt(0)?.getText()?.toString()?.trim()
        if (rawText.isNullOrEmpty()) {
            Toast.makeText(this, R.string.msg_clipboard_empty, Toast.LENGTH_SHORT).show()
            return
        }

        try {
            val (url, key) = ConfigUtils.parseImportText(rawText)
            if (saveConfig(url, key)) {
                refreshList()
                Toast.makeText(this, R.string.msg_config_saved_connect, Toast.LENGTH_SHORT).show()
            } else {
                Toast.makeText(this, R.string.msg_already_exists, Toast.LENGTH_SHORT).show()
            }
        } catch (e: Exception) {
            Log.e("MainActivity", "Import failed: ${e.message}")
            Toast.makeText(this, getString(R.string.msg_invalid_config), Toast.LENGTH_LONG).show()
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
        return ConfigUtils.configLabel(url)
    }

    private fun installCACert() {
        val certDir = File(filesDir, "certs")
        certDir.mkdirs()
        val certFile = File(certDir, "ca.pem")
        val keyFile = File(certDir, "ca.key")
        generateAndExportCert(certFile, keyFile)
    }

    private fun generateAndExportCert(certFile: File, keyFile: File) {
        val err = Mobile.generateCA(certFile.absolutePath, keyFile.absolutePath)
        if (err.isNotEmpty()) {
            Toast.makeText(this, "CA generation failed: $err", Toast.LENGTH_LONG).show()
            return
        }
        createDocumentLauncher.launch("zyrln-ca.pem")
    }

    private fun saveCertToUri(uri: Uri) {
        val certFile = File(File(filesDir, "certs"), "ca.pem")
        try {
            contentResolver.openOutputStream(uri)?.use { output ->
                certFile.inputStream().use { input ->
                    input.copyTo(output)
                }
            }
            Toast.makeText(this, R.string.msg_cert_saved_success, Toast.LENGTH_SHORT).show()

            AlertDialog.Builder(this)
                .setTitle(R.string.dialog_ca_title)
                .setMessage(R.string.dialog_ca_reinstall_message)
                .setPositiveButton(R.string.btn_open_settings) { _, _ ->
                    startActivity(Intent(Settings.ACTION_SECURITY_SETTINGS))
                }
                .setNegativeButton(R.string.btn_later, null)
                .show()
        } catch (e: Exception) {
            Log.e("MainActivity", "Failed to save cert: ${e.message}")
            Toast.makeText(this, "Save failed: ${e.message}", Toast.LENGTH_LONG).show()
        }
    }
}
