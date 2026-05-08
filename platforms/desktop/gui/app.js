document.addEventListener('DOMContentLoaded', () => {
    const translations = {
        en: {
            'status.disconnected': 'Disconnected',
            'app.title': 'Zyrln Control Center',
            'brand.name': 'ZYRLN',
            'status.running': 'Running',
            'status.unavailable': 'Status unavailable',
            'button.connect': 'Connect',
            'button.disconnect': 'Disconnect',
            'section.config': 'Configuration',
            'label.endpoints': 'Relay Endpoints (Apps Script)',
            'help.endpoints': 'One or more Google Apps Script URLs. Commas allow automatic failover if one fails.',
            'hint.endpoints': 'Separate multiple URLs with commas.',
            'label.authKey': 'Auth Key',
            'help.authKey': 'Secret key used for authenticating with your Apps Script relay. Must match across all components.',
            'label.listen': 'HTTP/HTTPS Listen Address',
            'help.listen': 'The local IP and port where Zyrln will listen for connections (default 127.0.0.1:8085).',
            'label.socksListen': 'SOCKS5 Address',
            'help.socksListen': 'The local IP and port where the SOCKS5 listener accepts connections (default 127.0.0.1:1080).',
            'button.save': 'Save Changes',
            'section.tools': 'Tools & Diagnostics',
            'tool.diagnostics': 'Diagnostics',
            'tool.diagnosticsDesc': 'Verify relay chain reachability',
            'button.runTest': 'Run Test',
            'tool.android': 'Android Sync',
            'tool.androidDesc': 'Export for mobile app',
            'button.export': 'Export',
            'tool.security': 'Security',
            'tool.securityDesc': 'Manage CA certificates',
            'button.installCert': 'Install Certificate',
            'help.cert': 'Generate and download the CA certificate',
            'stat.uptime': 'Uptime',
            'stat.requests': 'Requests',
            'section.monitor': 'Live Monitor',
            'placeholder.filter': 'Filter logs...',
            'button.clearLogs': 'Clear Logs',
            'log.initial': 'System initialized. Ready for connection.',
            'modal.exportTitle': 'Export Configuration',
            'modal.exportDesc': 'Copy this JSON into your Android app:',
            'modal.mobileSync': 'Mobile Sync',
            'button.copyClose': 'Copy & Close',
            'progress.processing': 'Processing...',
            'progress.saving': 'Saving configuration...',
            'progress.starting': 'Starting HTTP and SOCKS5 listeners...',
            'progress.stopping': 'Stopping proxy...',
            'progress.diagnostics': 'Running connection diagnostics...',
            'progress.export': 'Generating export data...',
            'progress.cert': 'Generating certificate...',
            'toast.configSaved': 'Configuration saved',
            'toast.saveFailed': 'Save failed',
            'toast.saveError': 'Error saving configuration',
            'toast.configIncomplete': 'Saved configuration incomplete',
            'toast.proxyStarted': 'Proxy started',
            'toast.proxyStopped': 'Proxy stopped',
            'toast.actionFailed': 'Action failed',
            'toast.exportFailed': 'Export failed',
            'toast.certReady': 'Certificate Ready',
            'toast.copied': 'Copied to clipboard!',
            'log.configLoaded': 'Configuration loaded from config.env',
            'log.configUpdated': 'Configuration updated.',
            'log.cannotStart': 'Cannot start proxy: saved relay endpoint, auth key, and listen addresses are required.',
            'log.proxyStarted': 'Proxy started: HTTP on {listen}, SOCKS5 on {socksListen}',
            'log.proxyStopped': 'Proxy stopped.',
            'log.diagnosticsComplete': 'Diagnostics complete.',
            'log.diagnosticsFailed': 'Diagnostics failed: {error}',
            'log.exported': 'Exported mobile configuration to modal.',
            'log.openSave': 'Opening Save dialog...',
            'log.saveCancelled': 'Save cancelled.',
            'log.certRegenerated': 'Certificate regenerated. Writing to file...',
            'log.certSaved': 'Certificate saved successfully.',
            'log.certDownloaded': 'Certificate downloaded to your standard folder.'
        },
        fa: {
            'status.disconnected': 'قطع است',
            'app.title': 'مرکز کنترل زیرلن',
            'brand.name': 'زیرلن',
            'status.running': 'در حال اجرا',
            'status.unavailable': 'وضعیت در دسترس نیست',
            'button.connect': 'اتصال',
            'button.disconnect': 'قطع اتصال',
            'section.config': 'پیکربندی',
            'label.endpoints': 'آدرس‌های رله (Apps Script)',
            'help.endpoints': 'یک یا چند آدرس Google Apps Script. چند آدرس را با کاما جدا کنید.',
            'hint.endpoints': 'برای چند آدرس، آن‌ها را با کاما جدا کنید.',
            'label.authKey': 'کلید احراز هویت',
            'help.authKey': 'کلید محرمانه برای اتصال به رله Apps Script. باید در همه بخش‌ها یکسان باشد.',
            'label.listen': 'آدرس HTTP/HTTPS',
            'help.listen': 'آی‌پی و پورت محلی که زیرلن روی آن اتصال‌ها را می‌پذیرد. پیش‌فرض 127.0.0.1:8085 است.',
            'label.socksListen': 'آدرس SOCKS5',
            'help.socksListen': 'آی‌پی و پورت محلی که شنونده SOCKS5 روی آن اتصال‌ها را می‌پذیرد. پیش‌فرض 127.0.0.1:1080 است.',
            'button.save': 'ذخیره تغییرات',
            'section.tools': 'ابزارها و عیب‌یابی',
            'tool.diagnostics': 'عیب‌یابی',
            'tool.diagnosticsDesc': 'بررسی دسترسی زنجیره رله',
            'button.runTest': 'اجرای تست',
            'tool.android': 'همگام‌سازی اندروید',
            'tool.androidDesc': 'خروجی برای اپ موبایل',
            'button.export': 'خروجی',
            'tool.security': 'امنیت',
            'tool.securityDesc': 'مدیریت گواهینامه CA',
            'button.installCert': 'نصب گواهینامه',
            'help.cert': 'ساخت و دانلود گواهینامه CA',
            'stat.uptime': 'زمان اجرا',
            'stat.requests': 'درخواست‌ها',
            'section.monitor': 'مانیتور زنده',
            'placeholder.filter': '...فیلتر لاگ‌ها',
            'button.clearLogs': 'پاک کردن لاگ‌ها',
            'modal.exportTitle': 'خروجی پیکربندی',
            'modal.exportDesc': 'این JSON را در اپ اندروید وارد کنید:',
            'modal.mobileSync': 'همگام‌سازی موبایل',
            'button.copyClose': 'کپی و بستن',
            'progress.processing': 'در حال پردازش...',
            'progress.saving': 'در حال ذخیره پیکربندی...',
            'progress.starting': 'در حال شروع شنونده‌های HTTP و SOCKS5...',
            'progress.stopping': 'در حال توقف پروکسی...',
            'progress.diagnostics': 'در حال اجرای عیب‌یابی اتصال...',
            'progress.export': 'در حال ساخت خروجی...',
            'progress.cert': 'در حال ساخت گواهینامه...',
            'toast.configSaved': 'پیکربندی ذخیره شد',
            'toast.saveFailed': 'ذخیره ناموفق بود',
            'toast.saveError': 'خطا در ذخیره پیکربندی',
            'toast.configIncomplete': 'پیکربندی ذخیره‌شده کامل نیست',
            'toast.proxyStarted': 'پروکسی شروع شد',
            'toast.proxyStopped': 'پروکسی متوقف شد',
            'toast.actionFailed': 'عملیات ناموفق بود',
            'toast.exportFailed': 'خروجی ناموفق بود',
            'toast.certReady': 'گواهینامه آماده است',
            'toast.copied': 'در کلیپ‌بورد کپی شد',
        }
    };

    let currentLang = localStorage.getItem('zyrln_lang') || 'en';
    const t = (key, params = {}) => {
        const value = translations[currentLang][key] || translations.en[key] || key;
        return Object.entries(params).reduce((text, [name, replacement]) => (
            text.replaceAll(`{${name}}`, replacement)
        ), value);
    };
    const tLog = (key, params = {}) => {
        const value = translations.en[key] || key;
        return Object.entries(params).reduce((text, [name, replacement]) => (
            text.replaceAll(`{${name}}`, replacement)
        ), value);
    };
    const legacyPersianLogs = new Map([
        ['پیکربندی از config.env خوانده شد', translations.en['log.configLoaded']],
        ['پیکربندی به‌روزرسانی شد.', translations.en['log.configUpdated']],
        ['در حال توقف پروکسی...', translations.en['progress.stopping']],
        ['در حال شروع پروکسی...', translations.en['progress.starting']],
        ['در حال شروع شنونده‌های HTTP و SOCKS5...', translations.en['progress.starting']],
        ['در حال اجرای عیب‌یابی اتصال...', translations.en['progress.diagnostics']],
        ['عیب‌یابی کامل شد.', translations.en['log.diagnosticsComplete']],
        ['پیکربندی موبایل در پنجره خروجی نمایش داده شد.', translations.en['log.exported']],
        ['در حال باز کردن پنجره ذخیره...', translations.en['log.openSave']],
        ['ذخیره لغو شد.', translations.en['log.saveCancelled']],
        ['گواهینامه دوباره ساخته شد. در حال نوشتن فایل...', translations.en['log.certRegenerated']],
        ['گواهینامه با موفقیت ذخیره شد.', translations.en['log.certSaved']],
        ['گواهینامه در پوشه دانلود پیش‌فرض ذخیره شد.', translations.en['log.certDownloaded']],
    ]);
    const normalizeLogMessage = (msg) => legacyPersianLogs.get(msg) || msg;

    // State management
    window.__ZYRLN_STATE__ = {
        running: false,
        uptime: '00:00:00',
        logs: [],
        probesRunning: false,
        lastSavedConfig: {}
    };

    // DOM Elements
    const configForm = document.getElementById('configForm');
    const toggleProxyBtn = document.getElementById('toggleProxyBtn');
    const proxyStatusIndicator = document.getElementById('proxyStatusIndicator');
    const proxyStatusText = document.getElementById('proxyStatusText');
    const logOutput = document.getElementById('logOutput');
    const clearLogsBtn = document.getElementById('clearLogsBtn');
    const btnRegenCA = document.getElementById('btnRegenCA');
    const runProbesBtn = document.getElementById('runProbesBtn');
    const exportMobileBtn = document.getElementById('exportMobileBtn');
    const logFilter = document.getElementById('logFilter');
    const toggleAuthVisible = document.getElementById('toggleAuthVisible');
    const authKeyInput = document.getElementById('auth-key');
    const languageToggleBtn = document.getElementById('languageToggleBtn');
    const themeToggleBtn = document.getElementById('themeToggleBtn');

    // UI Feedback Elements
    const progressIndicator = document.getElementById('progressIndicator');
    const progressText = document.getElementById('progressText');

    // Modal Elements
    const modalOverlay = document.getElementById('modalOverlay');
    const modalTextArea = document.getElementById('modalTextArea');
    const modalActionBtn = document.getElementById('modalActionBtn');
    const closeModal = document.getElementById('closeModal');

    // Config Management
    async function loadConfig() {
        try {
            const response = await fetch('/api/config');
            const config = await response.json();
            window.__ZYRLN_STATE__.lastSavedConfig = config;
            for (const [key, value] of Object.entries(config)) {
                const input = document.getElementById(key);
                if (input) input.value = value;
            }
            showLog(tLog('log.configLoaded'));
            validateInputs();
        } catch (err) {
            showLog(`Error loading config: ${err.message}`, 'error');
        }
    }

    configForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showProgress(t('progress.saving'));
        const formData = new FormData(configForm);
        const data = Object.fromEntries(formData.entries());

        try {
            const response = await fetch('/api/config', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data)
            });

            if (response.ok) {
                window.__ZYRLN_STATE__.lastSavedConfig = data;
                showToast(t('toast.configSaved'));
                showLog(tLog('log.configUpdated'));
                validateInputs();
            } else {
                const errText = await response.text();
                showLog(`Save failed: ${errText}`, 'error');
                showToast(t('toast.saveFailed'), 'error');
            }
        } catch (err) {
            showToast(t('toast.saveError'), 'error');
            showLog(`Save failed: ${err.message}`, 'error');
        } finally {
            hideProgress();
        }
    });

    // Proxy Control
    toggleProxyBtn.addEventListener('click', async () => {
        const isRunning = window.__ZYRLN_STATE__.running;
        if (!isRunning && !hasSavedConfig()) {
            showLog(tLog('log.cannotStart'), 'error');
            showToast(t('toast.configIncomplete'), 'error');
            validateInputs();
            return;
        }
        const endpoint = isRunning ? '/api/stop' : '/api/start';

        showProgress(isRunning ? t('progress.stopping') : t('progress.starting'));
        showLog(isRunning ? tLog('progress.stopping') : tLog('progress.starting'));
        try {
            const response = await fetch(endpoint, { method: 'POST' });
            if (response.ok) {
                showLog(isRunning ? `✅ ${tLog('log.proxyStopped')}` : `✅ ${tLog('log.proxyStarted', {
                    listen: document.getElementById('listen').value || '127.0.0.1:8085',
                    socksListen: document.getElementById('socks-listen').value || '127.0.0.1:1080'
                })}`);
                showToast(isRunning ? t('toast.proxyStopped') : t('toast.proxyStarted'));
                updateStatus();
                validateInputs();
            } else {
                const errText = await response.text();
                showLog(`❌ Failed: ${errText}`, 'error');
                showToast(`${t('toast.actionFailed')}: ${errText}`, 'error');
            }
        } catch (err) {
            showLog(`❌ Connection error: ${err.message}`, 'error');
        } finally {
            setTimeout(hideProgress, 500);
        }
    });

    // Tools & Actions
    runProbesBtn.addEventListener('click', async () => {
        if (window.__ZYRLN_STATE__.probesRunning) return;

        window.__ZYRLN_STATE__.probesRunning = true;
        showProgress(t('progress.diagnostics'));
        showLog(tLog('progress.diagnostics'));

        try {
            const response = await fetch('/api/probes');
            const results = await response.json();
            results.forEach(r => {
                const status = r.ok ? 'OK' : 'FAIL';
                showLog(`${r.probe.name}: ${status} (${r.duration_ms}ms)`, r.ok ? 'info' : 'error');
            });
            showLog(tLog('log.diagnosticsComplete'));
        } catch (err) {
            showLog(tLog('log.diagnosticsFailed', { error: err.message }), 'error');
        } finally {
            window.__ZYRLN_STATE__.probesRunning = false;
            hideProgress();
        }
    });

    exportMobileBtn.addEventListener('click', async () => {
        showProgress(t('progress.export'));
        try {
            const response = await fetch('/api/export');
            const config = await response.json();
            const jsonStr = JSON.stringify(config, null, 2);

            openModal(t('modal.mobileSync'), jsonStr);
            showLog(tLog('log.exported'));
        } catch (err) {
            showToast(t('toast.exportFailed'), 'error');
        } finally {
            hideProgress();
        }
    });

    btnRegenCA.addEventListener('click', async () => {
        showLog(tLog('log.openSave'), 'info');
        
        try {
            let handle = null;
            // 1. Try to get a save handle immediately (User Activation)
            if ('showSaveFilePicker' in window) {
                try {
                    handle = await window.showSaveFilePicker({
                        suggestedName: 'zyrln-ca.pem',
                        types: [{
                            description: 'PEM Certificate',
                            accept: { 'application/x-x509-ca-cert': ['.pem'] },
                        }],
                    });
                } catch (err) {
                    if (err.name === 'AbortError') {
                        showLog(tLog('log.saveCancelled'), 'info');
                        return;
                    }
                    console.error('Picker failed:', err);
                }
            }

            showProgress(t('progress.cert'));
            // 2. Regenerate on backend
            const response = await fetch('/api/init-ca', { method: 'POST' });
            if (!response.ok) throw new Error('Generation failed');
            
            showLog(`✅ ${tLog('log.certRegenerated')}`, 'info');
            
            // 3. Download the data
            const certRes = await fetch('/api/download-ca');
            const blob = await certRes.blob();
            
            // 4. Save to handle OR fallback
            if (handle) {
                const writable = await handle.createWritable();
                await writable.write(blob);
                await writable.close();
                showLog(`✅ ${tLog('log.certSaved')}`, 'info');
            } else {
                // Fallback for non-supported browsers
                const url = window.URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.href = url;
                a.download = 'zyrln-ca.pem';
                a.click();
                window.URL.revokeObjectURL(url);
                showLog(`✅ ${tLog('log.certDownloaded')}`, 'info');
            }
            
            showToast(t('toast.certReady'));
            updateStatus();
        } catch (err) {
            console.error('Action Error:', err);
            showLog(`❌ Error: ${err.message}`, 'error');
            showToast(t('toast.actionFailed'));
        } finally {
            hideProgress();
        }
    });

    // Log Filtering
    logFilter.addEventListener('input', (e) => {
        const term = e.target.value.toLowerCase();
        const entries = logOutput.querySelectorAll('.log-entry');
        entries.forEach(entry => {
            const text = entry.textContent.toLowerCase();
            entry.style.display = text.includes(term) ? 'block' : 'none';
        });
    });

    // Modal Logic
    function openModal(title, content) {
        document.getElementById('modalTitle').textContent = title;
        modalTextArea.value = content;
        modalOverlay.classList.add('active');
    }

    closeModal.onclick = () => modalOverlay.classList.remove('active');
    window.onclick = (e) => { if (e.target == modalOverlay) modalOverlay.classList.remove('active'); };

    modalActionBtn.onclick = async () => {
        await navigator.clipboard.writeText(modalTextArea.value);
        showToast(t('toast.copied'));
        modalOverlay.classList.remove('active');
    };

    // UI Helpers
    function showProgress(text) {
        progressText.textContent = text;
        progressIndicator.classList.add('active');
    }

    function hideProgress() {
        progressIndicator.classList.remove('active');
    }

    const eyeIcon = `
        <svg viewBox="0 0 24 24" width="18" height="18" stroke="currentColor" stroke-width="2" fill="none" stroke-linecap="round" stroke-linejoin="round">
            <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path>
            <circle cx="12" cy="12" r="3"></circle>
        </svg>`;
    const eyeOffIcon = `
        <svg viewBox="0 0 24 24" width="18" height="18" stroke="currentColor" stroke-width="2" fill="none" stroke-linecap="round" stroke-linejoin="round">
            <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"></path>
            <line x1="1" y1="1" x2="23" y2="23"></line>
        </svg>`;

    toggleAuthVisible.innerHTML = eyeIcon;
    toggleAuthVisible.onclick = () => {
        const isPassword = authKeyInput.type === 'password';
        authKeyInput.type = isPassword ? 'text' : 'password';
        toggleAuthVisible.innerHTML = isPassword ? eyeOffIcon : eyeIcon;
    };

    function showLog(msg, type = 'info', savedTime = null) {
        msg = normalizeLogMessage(msg);
        const entry = document.createElement('div');
        entry.className = `log-entry ${type}`;
        const time = savedTime || new Date().toLocaleTimeString([], { hour12: false });
        entry.textContent = `[${time}] ${msg}`;
        entry.dataset.time = time;
        entry.dataset.msg = msg;
        entry.dataset.type = type;

        // Save to state and local storage
        window.__ZYRLN_STATE__.logs.push({ time, msg, type });
        if (window.__ZYRLN_STATE__.logs.length > 500) window.__ZYRLN_STATE__.logs.shift();
        localStorage.setItem('zyrln_logs', JSON.stringify(window.__ZYRLN_STATE__.logs));

        // Apply current filter
        const term = logFilter.value.toLowerCase();
        if (term && !entry.textContent.toLowerCase().includes(term)) {
            entry.style.display = 'none';
        }

        logOutput.appendChild(entry);
        logOutput.scrollTop = logOutput.scrollHeight;
    }

    function showToast(msg) {
        const container = document.getElementById('toastContainer');
        const toast = document.createElement('div');
        toast.className = 'toast';
        toast.textContent = msg;
        container.appendChild(toast);
        setTimeout(() => {
            toast.style.opacity = '0';
            setTimeout(() => toast.remove(), 300);
        }, 3000);
    }

    function applyLocalization() {
        document.documentElement.lang = currentLang;
        document.documentElement.dir = currentLang === 'fa' ? 'rtl' : 'ltr';
        document.title = t('app.title');
        document.body.classList.toggle('rtl', currentLang === 'fa');
        languageToggleBtn.textContent = currentLang === 'fa' ? 'EN' : 'FA';

        document.querySelectorAll('[data-i18n]').forEach(el => {
            el.textContent = t(el.dataset.i18n);
        });
        document.querySelectorAll('[data-i18n-title]').forEach(el => {
            el.title = t(el.dataset.i18nTitle);
        });
        document.querySelectorAll('[data-i18n-placeholder]').forEach(el => {
            el.placeholder = t(el.dataset.i18nPlaceholder);
        });
        updateStatusText();
    }

    function updateStatusText() {
        if (window.__ZYRLN_STATE__.statusUnavailable) {
            proxyStatusText.textContent = t('status.unavailable');
            return;
        }
        if (window.__ZYRLN_STATE__.running) {
            proxyStatusText.textContent = t('status.running');
            toggleProxyBtn.textContent = t('button.disconnect');
        } else {
            proxyStatusText.textContent = t('status.disconnected');
            toggleProxyBtn.textContent = t('button.connect');
        }
    }

    async function updateStatus() {
        try {
            const response = await fetch('/api/status');
            if (!response.ok) throw new Error(await response.text());
            const status = await response.json();
            window.__ZYRLN_STATE__.running = status.running;
            window.__ZYRLN_STATE__.statusUnavailable = false;

            if (status.running) {
                proxyStatusIndicator.classList.add('online');
                toggleProxyBtn.classList.add('danger');
            } else {
                proxyStatusIndicator.classList.remove('online');
                toggleProxyBtn.classList.remove('danger');
            }
            updateStatusText();

            document.getElementById('uptimeValue').textContent = status.uptime || '00:00:00';
            document.getElementById('requestCount').textContent = status.requests || '0';
            
            // Ensure inputs are disabled/enabled based on running state
            validateInputs();
        } catch (err) {
            window.__ZYRLN_STATE__.statusUnavailable = true;
            proxyStatusIndicator.classList.remove('online');
            proxyStatusText.textContent = t('status.unavailable');
            setButtonDisabled(toggleProxyBtn, true);
        }
    }

    languageToggleBtn.onclick = () => {
        currentLang = currentLang === 'fa' ? 'en' : 'fa';
        localStorage.setItem('zyrln_lang', currentLang);
        applyLocalization();
    };

    function applyTheme() {
        const theme = localStorage.getItem('zyrln_theme') || 'dark';
        document.body.classList.toggle('light-mode', theme === 'light');
        document.body.classList.toggle('dark-mode', theme !== 'light');
        themeToggleBtn.textContent = theme === 'light' ? '☾' : '☀';
    }

    themeToggleBtn.onclick = () => {
        const currentTheme = document.body.classList.contains('light-mode') ? 'light' : 'dark';
        localStorage.setItem('zyrln_theme', currentTheme === 'light' ? 'dark' : 'light');
        applyTheme();
    };

    clearLogsBtn.onclick = () => {
        logOutput.innerHTML = '';
        window.__ZYRLN_STATE__.logs = [];
        localStorage.removeItem('zyrln_logs');
    };

    // Initialize
    const savedLogs = localStorage.getItem('zyrln_logs');
    if (savedLogs) {
        try {
            const logs = JSON.parse(savedLogs);
            logs.forEach(l => showLog(normalizeLogMessage(l.msg), l.type, l.time));
        } catch (e) { localStorage.removeItem('zyrln_logs'); }
    }

    function validateInputs() {
        const isRunning = window.__ZYRLN_STATE__.running;

        // Disable configuration section ONLY if running
        const configInputs = configForm.querySelectorAll('input, textarea, button[type="submit"]');
        configInputs.forEach(el => {
            el.disabled = isRunning;
            if (el.classList.contains('save-btn')) {
                el.style.opacity = isRunning ? '0.5' : '1';
            }
        });

        // Basic requirement: fields must not be empty
        const hasRunnableConfig = hasSavedConfig();

        setButtonDisabled(toggleProxyBtn, !isRunning && !hasRunnableConfig);

        // These actions use saved config.env, not unsaved form edits.
        [runProbesBtn, exportMobileBtn].forEach(btn => {
            if (btn) {
                setButtonDisabled(btn, !hasRunnableConfig);
            }
        });

        // Install Certificate is special — it only needs the server to be ready, but we'll keep it enabled
        if (btnRegenCA) {
            setButtonDisabled(btnRegenCA, false);
        }
    }

    function hasSavedConfig() {
        const config = window.__ZYRLN_STATE__.lastSavedConfig || {};
        const url = (config['fronted-appscript-url'] || '').trim();
        const key = (config['auth-key'] || '').trim();
        const listen = (config.listen || '127.0.0.1:8085').trim();
        return url !== '' && key !== '' && listen !== '';
    }

    function setButtonDisabled(btn, disabled) {
        btn.disabled = disabled;
        btn.setAttribute('aria-disabled', disabled ? 'true' : 'false');
        btn.style.opacity = disabled ? '0.5' : '1';
    }

    // Listen for input changes
    ['fronted-appscript-url', 'auth-key', 'listen', 'socks-listen'].forEach(id => {
        const el = document.getElementById(id);
        if (el) el.addEventListener('input', validateInputs);
    });

    applyTheme();
    applyLocalization();
    loadConfig().then(validateInputs);
    setInterval(updateStatus, 2000);
    updateStatus();
});
