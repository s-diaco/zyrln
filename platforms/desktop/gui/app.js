document.addEventListener('DOMContentLoaded', () => {
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
            showLog('Configuration loaded from config.env');
            validateInputs();
        } catch (err) {
            showLog(`Error loading config: ${err.message}`, 'error');
        }
    }

    configForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showProgress('Saving configuration...');
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
                showToast('Configuration saved');
                showLog('Configuration updated.');
                validateInputs();
            } else {
                const errText = await response.text();
                showLog(`Save failed: ${errText}`, 'error');
                showToast('Save failed', 'error');
            }
        } catch (err) {
            showToast('Error saving configuration', 'error');
            showLog(`Save failed: ${err.message}`, 'error');
        } finally {
            hideProgress();
        }
    });

    // Proxy Control
    toggleProxyBtn.addEventListener('click', async () => {
        const isRunning = window.__ZYRLN_STATE__.running;
        if (!isRunning && !hasSavedConfig()) {
            showLog('Cannot start proxy: saved relay endpoint, auth key, and listen address are required.', 'error');
            showToast('Saved configuration incomplete', 'error');
            validateInputs();
            return;
        }
        const endpoint = isRunning ? '/api/stop' : '/api/start';

        showProgress(isRunning ? 'Stopping proxy...' : 'Starting proxy...');
        showLog(isRunning ? 'Stopping proxy...' : 'Starting proxy...');
        try {
            const response = await fetch(endpoint, { method: 'POST' });
            if (response.ok) {
                showLog(isRunning ? '✅ Proxy stopped.' : '✅ Proxy started on ' + (document.getElementById('listen').value || '127.0.0.1:8085'));
                showToast(isRunning ? 'Proxy stopped' : 'Proxy started');
                updateStatus();
                validateInputs();
            } else {
                const errText = await response.text();
                showLog(`❌ Failed: ${errText}`, 'error');
                showToast('Action failed: ' + errText, 'error');
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
        showProgress('Running connection diagnostics...');
        showLog('Running connection diagnostics...');

        try {
            const response = await fetch('/api/probes');
            const results = await response.json();
            results.forEach(r => {
                const status = r.ok ? 'OK' : 'FAIL';
                showLog(`${r.probe.name}: ${status} (${r.duration_ms}ms)`, r.ok ? 'info' : 'error');
            });
            showLog('Diagnostics complete.');
        } catch (err) {
            showLog('Diagnostics failed: ' + err.message, 'error');
        } finally {
            window.__ZYRLN_STATE__.probesRunning = false;
            hideProgress();
        }
    });

    exportMobileBtn.addEventListener('click', async () => {
        showProgress('Generating export data...');
        try {
            const response = await fetch('/api/export');
            const config = await response.json();
            const jsonStr = JSON.stringify(config, null, 2);

            openModal('Mobile Sync', jsonStr);
            showLog('Exported mobile configuration to modal.');
        } catch (err) {
            showToast('Export failed', 'error');
        } finally {
            hideProgress();
        }
    });

    btnRegenCA.addEventListener('click', async () => {
        showLog('Opening Save dialog...', 'info');
        
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
                        showLog('Save cancelled.', 'info');
                        return;
                    }
                    console.error('Picker failed:', err);
                }
            }

            showProgress('Generating certificate...');
            // 2. Regenerate on backend
            const response = await fetch('/api/init-ca', { method: 'POST' });
            if (!response.ok) throw new Error('Generation failed');
            
            showLog('✅ Certificate regenerated. Writing to file...', 'info');
            
            // 3. Download the data
            const certRes = await fetch('/api/download-ca');
            const blob = await certRes.blob();
            
            // 4. Save to handle OR fallback
            if (handle) {
                const writable = await handle.createWritable();
                await writable.write(blob);
                await writable.close();
                showLog('✅ Certificate saved successfully.', 'info');
            } else {
                // Fallback for non-supported browsers
                const url = window.URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.href = url;
                a.download = 'zyrln-ca.pem';
                a.click();
                window.URL.revokeObjectURL(url);
                showLog('✅ Certificate downloaded to your standard folder.', 'info');
            }
            
            showToast('Certificate Ready');
            updateStatus();
        } catch (err) {
            console.error('Action Error:', err);
            showLog(`❌ Error: ${err.message}`, 'error');
            showToast('Action failed');
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
        showToast('Copied to clipboard!');
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

    async function updateStatus() {
        try {
            const response = await fetch('/api/status');
            if (!response.ok) throw new Error(await response.text());
            const status = await response.json();
            window.__ZYRLN_STATE__.running = status.running;

            if (status.running) {
                proxyStatusIndicator.classList.add('online');
                proxyStatusText.textContent = 'Running';
                toggleProxyBtn.textContent = 'Disconnect';
                toggleProxyBtn.classList.add('danger');
            } else {
                proxyStatusIndicator.classList.remove('online');
                proxyStatusText.textContent = 'Disconnected';
                toggleProxyBtn.textContent = 'Connect';
                toggleProxyBtn.classList.remove('danger');
            }

            document.getElementById('uptimeValue').textContent = status.uptime || '00:00:00';
            document.getElementById('requestCount').textContent = status.requests || '0';
            
            // Ensure inputs are disabled/enabled based on running state
            validateInputs();
        } catch (err) {
            proxyStatusIndicator.classList.remove('online');
            proxyStatusText.textContent = 'Status unavailable';
            setButtonDisabled(toggleProxyBtn, true);
        }
    }

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
            logs.forEach(l => showLog(l.msg, l.type, l.time));
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
    ['fronted-appscript-url', 'auth-key', 'listen'].forEach(id => {
        const el = document.getElementById(id);
        if (el) el.addEventListener('input', validateInputs);
    });

    loadConfig().then(validateInputs);
    setInterval(updateStatus, 2000);
    updateStatus();
});
