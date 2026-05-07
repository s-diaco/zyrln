document.addEventListener('DOMContentLoaded', () => {
    // State management
    window.__ZYRLN_STATE__ = {
        running: false,
        uptime: '00:00:00',
        logs: [],
        probesRunning: false
    };

    // DOM Elements
    const configForm = document.getElementById('configForm');
    const toggleProxyBtn = document.getElementById('toggleProxyBtn');
    const proxyStatusIndicator = document.getElementById('proxyStatusIndicator');
    const proxyStatusText = document.getElementById('proxyStatusText');
    const logOutput = document.getElementById('logOutput');
    const clearLogsBtn = document.getElementById('clearLogsBtn');
    const initCABtn = document.getElementById('initCABtn');
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
            for (const [key, value] of Object.entries(config)) {
                const input = document.getElementById(key);
                if (input) input.value = value;
            }
            showLog('Configuration loaded from config.env');
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
                showToast('Configuration saved');
                showLog('Configuration updated.');
            }
        } catch (err) {
            showToast('Error saving configuration', 'error');
        } finally {
            hideProgress();
        }
    });

    // Proxy Control
    toggleProxyBtn.addEventListener('click', async () => {
        const isRunning = window.__ZYRLN_STATE__.running;
        const endpoint = isRunning ? '/api/stop' : '/api/start';
        
        showProgress(isRunning ? 'Stopping proxy...' : 'Starting proxy...');
        showLog(isRunning ? 'Stopping proxy...' : 'Starting proxy...');
        try {
            const response = await fetch(endpoint, { method: 'POST' });
            if (response.ok) {
                showLog(isRunning ? '✅ Proxy stopped.' : '✅ Proxy started on ' + (document.getElementById('listen').value || '127.0.0.1:8085'));
                showToast(isRunning ? 'Proxy stopped' : 'Proxy started');
                updateStatus();
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

    document.getElementById('downloadCaBtn').addEventListener('click', () => {
        window.location.href = '/api/download-ca';
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

    initCABtn.addEventListener('click', async () => {
        showProgress('Generating CA Certificate...');
        try {
            const response = await fetch('/api/init-ca', { method: 'POST' });
            if (response.ok) {
                const data = await response.json();
                showLog(data.message || 'CA generated.');
                showToast(data.message || 'CA Generated — import certs/zyrln-ca.pem into your browser');
                updateStatus(); // Reflect that proxy was stopped
            } else {
                const errText = await response.text();
                showLog('CA Init failed: ' + errText, 'error');
                showToast('CA Init failed: ' + errText, 'error');
            }
        } catch (err) {
            showLog('CA Init error: ' + err.message, 'error');
            showToast('CA Init failed', 'error');
        } finally {
            hideProgress();
        }
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

    toggleAuthVisible.onclick = () => {
        const type = authKeyInput.type === 'password' ? 'text' : 'password';
        authKeyInput.type = type;
        toggleAuthVisible.textContent = type === 'password' ? '👁️' : '🔒';
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
        } catch (err) {}
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

    loadConfig();
    setInterval(updateStatus, 2000);
    updateStatus();
});
