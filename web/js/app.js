// ============================
// Docker Registry Dashboard - Frontend Application
// ============================
(function () {
    'use strict';

    const API = {
        async request(method, url, body = null) {
            const opts = { method, headers: { 'Content-Type': 'application/json' } };
            if (body) opts.body = JSON.stringify(body);
            const res = await fetch(url, opts);
            const data = await res.json();
            if (!data.success) throw new Error(data.error || 'Unknown error');
            return data;
        },
        getDashboardStats: () => API.request('GET', '/api/dashboard/stats'),
        getRegistries: () => API.request('GET', '/api/registries'),
        createRegistry: (d) => API.request('POST', '/api/registries', d),
        updateRegistry: (id, d) => API.request('PUT', `/api/registries/${id}`, d),
        deleteRegistry: (id) => API.request('DELETE', `/api/registries/${id}`),
        testRegistry: (id) => API.request('POST', `/api/registries/${id}/test`),
        getRepositories: (id) => API.request('GET', `/api/registries/${id}/repositories`),
        getTags: (id, repo) => API.request('GET', `/api/registries/${id}/tags?repo=${encodeURIComponent(repo)}`),
        getManifest: (id, repo, tag) => API.request('GET', `/api/registries/${id}/manifest?repo=${encodeURIComponent(repo)}&tag=${encodeURIComponent(tag)}`),
        deleteTag: (id, repo, tag) => API.request('DELETE', `/api/registries/${id}/tag?repo=${encodeURIComponent(repo)}&tag=${encodeURIComponent(tag)}`),
        getStorageConfig: () => API.request('GET', '/api/storage'),
        saveStorageConfig: (d) => API.request('POST', '/api/storage', d),
        testStorageConnection: (d) => API.request('POST', '/api/storage/test', d),
        getRegistryStatus: () => API.request('GET', '/api/registry/status'),
        restartRegistry: () => API.request('POST', '/api/registry/restart'),
        stopRegistry: () => API.request('POST', '/api/registry/stop'),
        startRegistry: () => API.request('POST', '/api/registry/start'),
        getRegistryLogs: () => API.request('GET', '/api/registry/logs'),
        getRetention: (id) => API.request('GET', `/api/registries/${id}/retention`),
        saveRetention: (id, d) => API.request('POST', `/api/registries/${id}/retention`, d),
        runRetention: (id, dry) => API.request('POST', `/api/registries/${id}/retention/run?dry_run=${dry}`),

        // Vulnerability Scan
        triggerScan: (data) => API.request('POST', '/api/scan/trigger', data),
        getScanResult: (regId, repo, tag) => API.request('GET', `/api/scan/result?registry_id=${regId}&repository=${repo}&tag=${tag}`),
        listScans: (id) => fetch(`/api/scan/list?registry_id=${id}`).then(r => r.json()),
        listVulnerabilities: (id) => API.request('GET', `/api/vulnerabilities/list?registry_id=${id}`),
        getScanPolicy: (id) => fetch(`/api/registries/${id}/scan-policy`).then(r => r.json()),
        saveScanPolicy: (id, data) => fetch(`/api/registries/${id}/scan-policy`, { method: 'POST', body: JSON.stringify(data) }).then(r => r.json()),
    };

    // Toast Notifications
    const Toast = {
        container: document.getElementById('toast-container'),
        show(message, type = 'info', duration = 4000) {
            const icons = {
                success: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>',
                error: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>',
                warning: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>',
                info: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg>',
            };
            const toast = document.createElement('div');
            toast.className = `toast toast-${type}`;
            toast.innerHTML = `<div class="toast-icon">${icons[type]}</div><span class="toast-message">${message}</span><button class="toast-close" onclick="this.parentElement.classList.add('removing');setTimeout(()=>this.parentElement.remove(),300)"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg></button>`;
            this.container.appendChild(toast);
            setTimeout(() => { toast.classList.add('removing'); setTimeout(() => toast.remove(), 300); }, duration);
        },
        success: (m) => Toast.show(m, 'success'),
        error: (m) => Toast.show(m, 'error', 6000),
        warning: (m) => Toast.show(m, 'warning'),
        info: (m) => Toast.show(m, 'info'),
    };

    // Modal
    const Modal = {
        overlay: document.getElementById('modal-overlay'),
        title: document.getElementById('modal-title'),
        body: document.getElementById('modal-body'),
        closeBtn: document.getElementById('modal-close'),
        open(title, html) { this.title.textContent = title; this.body.innerHTML = html; this.overlay.classList.add('active'); },
        close() { this.overlay.classList.remove('active'); },
        init() {
            this.closeBtn.addEventListener('click', () => this.close());
            this.overlay.addEventListener('click', (e) => { if (e.target === this.overlay) this.close(); });
        }
    };

    // Confirm Dialog
    const Confirm = {
        overlay: document.getElementById('confirm-overlay'),
        title: document.getElementById('confirm-title'),
        message: document.getElementById('confirm-message'),
        okBtn: document.getElementById('confirm-ok'),
        cancelBtn: document.getElementById('confirm-cancel'),
        _resolve: null,
        show(title, message) {
            return new Promise((resolve) => {
                this.title.textContent = title; this.message.textContent = message;
                this.overlay.classList.add('active'); this._resolve = resolve;
            });
        },
        init() {
            this.okBtn.addEventListener('click', () => { this.overlay.classList.remove('active'); if (this._resolve) this._resolve(true); });
            this.cancelBtn.addEventListener('click', () => { this.overlay.classList.remove('active'); if (this._resolve) this._resolve(false); });
            this.overlay.addEventListener('click', (e) => { if (e.target === this.overlay) { this.overlay.classList.remove('active'); if (this._resolve) this._resolve(false); } });
        }
    };

    // Utilities
    function formatBytes(b) { if (!b) return '0 B'; const k = 1024, s = ['B', 'KB', 'MB', 'GB', 'TB'], i = Math.floor(Math.log(b) / Math.log(k)); return parseFloat((b / Math.pow(k, i)).toFixed(2)) + ' ' + s[i]; }
    function truncateDigest(d, l = 16) { if (!d) return ''; return d.startsWith('sha256:') ? 'sha256:' + d.slice(7, 7 + l) + '...' : d.slice(0, l) + '...'; }
    function showLoading() { return '<div class="loading-spinner"><div class="spinner"></div><span>Loading...</span></div>'; }
    function escapeHtml(s) { if (!s) return ''; const d = document.createElement('div'); d.appendChild(document.createTextNode(s)); return d.innerHTML; }
    function showEmpty(icon, title, msg, action = '') { return `<div class="empty-state"><div class="empty-state-icon">${icon}</div><h3>${title}</h3><p>${msg}</p>${action}</div>`; }

    // ============================
    // Page Renderers
    // ============================
    async function renderDashboard() {
        const c = document.getElementById('page-container');
        c.innerHTML = '<div class="page-enter">' + showLoading() + '</div>';
        try {
            const res = await API.getDashboardStats();
            const s = res.data;
            const er = s.embedded_registry || {};

            let regCardsHtml = '';
            if (s.registries && s.registries.length > 0) {
                regCardsHtml = s.registries.map((r, i) => `
                    <div class="registry-card" style="animation-delay:${i * 0.08}s" onclick="window.app.viewRegistryImages(${r.id})">
                        <div class="registry-card-header">
                            <div class="registry-card-info"><h3>${escapeHtml(r.name)}</h3><div class="registry-card-url">${escapeHtml(r.url)}</div></div>
                            <span class="badge ${r.status === 'online' ? 'badge-success' : 'badge-danger'}"><span class="badge-dot"></span>${r.status}</span>
                        </div>
                        <div class="registry-card-stats">
                            <div class="registry-stat"><span class="registry-stat-value">${r.image_count}</span><span class="registry-stat-label">Images</span></div>
                        </div>
                    </div>`).join('');
            }

            c.innerHTML = `<div class="page-enter">
                <!-- Embedded Registry Status -->
                <div class="card" style="margin-bottom:24px;border-left:3px solid ${er.running ? 'var(--success)' : 'var(--danger)'}">
                    <div style="display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:12px">
                        <div style="display:flex;align-items:center;gap:14px">
                            <div style="width:48px;height:48px;border-radius:var(--radius-md);background:${er.running ? 'var(--success-bg)' : 'var(--danger-bg)'};display:flex;align-items:center;justify-content:center">
                                <span style="font-size:1.5rem">${er.running ? '🐳' : '⏹️'}</span>
                            </div>
                            <div>
                                <div style="font-weight:700;font-size:1.05rem">Embedded Registry V2</div>
                                <div style="font-size:0.85rem;color:var(--text-muted)">${er.running ? 'Running at <strong style=color:var(--text-accent)>' + escapeHtml(er.url || '') + '</strong>' : 'Not running'}</div>
                            </div>
                        </div>
                        <div style="display:flex;gap:8px">
                            ${er.running
                    ? '<button class="btn btn-sm btn-ghost" onclick="window.app.embeddedAction(\'restart\')">🔄 Restart</button><button class="btn btn-sm btn-danger" onclick="window.app.embeddedAction(\'stop\')">Stop</button>'
                    : '<button class="btn btn-sm btn-success" onclick="window.app.embeddedAction(\'start\')">▶ Start</button>'
                }
                        </div>
                    </div>
                </div>

                <div class="stats-grid">
                    <div class="stat-card stat-registries"><div class="stat-icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"/><path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/></svg></div><div class="stat-value">${s.total_registries}</div><div class="stat-label">Registries</div></div>
                    <div class="stat-card stat-images"><div class="stat-icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg></div><div class="stat-value">${s.total_images}</div><div class="stat-label">Images</div></div>
                    <div class="stat-card stat-tags"><div class="stat-icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M20.59 13.41l-7.17 7.17a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z"/><line x1="7" y1="7" x2="7.01" y2="7"/></svg></div><div class="stat-value">${s.total_tags}</div><div class="stat-label">Tags</div></div>
                    <div class="stat-card stat-storage"><div class="stat-icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 7V4a2 2 0 0 1 2-2h8.5L20 7.5V20a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2v-3"/><polyline points="14 2 14 8 20 8"/></svg></div><div class="stat-value" style="text-transform:uppercase;font-size:1.3rem">${s.storage_type || 'N/A'}</div><div class="stat-label">Storage Type</div></div>
                </div>
                ${regCardsHtml ? '<div class="section-header"><h2>Connected Registries</h2></div><div class="registry-grid">' + regCardsHtml + '</div>' : ''}
            </div>`;
        } catch (err) {
            c.innerHTML = showEmpty('<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>', 'Unable to load dashboard', err.message);
        }
    }

    async function renderRegistries() {
        const c = document.getElementById('page-container');
        c.innerHTML = '<div class="page-enter">' + showLoading() + '</div>';
        try {
            const res = await API.getRegistries();
            const regs = res.data || [];
            if (!regs.length) { c.innerHTML = '<div class="page-enter">' + showEmpty('<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"/><path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/></svg>', 'No registries configured', 'Add your first Docker Registry V2 connection.', '<button class="btn btn-primary" onclick="window.app.showAddRegistry()"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg> Add Registry</button>') + '</div>'; return; }
            const cards = regs.map((r, i) => `
                <div class="registry-card" style="animation-delay:${i * 0.06}s">
                    <div class="registry-card-header">
                        <div class="registry-card-info"><h3>${escapeHtml(r.name)}</h3><div class="registry-card-url">${escapeHtml(r.url)}</div></div>
                        <div class="registry-card-actions">
                            <button class="btn btn-icon btn-ghost" onclick="event.stopPropagation();window.app.testRegistry(${r.id})" title="Test"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg></button>
                            <button class="btn btn-icon btn-ghost" onclick="event.stopPropagation();window.app.showEditRegistry(${r.id})" title="Edit"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg></button>
                            <button class="btn btn-icon btn-ghost" onclick="event.stopPropagation();window.app.deleteRegistry(${r.id},'${escapeHtml(r.name)}')" title="Delete" style="color:var(--danger)"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg></button>
                        </div>
                    </div>
                    <div class="registry-card-stats" onclick="window.app.viewRegistryImages(${r.id})" style="cursor:pointer">
                        <div class="registry-stat"><span class="registry-stat-value">${r.insecure ? 'HTTP' : 'HTTPS'}</span><span class="registry-stat-label">Protocol</span></div>
                        <div class="registry-stat"><span class="registry-stat-value">${r.username ? 'Yes' : 'No'}</span><span class="registry-stat-label">Auth</span></div>
                        <div class="registry-stat"><span class="registry-stat-label" style="color:var(--text-accent);cursor:pointer">Browse Images →</span></div>
                    </div>
                </div>`).join('');
            c.innerHTML = `<div class="page-enter"><div class="section-header"><h2>Managed Registries</h2><div class="section-header-actions"><button class="btn btn-primary" onclick="window.app.showAddRegistry()"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg> Add Registry</button></div></div><div class="registry-grid">${cards}</div></div>`;
        } catch (err) { Toast.error('Failed to load registries: ' + err.message); }
    }

    async function renderImages() {
        const c = document.getElementById('page-container');
        c.innerHTML = '<div class="page-enter">' + showLoading() + '</div>';
        try {
            const res = await API.getRegistries();
            const regs = res.data || [];
            if (!regs.length) { c.innerHTML = '<div class="page-enter">' + showEmpty('<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>', 'No registries available', 'Please add a registry first.', '<button class="btn btn-primary" onclick="window.app.navigate(\'registries\')">Go to Registries</button>') + '</div>'; return; }
            const opts = regs.map(r => `<option value="${r.id}">${escapeHtml(r.name)} (${escapeHtml(r.url)})</option>`).join('');
            c.innerHTML = `<div class="page-enter"><div class="registry-selector"><label for="registry-select">Select Registry:</label><select id="registry-select" class="form-select" onchange="window.app.loadImages(this.value)"><option value="">-- Choose a registry --</option>${opts}</select></div><div id="images-content"></div></div>`;
            if (window.app._selectedRegistry) { document.getElementById('registry-select').value = window.app._selectedRegistry; window.app.loadImages(window.app._selectedRegistry); }
        } catch (err) { Toast.error('Failed: ' + err.message); }
    }

    async function renderStorage() {
        const c = document.getElementById('page-container');
        c.innerHTML = '<div class="page-enter">' + showLoading() + '</div>';
        try {
            const [storageRes, statusRes] = await Promise.all([API.getStorageConfig(), API.getRegistryStatus()]);
            const cfg = storageRes.data || { type: 'local' };
            const regStatus = statusRes.data || {};

            c.innerHTML = `<div class="page-enter">
                <div class="section-header"><h2>Storage Configuration</h2></div>
                <!-- Registry Status Card -->
                <div class="card" style="margin-bottom:24px;border-left:3px solid ${regStatus.running ? 'var(--success)' : 'var(--danger)'}">
                    <div style="display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:12px">
                        <div><div style="font-weight:700">🐳 Embedded Registry</div><div style="font-size:0.85rem;color:var(--text-muted)">${regStatus.running ? 'Running at ' + escapeHtml(regStatus.url || '') : 'Stopped'}${regStatus.started_at ? ' · Started: ' + new Date(regStatus.started_at).toLocaleString() : ''}</div></div>
                        <div style="display:flex;gap:8px">
                            ${regStatus.running ? '<button class="btn btn-sm btn-ghost" onclick="window.app.embeddedAction(\'restart\')">🔄 Restart</button><button class="btn btn-sm btn-danger" onclick="window.app.embeddedAction(\'stop\')">Stop</button>' : '<button class="btn btn-sm btn-success" onclick="window.app.embeddedAction(\'start\')">▶ Start</button>'}
                            <button class="btn btn-sm btn-ghost" onclick="window.app.showRegistryLogs()">📋 Logs</button>
                        </div>
                    </div>
                </div>
                <div class="card" style="max-width:700px">
                    <div class="storage-tabs">
                        <button class="storage-tab ${cfg.type === 'local' ? 'active' : ''}" data-tab="local" onclick="window.app.switchStorageTab('local')">📁 Local</button>
                        <button class="storage-tab ${cfg.type === 's3' ? 'active' : ''}" data-tab="s3" onclick="window.app.switchStorageTab('s3')">☁️ Object Storage</button>
                        <button class="storage-tab ${cfg.type === 'sftp' ? 'active' : ''}" data-tab="sftp" onclick="window.app.switchStorageTab('sftp')">🔐 SFTP</button>
                    </div>
                    <form id="storage-form" onsubmit="event.preventDefault();window.app.saveStorage()">
                        <div id="storage-local" class="storage-form-section ${cfg.type === 'local' ? 'active' : ''}">
                            <div class="form-group"><label class="form-label">Storage Path</label><input type="text" id="local-path" class="form-input" value="${escapeHtml(cfg.local_path || '/var/lib/registry')}" placeholder="/var/lib/registry"><div class="form-hint">Path inside the container where data is stored</div></div>
                        </div>
                        <div id="storage-s3" class="storage-form-section ${cfg.type === 's3' ? 'active' : ''}">
                            <div class="form-row"><div class="form-group"><label class="form-label">Endpoint</label><input type="text" id="s3-endpoint" class="form-input" value="${escapeHtml(cfg.s3_endpoint || '')}" placeholder="s3.amazonaws.com or minio:9000"></div><div class="form-group"><label class="form-label">Region</label><input type="text" id="s3-region" class="form-input" value="${escapeHtml(cfg.s3_region || '')}" placeholder="us-east-1"></div></div>
                            <div class="form-group"><label class="form-label">Bucket</label><input type="text" id="s3-bucket" class="form-input" value="${escapeHtml(cfg.s3_bucket || '')}" placeholder="my-registry-bucket"></div>
                            <div class="form-row"><div class="form-group"><label class="form-label">Access Key</label><input type="text" id="s3-access-key" class="form-input" value="${escapeHtml(cfg.s3_access_key || '')}"></div><div class="form-group"><label class="form-label">Secret Key</label><input type="password" id="s3-secret-key" class="form-input" value="${escapeHtml(cfg.s3_secret_key || '')}"></div></div>
                            <div class="form-group"><label class="form-check"><input type="checkbox" id="s3-use-ssl" ${cfg.s3_use_ssl ? 'checked' : ''}><span class="form-check-label">Use SSL/TLS</span></label></div>
                        </div>
                        <div id="storage-sftp" class="storage-form-section ${cfg.type === 'sftp' ? 'active' : ''}">
                            <div class="form-row"><div class="form-group"><label class="form-label">Host</label><input type="text" id="sftp-host" class="form-input" value="${escapeHtml(cfg.sftp_host || '')}" placeholder="sftp.example.com"></div><div class="form-group"><label class="form-label">Port</label><input type="number" id="sftp-port" class="form-input" value="${cfg.sftp_port || 22}"></div></div>
                            <div class="form-row"><div class="form-group"><label class="form-label">Username</label><input type="text" id="sftp-user" class="form-input" value="${escapeHtml(cfg.sftp_user || '')}"></div><div class="form-group"><label class="form-label">Password</label><input type="password" id="sftp-password" class="form-input" value="${escapeHtml(cfg.sftp_password || '')}"></div></div>
                            <div class="form-group"><label class="form-label">Remote Path</label><input type="text" id="sftp-path" class="form-input" value="${escapeHtml(cfg.sftp_path || '')}" placeholder="/data/registry"></div>
                            <div class="form-group"><label class="form-label">Private Key (optional)</label><textarea id="sftp-key" class="form-textarea" placeholder="SSH private key...">${escapeHtml(cfg.sftp_private_key || '')}</textarea></div>
                        </div>
                        <div style="display:flex;gap:12px;margin-top:8px">
                            <button type="submit" class="btn btn-primary"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z"/><polyline points="17 21 17 13 7 13 7 21"/><polyline points="7 3 7 8 15 8"/></svg> Save & Apply</button>
                            <button type="button" class="btn btn-ghost" onclick="window.app.testStorage()"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg> Test</button>
                        </div>
                    </form>
                </div></div>`;
        } catch (err) { Toast.error('Failed to load storage config: ' + err.message); }
    }

    async function renderRetention() {
        const c = document.getElementById('page-container');
        c.innerHTML = '<div class="page-enter">' + showLoading() + '</div>';
        try {
            const res = await API.getRegistries();
            const regs = res.data || [];
            if (!regs.length) {
                c.innerHTML = '<div class="page-enter">' + showEmpty('<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"></polyline><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path></svg>', 'No registries configured', 'Please add a registry first.') + '</div>';
                return;
            }

            const opts = regs.map(r => `<option value="${r.id}">${escapeHtml(r.name)} (${escapeHtml(r.url)})</option>`).join('');

            c.innerHTML = `
                <div class="page-enter">
                    <div class="section-header"><h2>Image Cleanup Policy</h2></div>
                    <div class="card" style="margin-bottom:24px">
                        <div class="form-group">
                            <label class="form-label">Select Registry to Configure</label>
                            <select id="retention-reg-select" class="form-select" onchange="window.app.loadRetentionPolicy(this.value)">
                                <option value="">-- Select Registry --</option>
                                ${opts}
                            </select>
                        </div>
                    </div>
                    <div id="retention-config-area"></div>
                </div>`;

            if (window.app._selectedRegistry) {
                const sel = document.getElementById('retention-reg-select');
                if (sel) {
                    sel.value = window.app._selectedRegistry;
                    window.app.loadRetentionPolicy(window.app._selectedRegistry);
                }
            } else if (regs.length > 0) {
                const sel = document.getElementById('retention-reg-select');
                if (sel) {
                    sel.value = regs[0].id; // Auto select first
                    window.app.loadRetentionPolicy(regs[0].id);
                }
            }
        } catch (e) { Toast.error(e.message); }
    }

    // Scanner Renderer
    async function renderScanner() {
        const c = document.getElementById('page-container');
        c.innerHTML = '<div class="page-enter">' + showLoading() + '</div>';
        try {
            const res = await API.getRegistries();
            const regs = res.data || [];
            if (!regs.length) { c.innerHTML = '<div class="page-enter">' + showEmpty('🛡️', 'No registries', 'Add a registry first.') + '</div>'; return; }

            const selectedReg = window.app._selectedRegistry || (regs[0] ? regs[0].id : null);
            const opts = regs.map(r => `<option value="${r.id}" ${r.id == selectedReg ? 'selected' : ''}>${escapeHtml(r.name)}</option>`).join('');

            c.innerHTML = `
                <div class="page-enter">
                    <div class="section-header"><h2>Vulnerability Scanner (Trivy & OSV)</h2></div>
                    <div class="card" style="margin-bottom:24px;padding:20px;display:flex;justify-content:space-between;align-items:flex-end;gap:16px">
                        <div style="flex:1;max-width:400px">
                            <label class="form-label" style="display:block;margin-bottom:8px">Select Registry</label>
                            <select id="scan-reg-select" class="form-select" style="width:100%" onchange="window.app.loadScanImages(this.value)">${opts}</select>
                        </div>
                        <div>
                            <button class="btn btn-secondary" onclick="const v=document.getElementById('scan-reg-select').value; if(v) window.app.configureSchedule(v); else alert('Select registry first')">
                                ⏱️ Schedule Scan
                            </button>
                        </div>
                    </div>
                    <div id="scan-content"></div>
                </div>`;
            if (selectedReg) window.app.loadScanImages(selectedReg);
        } catch (e) { c.innerHTML = showEmpty('⚠️', 'Error', e.message); }
    }

    // Vuln Report Renderer
    async function renderVulnReport() {
        const c = document.getElementById('page-container');
        c.innerHTML = '<div class="page-enter">' + showLoading() + '</div>';
        try {
            const res = await API.getRegistries();
            const regs = res.data || [];
            if (!regs.length) { c.innerHTML = '<div class="page-enter">' + showEmpty('📋', 'No registries', 'Add a registry first.') + '</div>'; return; }

            const selectedReg = window.app._selectedRegistry || (regs[0] ? regs[0].id : null);
            const opts = regs.map(r => `<option value="${r.id}" ${r.id == selectedReg ? 'selected' : ''}>${escapeHtml(r.name)}</option>`).join('');

            c.innerHTML = `
                <div class="page-enter">
                    <div class="section-header"><h2>📋 Vulnerability Report</h2></div>
                    <div class="card" style="margin-bottom:24px;padding:20px">
                        <div style="display:flex;gap:16px;align-items:flex-end;flex-wrap:wrap">
                            <div style="flex:1;min-width:200px">
                                <label class="form-label" style="display:block;margin-bottom:8px">Select Registry</label>
                                <select id="vuln-reg-select" class="form-select" style="width:100%" onchange="window.app.loadVulnerabilities(this.value)">${opts}</select>
                            </div>
                            <div style="flex:2;min-width:300px">
                                <label class="form-label" style="display:block;margin-bottom:8px">🔍 Search Vulnerabilities</label>
                                <input type="text" id="vuln-search" class="form-input" placeholder="Search by CVE ID, package, description..." oninput="window.app.filterVulnerabilities()">
                            </div>
                        </div>
                        <div style="display:flex;gap:12px;margin-top:16px;flex-wrap:wrap">
                            <div>
                                <label class="form-label" style="display:block;margin-bottom:8px">Filter by Repository</label>
                                <input type="text" id="vuln-filter-repo" class="form-input" placeholder="e.g. nginx..." oninput="window.app.filterVulnerabilities()">
                            </div>
                            <div>
                                <label class="form-label" style="display:block;margin-bottom:8px">Filter by Tag</label>
                                <input type="text" id="vuln-filter-tag" class="form-input" placeholder="e.g. latest..." oninput="window.app.filterVulnerabilities()">
                            </div>
                            <div>
                                <label class="form-label" style="display:block;margin-bottom:8px">Severity</label>
                                <select id="vuln-filter-severity" class="form-select" onchange="window.app.filterVulnerabilities()">
                                    <option value="">All Severities</option>
                                    <option value="CRITICAL">Critical</option>
                                    <option value="HIGH">High</option>
                                    <option value="MEDIUM">Medium</option>
                                    <option value="LOW">Low</option>
                                    <option value="UNKNOWN">Unknown</option>
                                </select>
                            </div>
                            <div>
                                <label class="form-label" style="display:block;margin-bottom:8px">Scanner</label>
                                <select id="vuln-filter-scanner" class="form-select" onchange="window.app.filterVulnerabilities()">
                                    <option value="">All Scanners</option>
                                    <option value="Trivy">Trivy</option>
                                    <option value="OSV">OSV</option>
                                </select>
                            </div>
                        </div>
                    </div>
                    <div id="vuln-report-content"></div>
                </div>`;
            if (selectedReg) window.app.loadVulnerabilities(selectedReg);
        } catch (e) { c.innerHTML = showEmpty('⚠️', 'Error', e.message); }
    }

    // App Controller
    const app = {
        currentPage: 'dashboard', _selectedRegistry: null,
        init() {
            Modal.init(); Confirm.init();
            window.addEventListener('hashchange', () => this.handleRoute());
            document.getElementById('sidebar-toggle').addEventListener('click', () => document.getElementById('sidebar').classList.toggle('open'));
            document.querySelector('.main-content').addEventListener('click', () => document.getElementById('sidebar').classList.remove('open'));
            this.handleRoute();
        },

        // Retention Logic
        async loadRetentionPolicy(regId) {
            if (!regId) { document.getElementById('retention-config-area').innerHTML = ''; return; }
            this._selectedRegistry = regId;
            const area = document.getElementById('retention-config-area');
            area.innerHTML = showLoading();

            try {
                const res = await API.getRetention(regId);
                const p = res.data;
                const lastRun = p.last_run_at ? new Date(p.last_run_at).toLocaleString() : 'Never';

                area.innerHTML = `
                    <div class="card fade-in">
                        <div class="section-header" style="margin-top:0">
                            <h3>Configuration</h3>
                            <span class="badge badge-info">Last Run: ${lastRun}</span>
                        </div>
                        <form onsubmit="event.preventDefault();window.app.saveRetentionPolicy(${regId})">
                            <div class="form-group">
                                <label class="form-label">Keep Last N Images</label>
                                <input type="number" id="keep-last" class="form-input" value="${p.keep_last_count}" min="0">
                                <div class="form-hint">Number of most recent images to always keep (0 to disable)</div>
                            </div>
                            <div class="form-group">
                                <label class="form-label">Keep Images Newer Than (Days)</label>
                                <input type="number" id="keep-days" class="form-input" value="${p.keep_days}" min="0">
                                <div class="form-hint">Keep images pushed within the last N days (0 to disable)</div>
                            </div>
                            
                            <hr style="border:0;border-top:1px solid var(--border);margin:20px 0;">
                            <h4>Filters & Whitelists (Regex)</h4>
                            
                            <div class="form-group">
                                <label class="form-label">Whitelist Tags (Do Not Delete)</label>
                                <input type="text" id="exclude-tags" class="form-input" value="${escapeHtml(p.exclude_tags || '')}" placeholder="e.g. ^latest$|^v1\..*">
                                <div class="form-hint">Regex pattern for tags to ALWAYS keep (e.g. <code>^latest$</code>)</div>
                            </div>
                             <div class="form-group">
                                <label class="form-label">Process Repositories (Include)</label>
                                <input type="text" id="filter-repos" class="form-input" value="${escapeHtml(p.filter_repos || '')}" placeholder="e.g. ^myservice.*">
                                <div class="form-hint">Only process repos matching this regex (empty = process all)</div>
                            </div>
                            <div class="form-group">
                                <label class="form-label">Exclude Repositories</label>
                                <input type="text" id="exclude-repos" class="form-input" value="${escapeHtml(p.exclude_repos || '')}" placeholder="e.g. ^base-images/.*">
                                <div class="form-hint">Skip repos matching this regex</div>
                            </div>

                            <hr style="border:0;border-top:1px solid var(--border);margin:20px 0;">

                            <div class="form-group">
                                <label class="form-check">
                                    <input type="checkbox" id="retention-dry-run" ${p.dry_run ? 'checked' : ''}>
                                    <span class="form-check-label">Dry Run (Simulate only, do not delete)</span>
                                </label>
                            </div>
                            <div style="display:flex;gap:12px;margin-top:16px">
                                <button type="submit" class="btn btn-primary">Save Policy</button>
                                <button type="button" class="btn btn-danger" onclick="window.app.runRetention(${regId})">Run Cleanup Now</button>
                            </div>
                        </form>
                    </div>
                    <div id="retention-logs-area" style="margin-top:24px"></div>
                `;
            } catch (e) { area.innerHTML = showEmpty('⚠️', 'Error', e.message); }
        },

        async saveRetentionPolicy(id) {
            const data = {
                keep_last_count: parseInt(document.getElementById('keep-last').value) || 0,
                keep_days: parseInt(document.getElementById('keep-days').value) || 0,
                dry_run: document.getElementById('retention-dry-run').checked,
                filter_repos: document.getElementById('filter-repos').value,
                exclude_repos: document.getElementById('exclude-repos').value,
                exclude_tags: document.getElementById('exclude-tags').value
            };
            try { await API.saveRetention(id, data); Toast.success('Policy saved!'); } catch (e) { Toast.error(e.message); }
        },

        async runRetention(id) {
            const wetRun = !document.getElementById('retention-dry-run').checked;
            const mode = wetRun ? 'DELETE' : 'Simulate';
            if (wetRun && !(await Confirm.show('Run Cleanup', 'This will PERMANENTLY DELETE images not matching the policy. Continue?'))) return;

            const area = document.getElementById('retention-logs-area');
            area.innerHTML = showLoading();
            Toast.info(`Running cleanup (${mode})...`);

            try {
                const dry = !wetRun;
                const res = await API.runRetention(id, dry);
                const logs = res.data || [];

                if (!logs.length) {
                    area.innerHTML = showEmpty('🧹', 'Clean', 'No images found to process.');
                    return;
                }

                const deleted = logs.filter(l => l.action.includes('delete') || l.action === 'error_delete');
                const kept = logs.filter(l => l.action === 'kept');

                let html = `
                    <div class="card fade-in">
                        <h3>Cleanup Result (${mode})</h3>
                        <div style="display:flex;gap:16px;margin:12px 0;">
                            <div class="registry-stat"><span class="registry-stat-value">${deleted.length}</span><span class="registry-stat-label">Deleted/Marked</span></div>
                            <div class="registry-stat"><span class="registry-stat-value">${kept.length}</span><span class="registry-stat-label">Kept</span></div>
                        </div>
                        <div style="overflow-x:auto">
                            <table style="width:100%;font-size:0.9rem;border-collapse:collapse;">
                                <thead><tr style="text-align:left;border-bottom:1px solid var(--border);color:var(--text-muted)"><th style="padding:8px">Image:Tag</th><th style="padding:8px">Created</th><th style="padding:8px">Action</th><th style="padding:8px">Reason</th></tr></thead>
                                <tbody>
                                    ${logs.map(l => {
                    let color = l.action === 'kept' ? 'var(--success)' : (l.action === 'deleted' ? 'var(--danger)' : 'var(--warning)');
                    return `<tr style="border-bottom:1px solid var(--border)">
                                            <td style="padding:8px">${escapeHtml(l.repository)}:<span style="color:var(--text-accent)">${escapeHtml(l.tag)}</span></td>
                                            <td style="padding:8px">${new Date(l.created).toLocaleDateString()}</td>
                                            <td style="padding:8px"><span class="badge" style="background:${color}20;color:${color}">${l.action}</span></td>
                                            <td style="padding:8px;color:var(--text-muted)">${escapeHtml(l.reason)}</td>
                                        </tr>`;
                }).join('')}
                                </tbody>
                            </table>
                        </div>
                    </div>`;
                area.innerHTML = html;
                Toast.success('Run completed');
            } catch (e) {
                area.innerHTML = showEmpty('⚠️', 'Run Failed', e.message);
                Toast.error(e.message);
            }
        },
        handleRoute() { const p = (window.location.hash.slice(1) || 'dashboard').split('/')[0]; this.navigate(p, false); },
        navigate(page, upHash = true) {
            this.currentPage = page;
            if (upHash) window.location.hash = page;
            document.querySelectorAll('.nav-item').forEach(i => i.classList.toggle('active', i.dataset.page === page));
            const titles = { dashboard: 'Dashboard', registries: 'Registries', images: 'Images', retention: 'Image Retention', scan: 'Vulnerability Scanner', 'vuln-report': 'Vulnerability Report', storage: 'Storage Settings' };
            document.getElementById('page-title').textContent = titles[page] || 'Dashboard';
            ({
                dashboard: renderDashboard,
                registries: renderRegistries,
                images: renderImages,
                retention: renderRetention,
                scan: renderScanner,
                'vuln-report': renderVulnReport,
                storage: renderStorage
            }[page] || renderDashboard)();
        },

        // Scan Methods
        async loadScanImages(regId) {
            this._selectedRegistry = regId;
            const area = document.getElementById('scan-content');
            if (!area) return;
            area.innerHTML = showLoading();
            if (!regId) { area.innerHTML = ''; return; }

            try {
                const [repoRes, scanRes] = await Promise.all([API.getRepositories(regId), API.listScans(regId)]);
                const repos = repoRes.data || [];

                if (!repos.length) {
                    area.innerHTML = showEmpty('📦', 'No Repositories', 'Registry is empty.');
                    return;
                }

                let html = `
                <div class="fade-in" style="margin-top:24px;width:100%">
                    <div style="display:flex;gap:20px;margin-bottom:24px;width:100%">
                        <div style="flex:1;position:relative">
                            <span style="position:absolute;left:16px;top:50%;transform:translateY(-50%);color:#64748b;pointer-events:none">🔍</span>
                            <input type="text" class="form-control" placeholder="Search repositories..." 
                                   style="width:100%;padding-left:48px;height:50px;background:#ffffff;border:1px solid #e2e8f0;border-radius:10px;color:#334155;font-size:1rem;box-shadow:0 2px 4px rgba(0,0,0,0.05)" 
                                   onkeyup="window.app.filterScanRepos(this.value)">
                        </div>
                        <div style="flex:1;position:relative">
                            <span style="position:absolute;left:16px;top:50%;transform:translateY(-50%);color:#64748b;pointer-events:none">📦</span>
                            <input type="text" class="form-control" placeholder="Search packages, CVE IDs..." 
                                   style="width:100%;padding-left:48px;height:50px;background:#ffffff;border:1px solid #e2e8f0;border-radius:10px;color:#334155;font-size:1rem;box-shadow:0 2px 4px rgba(0,0,0,0.05)" 
                                   onkeyup="window.app.filterScanPackages(this.value)">
                        </div>
                        <div style="width:200px;position:relative">
                            <select id="scan-tool-select" class="form-control" style="width:100%;height:50px;background:#ffffff;border:1px solid #e2e8f0;border-radius:10px;color:#334155;font-size:1rem;font-weight:600;box-shadow:0 2px 4px rgba(0,0,0,0.05);padding-left:16px">
                                <option value="trivy">Scanner: Trivy</option>
                                <option value="osv">Scanner: OSV</option>
                            </select>
                        </div>
                    </div>
                    
                    <div class="scan-repo-list" style="width:100%">`;

                html += repos.map((r, i) => `
                    <div class="scan-repo-item card mb-3 shadow-sm" data-repo="${escapeHtml(r.name).toLowerCase()}" style="background:#1a202c;border:1px solid #2d3748;border-radius:12px;overflow:hidden">
                        <div class="card-header" style="display:flex;align-items:center;justify-content:space-between;padding:20px 24px;cursor:pointer;background:transparent;border-bottom:none" onclick="window.app.toggleScanRepo(${regId}, '${escapeHtml(r.name)}', this)">
                            <div style="display:flex;align-items:center;gap:20px">
                                <div class="scan-repo-arrow" style="color:#718096;transition:transform 0.2s;font-size:1rem;width:24px;text-align:center">▶</div>
                                <div style="width:52px;height:52px;background:#2d3748;border-radius:12px;display:flex;align-items:center;justify-content:center;font-size:1.8rem;box-shadow:inset 0 2px 4px rgba(0,0,0,0.1)">📦</div>
                                <div>
                                    <h5 style="margin:0 0 6px 0;font-weight:600;font-size:1.2rem;color:#f7fafc;letter-spacing:0.3px">${escapeHtml(r.name)}</h5>
                                    <div>
                                        <span style="font-size:0.8rem;color:#cbd5e0;background:#4a5568;padding:2px 8px;border-radius:4px;font-weight:500">${r.tag_count} Tags</span>
                                    </div>
                                </div>
                            </div>
                            <button class="btn btn-outline-secondary btn-sm" style="border-radius:8px;padding:8px 16px;font-weight:500;border-color:#4a5568;color:#a0aec0;transition:all 0.2s" onmouseover="this.style.borderColor='#cbd5e0';this.style.color='#fff'" onmouseout="this.style.borderColor='#4a5568';this.style.color='#a0aec0'">View Images</button>
                        </div>
                        <div class="scan-repo-detail collapse" style="display:none;border-top:1px solid #2d3748;background:#13151a">
                            <div class="card-body p-0">
                                <div class="scan-tags-container p-4" data-repo="${escapeHtml(r.name)}">Loading tags...</div>
                            </div>
                        </div>
                    </div>
                `).join('');
                html += '</div></div>';

                area.innerHTML = html;
            } catch (e) { area.innerHTML = showEmpty('⚠️', 'Error', e.message); }
        },

        filterScanPackages(val) {
            this._packageFilter = val.toLowerCase();
            document.querySelectorAll('.rep-table tbody tr').forEach(row => {
                const visible = row.innerText.toLowerCase().includes(this._packageFilter);
                row.style.display = visible ? '' : 'none';
            });
        },

        filterScanRepos(val) {
            const term = val.toLowerCase();
            document.querySelectorAll('.scan-repo-item').forEach(item => {
                const match = item.dataset.repo.includes(term);
                item.style.display = match ? 'block' : 'none';
                // Close detail if hidden (optional)
                if (!match) {
                    const header = item.querySelector('.card-header');
                    const detail = item.querySelector('.scan-repo-detail');
                    const arrow = item.querySelector('.scan-repo-arrow');
                    if (detail) detail.style.display = 'none';
                    if (arrow) arrow.textContent = '▶';
                }
            });
        },

        async toggleScanRepo(regId, repo, row) {
            const nextRow = row.nextElementSibling;
            const arrow = row.querySelector('.scan-repo-arrow');
            const isHidden = nextRow.style.display === 'none';
            nextRow.style.display = isHidden ? 'table-row' : 'none';
            arrow.textContent = isHidden ? '▼' : '▶';

            if (isHidden) {
                const container = nextRow.querySelector('.scan-tags-container');
                container.innerHTML = showLoading();
                try {
                    const [tagRes, scanRes] = await Promise.all([API.getTags(regId, repo), API.listScans(regId)]);
                    const tags = tagRes.data || [];
                    const scans = scanRes.data || [];

                    const repoScans = {};
                    scans.filter(s => s.repository === repo).forEach(s => repoScans[s.tag] = s);

                    if (!tags.length) container.innerHTML = '<div style="padding:24px;text-align:center;color:#64748b;font-style:italic">No tags found for this repository</div>';
                    else {
                        container.innerHTML = `<table style="width:100%;table-layout:fixed;font-size:0.95rem;border-collapse:collapse;color:#e2e8f0;">
                            <thead><tr style="text-align:left;background:#1e293b;border-bottom:1px solid #334155;color:#94a3b8;font-weight:600;text-transform:uppercase;font-size:0.75rem;letter-spacing:0.05em">
                                <th style="padding:16px 24px;width:20%">Tag</th>
                                <th style="padding:16px 24px;width:20%">Digest</th>
                                <th style="padding:16px 24px;width:20%">Last Scan</th>
                                <th style="padding:16px 24px;width:15%">Result</th>
                                <th style="padding:16px 24px;width:25%">Action</th>
                            </tr></thead>
                            <tbody>${tags.map(t => {
                            const s = repoScans[t.name];
                            const detailId = `detail-${regId}-${repo}-${t.name}`.replace(/[^\w-]/g, '_');

                            let statusHtml = '<span class="badge badge-secondary">Not Scanned</span>';
                            let actionHtml = `
                                <div style="display:flex;gap:8px">
                                    <button class="btn btn-sm btn-primary" style="height:34px;padding:0 16px;border-radius:8px;font-weight:500;display:flex;align-items:center;gap:6px" onclick="event.stopPropagation();window.app.triggerScan(${regId},'${escapeHtml(repo)}','${escapeHtml(t.name)}','${t.digest}','trivy')">
                                        <span>🛡️ Scan Trivy</span>
                                    </button>
                                    <button class="btn btn-sm btn-info" style="height:34px;padding:0 16px;border-radius:8px;font-weight:500;background:#0ea5e9;border:none;color:white;display:flex;align-items:center;gap:6px" onclick="event.stopPropagation();window.app.triggerScan(${regId},'${escapeHtml(repo)}','${escapeHtml(t.name)}','${t.digest}','osv')">
                                        <span>🐞 Scan OSV</span>
                                    </button>
                                </div>`;

                            if (s) {
                                if (s.status === 'scanning' || s.status === 'pending') {
                                    statusHtml = '<span class="badge badge-warning" style="padding:6px 10px;font-size:0.8rem">Scanning...</span>';
                                    actionHtml = `<button class="btn btn-sm btn-ghost" disabled style="opacity:0.7">Please Wait...</button>`;
                                    setTimeout(() => window.app.refreshScanStatus(regId, repo, t.name), 4000);
                                } else if (s.status === 'failed') {
                                    statusHtml = '<span class="badge badge-danger">Failed</span>';
                                    actionHtml = `
                                        <div style="display:flex;gap:8px">
                                            <button class="btn btn-sm btn-outline-danger" style="height:32px;padding:0 12px;border-radius:8px" onclick="event.stopPropagation();window.app.triggerScan(${regId},'${escapeHtml(repo)}','${escapeHtml(t.name)}','${t.digest}','trivy')">Retry Trivy</button>
                                            <button class="btn btn-sm btn-outline-danger" style="height:32px;padding:0 12px;border-radius:8px" onclick="event.stopPropagation();window.app.triggerScan(${regId},'${escapeHtml(repo)}','${escapeHtml(t.name)}','${t.digest}','osv')">Retry OSV</button>
                                        </div>`;
                                } else {
                                    try {
                                        const sum = JSON.parse(s.summary);
                                        statusHtml = '';

                                        const renderBadges = (d) => {
                                            let h = '';
                                            if (d.Critical) h += `<span class="badge badge-danger" style="margin-right:4px;padding:4px 6px;font-size:0.75rem">C:${d.Critical}</span>`;
                                            if (d.High) h += `<span class="badge badge-warning" style="margin-right:4px;padding:4px 6px;font-size:0.75rem">H:${d.High}</span>`;
                                            if (!h) h = '<span class="badge badge-success" style="padding:4px 6px;font-size:0.75rem">Clean</span>';
                                            return h;
                                        };

                                        if (sum.trivy || sum.osv) {
                                            if (sum.trivy) statusHtml += `<div style="display:flex;align-items:center;margin-bottom:4px"><span style="width:40px;font-size:0.75rem;opacity:0.8">Trivy</span>${renderBadges(sum.trivy)}</div>`;
                                            if (sum.osv) statusHtml += `<div style="display:flex;align-items:center"><span style="width:40px;font-size:0.75rem;opacity:0.8">OSV</span>${renderBadges(sum.osv)}</div>`;
                                        } else {
                                            statusHtml = renderBadges(sum);
                                        }
                                    } catch (e) { statusHtml = '<span class="badge badge-success">Done</span>'; }
                                    actionHtml = `
                                        <div style="display:flex;gap:8px;align-items:center">
                                            <button class="btn btn-sm btn-secondary" onclick="event.stopPropagation();window.app.toggleReport(${regId},'${escapeHtml(repo)}','${escapeHtml(t.name)}','${detailId}')" style="height:34px;padding:0 14px;border-radius:8px;background:#334155;color:#f1f5f9;border:none;font-weight:500">View Report</button>
                                            
                                            <button class="btn btn-sm" onclick="event.stopPropagation();window.app.triggerScan(${regId},'${escapeHtml(repo)}','${escapeHtml(t.name)}','${t.digest}','trivy')" 
                                                    style="height:34px;padding:0 12px;border:1px solid #475569;border-radius:8px;color:#cbd5e0;background:transparent;font-size:0.85rem;font-weight:500;transition:all 0.2s" 
                                                    onmouseover="this.style.borderColor='#3b82f6';this.style.color='#60a5fa'" onmouseout="this.style.borderColor='#475569';this.style.color='#cbd5e0'">
                                                Trivy
                                            </button>
                                            
                                            <button class="btn btn-sm" onclick="event.stopPropagation();window.app.triggerScan(${regId},'${escapeHtml(repo)}','${escapeHtml(t.name)}','${t.digest}','osv')" 
                                                    style="height:34px;padding:0 12px;border:1px solid #475569;border-radius:8px;color:#cbd5e0;background:transparent;font-size:0.85rem;font-weight:500;transition:all 0.2s" 
                                                    onmouseover="this.style.borderColor='#06b6d4';this.style.color='#22d3ee'" onmouseout="this.style.borderColor='#475569';this.style.color='#cbd5e0'">
                                                OSV
                                            </button>
                                        </div>
                                    `;
                                }
                            }
                            return `<tr style="border-bottom:1px solid var(--border-subtle)">
                                    <td style="padding:10px;font-weight:500">${escapeHtml(t.name)}</td>
                                    <td style="padding:10px;font-family:monospace;color:var(--text-muted)">${truncateDigest(t.digest, 8)}</td>
                                    <td style="padding:10px">${s ? new Date(s.scanned_at).toLocaleString() : '-'}</td>
                                    <td style="padding:10px" id="scan-status-${regId}-${repo.replace(/\W/g, '')}-${t.name.replace(/\W/g, '')}">${statusHtml}</td>
                                    <td style="padding:10px">${actionHtml}</td>
                                </tr>
                                <tr id="${detailId}" style="display:none">
                                    <td colspan="5" style="padding:0">
                                        <div class="report-content-box" style="padding:20px 24px;background:#151520;border-bottom:1px solid var(--border)"></div>
                                    </td>
                                </tr>`;
                        }).join('')}</tbody></table>`;
                    }
                } catch (e) { container.innerHTML = `<div style="color:var(--danger)">Error: ${e.message}</div>`; }
            }
        },

        async triggerScan(regId, repo, tag, digest, specificScanner = null) {
            const scanner = specificScanner || (document.getElementById('scan-tool-select') ? document.getElementById('scan-tool-select').value : 'trivy');
            Toast.info(`Starting ${scanner.toUpperCase()} scan for ${repo}:${tag}...`);
            try {
                await API.triggerScan({ registry_id: parseInt(regId), repository: repo, tag: tag, digest: digest, scanner: scanner });
                Toast.success('Scan started');
                const cellId = `scan-status-${regId}-${repo.replace(/\W/g, '')}-${tag.replace(/\W/g, '')}`;
                const cell = document.getElementById(cellId);
                if (cell) {
                    cell.innerHTML = '<span class="badge badge-warning">Scanning...</span>';
                    cell.nextElementSibling.innerHTML = '<button class="btn btn-sm btn-ghost" disabled>Wait...</button>';
                }
                setTimeout(() => window.app.refreshScanStatus(regId, repo, tag), 3000);
            } catch (e) { Toast.error(e.message); }
        },

        async refreshScanStatus(regId, repo, tag) {
            try {
                const res = await API.getScanResult(regId, repo, tag);
                const s = res.data;
                const cellId = `scan-status-${regId}-${repo.replace(/\W/g, '')}-${tag.replace(/\W/g, '')}`;
                const cell = document.getElementById(cellId);
                if (s && cell) {
                    if (s.status === 'completed') {
                        try {
                            const sum = JSON.parse(s.summary);
                            const renderBadges = (d) => {
                                let h = '';
                                if (d.Critical) h += `<span class="badge badge-danger" style="margin-right:4px;padding:4px 6px;font-size:0.75rem">C:${d.Critical}</span>`;
                                if (d.High) h += `<span class="badge badge-warning" style="margin-right:4px;padding:4px 6px;font-size:0.75rem">H:${d.High}</span>`;
                                if (!h) h = '<span class="badge badge-success" style="padding:4px 6px;font-size:0.75rem">Clean</span>';
                                return h;
                            };

                            let html = '';
                            if (sum.trivy || sum.osv) {
                                if (sum.trivy) html += `<div style="display:flex;align-items:center;margin-bottom:4px"><span style="width:40px;font-size:0.75rem;opacity:0.8">Trivy</span>${renderBadges(sum.trivy)}</div>`;
                                if (sum.osv) html += `<div style="display:flex;align-items:center"><span style="width:40px;font-size:0.75rem;opacity:0.8">OSV</span>${renderBadges(sum.osv)}</div>`;
                            } else {
                                html = renderBadges(sum);
                            }
                            cell.innerHTML = html;
                            cell.nextElementSibling.innerHTML = `
                                <div style="display:flex;gap:8px;align-items:center">
                                    <button class="btn btn-sm btn-secondary" onclick="event.stopPropagation();window.app.viewScanReport(${regId},'${escapeHtml(repo)}','${escapeHtml(tag)}')" style="height:34px;padding:0 14px;border-radius:8px;background:#334155;color:#f1f5f9;border:none;font-weight:500">View Report</button>
                                    
                                    <button class="btn btn-sm" onclick="event.stopPropagation();window.app.triggerScan(${regId},'${escapeHtml(repo)}','${escapeHtml(tag)}','','trivy')" 
                                            style="height:34px;padding:0 12px;border:1px solid #475569;border-radius:8px;color:#cbd5e0;background:transparent;font-size:0.85rem;font-weight:500;transition:all 0.2s" 
                                            onmouseover="this.style.borderColor='#3b82f6';this.style.color='#60a5fa'" onmouseout="this.style.borderColor='#475569';this.style.color='#cbd5e0'">
                                        Trivy
                                    </button>
                                    
                                    <button class="btn btn-sm" onclick="event.stopPropagation();window.app.triggerScan(${regId},'${escapeHtml(repo)}','${escapeHtml(tag)}','','osv')" 
                                            style="height:34px;padding:0 12px;border:1px solid #475569;border-radius:8px;color:#cbd5e0;background:transparent;font-size:0.85rem;font-weight:500;transition:all 0.2s" 
                                            onmouseover="this.style.borderColor='#06b6d4';this.style.color='#22d3ee'" onmouseout="this.style.borderColor='#475569';this.style.color='#cbd5e0'">
                                        OSV
                                    </button>
                                </div>
                            `;
                        } catch (e) { }
                    } else if (s.status === 'failed') {
                        cell.innerHTML = '<span class="badge badge-danger">Failed</span>';
                        cell.nextElementSibling.innerHTML = `
                                        <div style="display:flex;gap:8px">
                                            <button class="btn btn-sm btn-outline-danger" style="height:32px;padding:0 12px;border-radius:8px" onclick="event.stopPropagation();window.app.triggerScan(${regId},'${escapeHtml(repo)}','${escapeHtml(tag)}','','trivy')">Retry Trivy</button>
                                            <button class="btn btn-sm btn-outline-danger" style="height:32px;padding:0 12px;border-radius:8px" onclick="event.stopPropagation();window.app.triggerScan(${regId},'${escapeHtml(repo)}','${escapeHtml(tag)}','','osv')">Retry OSV</button>
                                        </div>`;
                    } else {
                        setTimeout(() => window.app.refreshScanStatus(regId, repo, tag), 3000);
                    }
                }
            } catch (e) { }
        },

        async configureSchedule(regId) {
            try {
                const res = await API.getScanPolicy(regId);
                const p = res.registry_id ? res : { enabled: false, interval_hours: 24, filter_repos: '' };

                const html = `
                    <div style="margin-bottom:12px">
                        <label style="display:flex;align-items:center;gap:8px;font-weight:bold;cursor:pointer">
                            <input type="checkbox" id="sched-enabled" ${p.enabled ? 'checked' : ''}> Enable Periodic Scan
                        </label>
                    </div>
                    <div style="margin-bottom:12px">
                        <label>Run Interval (Hours)</label>
                        <input type="number" id="sched-interval" class="form-control" value="${p.interval_hours || 24}" min="1">
                        <small class="text-secondary">Default is every 24 hours.</small>
                    </div>
                    <div style="margin-bottom:12px">
                        <label>Filter Repositories (Regex)</label>
                        <input type="text" id="sched-filter" class="form-control" value="${escapeHtml(p.filter_repos || '')}" placeholder="e.g. ^prod-.*">
                        <small class="text-secondary">Leave empty to scan all repositories matching the filter every run.</small>
                    </div>
                    <div class="form-actions text-right" style="margin-top:20px">
                        <button type="button" class="btn btn-secondary" onclick="Modal.close()">Cancel</button>
                        <button type="button" class="btn btn-primary" onclick="window.app.saveSchedule(${regId})">Save Configuration</button>
                    </div>
                `;
                Modal.open('Vulnerability Scan Scheduler', html);
            } catch (e) { Toast.error('Failed to load schedule: ' + e.message); }
        },

        async saveSchedule(regId) {
            const enabled = document.getElementById('sched-enabled').checked;
            const interval = parseInt(document.getElementById('sched-interval').value);
            const filter = document.getElementById('sched-filter').value;

            try {
                await API.saveScanPolicy(regId, {
                    registry_id: parseInt(regId),
                    enabled: enabled,
                    interval_hours: interval,
                    filter_repos: filter
                });
                Toast.success('Schedule updated.');
                Modal.close();
            } catch (e) { Toast.error('Update failed: ' + e.message); }
        },

        async toggleReport_unused(regId, repo, tag, elementId) {
            // CSS Injection
            if (!document.getElementById('report-styles')) {
                const s = document.createElement('style');
                s.id = 'report-styles';
                s.textContent = `
                  .report-section { margin-bottom: 32px; animation: slideDown 0.3s ease-out forwards; }
                  .report-title { font-size: 1.1rem; font-weight: 700; margin-bottom: 16px; padding-bottom: 8px; border-bottom: 2px solid; display: flex; align-items: center; gap: 8px; }
                  .report-card { background: #f8fafc; border: 1px solid #e2e8f0; border-radius: 8px; margin-bottom: 16px; overflow: hidden; box-shadow: 0 2px 4px rgba(0,0,0,0.02); }
                  .report-header { background: #f1f5f9; padding: 10px 16px; border-bottom: 1px solid #e2e8f0; font-weight: 600; font-size: 0.9rem; color: #334155; display: flex; justify-content: space-between; }
                  .rep-table { width: 100%; border-collapse: collapse; font-size: 0.85rem; }
                  .rep-table th { text-align: left; padding: 8px 16px; background: #fff; border-bottom: 2px solid #e2e8f0; color: #64748b; font-weight: 600; font-size: 0.75rem; text-transform: uppercase; }
                  .rep-table td { padding: 8px 16px; border-bottom: 1px solid #f1f5f9; background: #fff; color: #334155; vertical-align: top; }
                  .rep-badge { padding: 3px 8px; border-radius: 4px; font-weight: 700; font-size: 0.7rem; text-transform: uppercase; display: inline-block; min-width: 60px; text-align: center; }
                  .rep-critical { background: #fee2e2; color: #991b1b; }
                  .rep-high { background: #ffedd5; color: #9a3412; }
                  .rep-medium { background: #fef9c3; color: #854d0e; }
                  .rep-low { background: #e0f2fe; color: #075985; }
                  .rep-unknown { background: #f1f5f9; color: #64748b; }
                  @keyframes slideDown { from { opacity: 0; transform: translateY(-10px); } to { opacity: 1; transform: translateY(0); } }
                `;
                document.head.appendChild(s);
            }

            const row = document.getElementById(elementId);
            if (!row) return;

            if (row.style.display !== 'none') {
                row.style.display = 'none';
                return;
            }

            row.style.display = 'table-row';
            const container = row.querySelector('.report-content-box');
            container.innerHTML = '<div style="padding:20px;text-align:center;color:#64748b">Loading report...</div>';

            try {
                const res = await API.getScanResult(regId, repo, tag);
                const data = res.data;
                const rawReport = JSON.parse(data.report || '{}');

                // Identify report types
                let reports = {};
                if (rawReport.trivy || rawReport.osv) {
                    reports = rawReport;
                } else {
                    // Check signature roughly
                    // Trivy usually has 'SchemaVersion', 'Results'.
                    // OSV usually has 'results' (lowercase).
                    if (rawReport.results && Array.isArray(rawReport.results) && !rawReport.Results) {
                        reports.osv = rawReport;
                    } else {
                        reports.trivy = rawReport; // Default to Trivy
                    }
                }

                let html = '';

                // --- TRIVY RENDERER ---
                if (reports.trivy && reports.trivy.Results && reports.trivy.Results.length) {
                    let sectionContent = '';
                    reports.trivy.Results.forEach(target => {
                        let body = '';
                        if (target.Vulnerabilities && target.Vulnerabilities.length) {
                            const rows = target.Vulnerabilities.map(v => {
                                const s = (v.Severity || 'unknown').toLowerCase();
                                const c = 'rep-' + (['critical', 'high', 'medium', 'low'].includes(s) ? s : 'unknown');
                                return `<tr>
                                    <td style="font-weight:600">${escapeHtml(v.PkgName)}</td>
                                    <td><span class="rep-badge ${c}">${v.Severity}</span></td>
                                    <td style="font-family:monospace;color:#64748b">${escapeHtml(v.InstalledVersion)}</td>
                                    <td style="font-family:monospace;color:#64748b">${escapeHtml(v.FixedVersion || '-')}</td>
                                    <td><a href="${v.PrimaryURL || '#'}" target="_blank" style="color:#2563eb">${v.VulnerabilityID}</a></td>
                                 </tr>`;
                            }).join('');
                            body = `<table class="rep-table"><thead><tr><th>Package</th><th>Severity</th><th>Installed</th><th>Fixed In</th><th>ID</th></tr></thead><tbody>${rows}</tbody></table>`;
                        } else {
                            body = `<div style="padding:16px;text-align:center;color:#166534;background:#f0fdf4">No vulnerabilities found (Clean)</div>`;
                        }
                        sectionContent += `
                            <div class="report-card">
                                <div class="report-header">
                                    <span>${escapeHtml(target.Target)}</span>
                                    <span style="opacity:0.7">${target.Type || ''}</span>
                                </div>
                                ${body}
                            </div>
                         `;
                    });

                    html += `
                        <div class="report-section">
                            <div class="report-title" style="border-color:#3b82f6;color:#1e40af">
                                🛡️ Trivy Report
                            </div>
                            ${sectionContent}
                        </div>
                    `;
                } else if (reports.trivy) {
                    html += `<div class="report-section"><div class="report-title" style="color:#1e40af;border-color:#3b82f6">🛡️ Trivy Report</div><div class="alert alert-success">No vulnerabilities data found or structure unrecognized.</div></div>`;
                }

                // --- OSV RENDERER ---
                if (reports.osv && reports.osv.results && reports.osv.results.length) {
                    let sectionContent = '';
                    reports.osv.results.forEach((res, idx) => {
                        if (!res.packages) return;
                        res.packages.forEach(pkg => {
                            if (!pkg.vulnerabilities) return;

                            let rows = pkg.vulnerabilities.map(v => {
                                // OSV Severity logic?
                                // Usually v.severity is array. ID is GHSA...
                                const sev = v.database_specific ? (v.database_specific.severity || 'Unknown') : 'Unknown';
                                // Or map header
                                let s = sev.toLowerCase();
                                if (s === 'moderate') s = 'medium';
                                const c = 'rep-' + (['critical', 'high', 'medium', 'low'].includes(s) ? s : 'unknown');

                                return `<tr>
                                    <td style="font-weight:600">${escapeHtml(pkg.package.name)}</td>
                                    <td><span class="rep-badge ${c}">${sev || 'UNK'}</span></td>
                                    <td style="font-family:monospace;color:#64748b">${escapeHtml(pkg.package.version)}</td>
                                    <td><a href="https://osv.dev/vulnerability/${v.id}" target="_blank" style="color:#0ea5e9">${v.id}</a></td>
                                    <td>${escapeHtml(v.summary || '')}</td>
                                 </tr>`;
                            }).join('');

                            if (rows) {
                                sectionContent += `
                                    <div class="report-card">
                                        <div class="report-header">
                                            <span>${escapeHtml(pkg.package.name)} (${pkg.package.version})</span>
                                            <span style="opacity:0.7">Source: ${escapeHtml(res.source ? res.source.path : 'Unknown')}</span>
                                        </div>
                                        <table class="rep-table"><thead><tr><th>Package</th><th>Severity</th><th>Version</th><th>ID</th><th>Summary</th></tr></thead><tbody>${rows}</tbody></table>
                                    </div>
                                 `;
                            }
                        });
                    });

                    if (!sectionContent) sectionContent = `<div style="padding:16px;text-align:center;color:#166534;background:#f0fdf4">No OSV findings.</div>`;

                    html += `
                        <div class="report-section">
                            <div class="report-title" style="border-color:#06b6d4;color:#0e7490">
                                🐞 OSV Report
                            </div>
                            ${sectionContent}
                        </div>
                    `;
                } else if (reports.osv) {
                    html += `<div class="report-section"><div class="report-title" style="color:#0e7490;border-color:#06b6d4">🐞 OSV Report</div><div class="alert alert-success">No OSV findings found.</div></div>`;
                }

                if (!html) html = '<div style="padding:20px;text-align:center">No report data available. Try scanning again.</div>';

                container.innerHTML = html;

                // Apply global package filter
                if (window.app._packageFilter) {
                    const term = window.app._packageFilter.toLowerCase();
                    container.querySelectorAll('.rep-table tbody tr').forEach(row => {
                        row.style.display = row.innerText.toLowerCase().includes(term) ? '' : 'none';
                    });
                }
            } catch (e) {
                container.innerHTML = `<div class="alert alert-danger" style="margin:0">Error loading report: ${e.message}</div>`;
            }
        },

        switchRepTab(e, type) {
            const btn = e.target.closest('.rep-tab');
            const parent = btn.closest('.report-content-box');
            parent.querySelectorAll('.rep-tab').forEach(b => b.classList.remove('active'));
            btn.classList.add('active');
            parent.querySelectorAll('.rep-pane').forEach(p => p.style.display = 'none');
            const pane = parent.querySelector(`.rep-pane-${type}`);
            if (pane) pane.style.display = 'block';
        },

        async toggleReport(regId, repo, tag, elementId) {
            // CSS Injection
            if (!document.getElementById('report-styles')) {
                const s = document.createElement('style');
                s.id = 'report-styles';
                s.textContent = `
                  .report-tabs { display: flex; gap: 0; margin-bottom: 20px; border-bottom: 1px solid #e2e8f0; }
                  .rep-tab { padding: 12px 24px; border: none; background: transparent; cursor: pointer; border-bottom: 2px solid transparent; font-weight: 600; color: #64748b; font-size: 0.95rem; transition: all 0.2s; }
                  .rep-tab.active { border-bottom-color: #3b82f6; color: #3b82f6; background: #eff6ff; }
                  .rep-tab:hover:not(.active) { color: #334155; background: #f8fafc; }
                  .rep-pane { animation: fadeIn 0.3s ease-out; }
                  .report-card { background: #fff; border: 1px solid #e2e8f0; border-radius: 8px; margin-bottom: 16px; overflow: hidden; box-shadow: 0 1px 3px rgba(0,0,0,0.05); }
                  .report-header { background: #f8fafc; padding: 12px 20px; border-bottom: 1px solid #e2e8f0; font-weight: 600; font-size: 0.9rem; color: #334155; display: flex; justify-content: space-between; align-items: center; }
                  .rep-table { width: 100%; border-collapse: collapse; font-size: 0.85rem; }
                  .rep-table th { text-align: left; padding: 10px 20px; background: #fff; border-bottom: 2px solid #e2e8f0; color: #64748b; font-weight: 600; font-size: 0.75rem; text-transform: uppercase; }
                  .rep-table td { padding: 10px 20px; border-bottom: 1px solid #f1f5f9; background: #fff; color: #334155; vertical-align: top; }
                  .rep-badge { padding: 3px 8px; border-radius: 4px; font-weight: 700; font-size: 0.7rem; text-transform: uppercase; display: inline-block; min-width: 60px; text-align: center; }
                  .rep-critical { background: #fee2e2; color: #991b1b; }
                  .rep-high { background: #ffedd5; color: #9a3412; }
                  .rep-medium { background: #fef9c3; color: #854d0e; }
                  .rep-low { background: #e0f2fe; color: #075985; }
                  .rep-unknown { background: #f1f5f9; color: #64748b; }
                  @keyframes fadeIn { from { opacity: 0; } to { opacity: 1; } }
                `;
                document.head.appendChild(s);
            }

            const row = document.getElementById(elementId);
            if (!row) return;

            if (row.style.display !== 'none') {
                row.style.display = 'none';
                return;
            }

            row.style.display = 'table-row';
            const container = row.querySelector('.report-content-box');
            container.innerHTML = '<div style="padding:24px;text-align:center;color:#64748b">Loading report...</div>';

            try {
                const res = await API.getScanResult(regId, repo, tag);
                const data = res.data;
                const rawReport = JSON.parse(data.report || '{}');

                // Identify report types
                let reports = {};
                if (rawReport.trivy || rawReport.osv) {
                    reports = rawReport;
                } else {
                    if (rawReport.results && Array.isArray(rawReport.results) && !rawReport.Results) {
                        reports.osv = rawReport;
                    } else {
                        reports.trivy = rawReport; // Default
                    }
                }

                // Prepare Content Generators
                const renderTrivyContent = (rep) => {
                    if (!rep || !rep.Results || !rep.Results.length) return `<div class="alert alert-success">No vulnerabilities found (Clean)</div>`;
                    let content = '';
                    rep.Results.forEach(target => {
                        let body = '';
                        if (target.Vulnerabilities && target.Vulnerabilities.length) {
                            const rows = target.Vulnerabilities.map(v => {
                                const s = (v.Severity || 'unknown').toLowerCase();
                                const c = 'rep-' + (['critical', 'high', 'medium', 'low'].includes(s) ? s : 'unknown');
                                return `<tr>
                                    <td style="font-weight:600">${escapeHtml(v.PkgName)}</td>
                                    <td><span class="rep-badge ${c}">${v.Severity}</span></td>
                                    <td style="font-family:monospace;color:#64748b">${escapeHtml(v.InstalledVersion)}</td>
                                    <td style="font-family:monospace;color:#64748b">${escapeHtml(v.FixedVersion || '-')}</td>
                                    <td><a href="${v.PrimaryURL || '#'}" target="_blank" style="color:#2563eb;text-decoration:none">ID: ${v.VulnerabilityID}</a></td>
                                 </tr>`;
                            }).join('');
                            body = `<table class="rep-table"><thead><tr><th>Package</th><th>Severity</th><th>Installed</th><th>Fixed In</th><th>Vulnerability ID</th></tr></thead><tbody>${rows}</tbody></table>`;
                        } else {
                            body = `<div style="padding:20px;text-align:center;color:#166534;background:#f0fdf4">No vulnerabilities found in this target.</div>`;
                        }
                        content += `
                            <div class="report-card">
                                <div class="report-header">
                                    <span>${escapeHtml(target.Target)}</span>
                                    <span style="opacity:0.7;font-size:0.8rem;background:#e2e8f0;padding:2px 6px;border-radius:4px">${target.Type || 'Unknown'}</span>
                                </div>
                                ${body}
                            </div>
                         `;
                    });
                    return content;
                };

                const renderOSVContent = (rep) => {
                    if (!rep || !rep.results || !rep.results.length) return `<div class="alert alert-info" style="background:#eff6ff;color:#1e40af;border-color:#dbeafe">No OSV scan data available or no findings.</div>`;
                    let content = '';
                    let findingsCount = 0;
                    rep.results.forEach(res => {
                        if (!res.packages) return;
                        res.packages.forEach(pkg => {
                            if (!pkg.vulnerabilities) return;
                            findingsCount++;
                            let rows = pkg.vulnerabilities.map(v => {
                                const sev = v.database_specific ? (v.database_specific.severity || 'Unknown') : 'Unknown';
                                let s = sev.toLowerCase();
                                if (s === 'moderate') s = 'medium';
                                const c = 'rep-' + (['critical', 'high', 'medium', 'low'].includes(s) ? s : 'unknown');
                                return `<tr>
                                    <td style="font-weight:600">${escapeHtml(pkg.package.name)}</td>
                                    <td><span class="rep-badge ${c}">${sev || 'UNK'}</span></td>
                                    <td style="font-family:monospace;color:#64748b">${escapeHtml(pkg.package.version)}</td>
                                    <td><a href="https://osv.dev/vulnerability/${v.id}" target="_blank" style="color:#0ea5e9;text-decoration:none">${v.id}</a></td>
                                    <td>${escapeHtml(v.summary || '')}</td>
                                 </tr>`;
                            }).join('');

                            if (rows) {
                                content += `
                                    <div class="report-card">
                                        <div class="report-header">
                                            <span>${escapeHtml(pkg.package.name)} (${pkg.package.version})</span>
                                            <span style="opacity:0.7">Source: ${escapeHtml(res.source ? res.source.path : 'Unknown')}</span>
                                        </div>
                                        <table class="rep-table"><thead><tr><th>Package</th><th>Severity</th><th>Version</th><th>ID</th><th>Summary</th></tr></thead><tbody>${rows}</tbody></table>
                                    </div>
                                 `;
                            }
                        });
                    });
                    if (!findingsCount) return `<div class="alert alert-success">No OSV findings found.</div>`;
                    return content || `<div class="alert alert-success">No findings.</div>`;
                };

                const trivyHTML = renderTrivyContent(reports.trivy);
                const osvHTML = renderOSVContent(reports.osv);

                // Initial active tab logic
                let activeTab = 'trivy';
                if (!reports.trivy && reports.osv) activeTab = 'osv';

                const tabsHtml = `
                    <div class="report-tabs">
                        <button class="rep-tab ${activeTab === 'trivy' ? 'active' : ''}" onclick="window.app.switchRepTab(event, 'trivy')">
                            🛡️ Trivy Report
                            ${reports.trivy ? '<span style="font-size:0.75em;opacity:0.7;margin-left:4px">✓</span>' : ''}
                        </button>
                        <button class="rep-tab ${activeTab === 'osv' ? 'active' : ''}" onclick="window.app.switchRepTab(event, 'osv')">
                            🐞 OSV Report
                            ${reports.osv ? '<span style="font-size:0.75em;opacity:0.7;margin-left:4px">✓</span>' : ''}
                        </button>
                    </div>
                    <div class="rep-pane rep-pane-trivy" style="display:${activeTab === 'trivy' ? 'block' : 'none'}">
                        ${!reports.trivy ? '<div style="padding:32px;text-align:center;color:#64748b;font-style:italic;background:#f8fafc;border-radius:8px;border:1px dashed #cbd5e0">Trivy scan has not been run or no data.</div><div style="text-align:center;margin-top:16px"><button class="btn btn-sm btn-outline-primary" onclick="window.app.triggerScan(this.dataset.reg, this.dataset.repo, this.dataset.tag, \'\', \'trivy\')" data-reg="${regId}" data-repo="${escapeHtml(repo)}" data-tag="${escapeHtml(tag)}">Run Trivy Scan</button></div>' : trivyHTML}
                    </div>
                    <div class="rep-pane rep-pane-osv" style="display:${activeTab === 'osv' ? 'block' : 'none'}">
                        ${!reports.osv ? '<div style="padding:32px;text-align:center;color:#64748b;font-style:italic;background:#f8fafc;border-radius:8px;border:1px dashed #cbd5e0">OSV scan has not been run.</div><div style="text-align:center;margin-top:16px"><button class="btn btn-sm btn-outline-info" onclick="window.app.triggerScan(this.dataset.reg, this.dataset.repo, this.dataset.tag, \'\', \'osv\')" data-reg="${regId}" data-repo="${escapeHtml(repo)}" data-tag="${escapeHtml(tag)}">Run OSV Scan</button></div>' : osvHTML}
                    </div>
                `;

                container.innerHTML = tabsHtml;

                // Filter applies to active pane
                if (window.app._packageFilter) {
                    const term = window.app._packageFilter.toLowerCase();
                    container.querySelectorAll('.rep-table tbody tr').forEach(row => {
                        row.style.display = row.innerText.toLowerCase().includes(term) ? '' : 'none';
                    });
                }
            } catch (e) {
                container.innerHTML = `<div class="alert alert-danger" style="margin:0">Error loading report: ${e.message}</div>`;
            }
        },

        // Registry CRUD
        showAddRegistry() {
            Modal.open('Add Registry', `<form id="add-registry-form" onsubmit="event.preventDefault();window.app.addRegistry()"><div class="form-group"><label class="form-label">Name</label><input type="text" id="reg-name" class="form-input" placeholder="My Registry" required></div><div class="form-group"><label class="form-label">URL</label><input type="text" id="reg-url" class="form-input" placeholder="https://registry.example.com" required><div class="form-hint">Full URL with protocol</div></div><div class="form-row"><div class="form-group"><label class="form-label">Username</label><input type="text" id="reg-username" class="form-input"></div><div class="form-group"><label class="form-label">Password</label><input type="password" id="reg-password" class="form-input"></div></div><div class="form-group"><label class="form-check"><input type="checkbox" id="reg-insecure"><span class="form-check-label">Allow insecure connection</span></label></div><div style="display:flex;gap:12px;justify-content:flex-end"><button type="button" class="btn btn-ghost" onclick="Modal.close()">Cancel</button><button type="submit" class="btn btn-primary">Add Registry</button></div></form>`);
        },
        async addRegistry() {
            try { await API.createRegistry({ name: document.getElementById('reg-name').value, url: document.getElementById('reg-url').value, username: document.getElementById('reg-username').value, password: document.getElementById('reg-password').value, insecure: document.getElementById('reg-insecure').checked }); Modal.close(); Toast.success('Registry added!'); this.navigate(this.currentPage); } catch (e) { Toast.error(e.message); }
        },
        async showEditRegistry(id) {
            try {
                const res = await API.getRegistries(); const r = (res.data || []).find(x => x.id === id); if (!r) return Toast.error('Not found');
                Modal.open('Edit Registry', `<form onsubmit="event.preventDefault();window.app.updateRegistry(${id})"><div class="form-group"><label class="form-label">Name</label><input type="text" id="edit-reg-name" class="form-input" value="${escapeHtml(r.name)}" required></div><div class="form-group"><label class="form-label">URL</label><input type="text" id="edit-reg-url" class="form-input" value="${escapeHtml(r.url)}" required></div><div class="form-row"><div class="form-group"><label class="form-label">Username</label><input type="text" id="edit-reg-username" class="form-input" value="${escapeHtml(r.username || '')}"></div><div class="form-group"><label class="form-label">Password</label><input type="password" id="edit-reg-password" class="form-input" value="${escapeHtml(r.password || '')}"></div></div><div class="form-group"><label class="form-check"><input type="checkbox" id="edit-reg-insecure" ${r.insecure ? 'checked' : ''}><span class="form-check-label">Allow insecure</span></label></div><div style="display:flex;gap:12px;justify-content:flex-end"><button type="button" class="btn btn-ghost" onclick="Modal.close()">Cancel</button><button type="submit" class="btn btn-primary">Save</button></div></form>`);
            } catch (e) { Toast.error(e.message); }
        },
        async updateRegistry(id) { try { await API.updateRegistry(id, { name: document.getElementById('edit-reg-name').value, url: document.getElementById('edit-reg-url').value, username: document.getElementById('edit-reg-username').value, password: document.getElementById('edit-reg-password').value, insecure: document.getElementById('edit-reg-insecure').checked }); Modal.close(); Toast.success('Updated!'); renderRegistries(); } catch (e) { Toast.error(e.message); } },
        async deleteRegistry(id, name) { if (!(await Confirm.show('Delete', 'Delete "' + name + '"?'))) return; try { await API.deleteRegistry(id); Toast.success('Deleted!'); renderRegistries(); } catch (e) { Toast.error(e.message); } },
        async testRegistry(id) { Toast.info('Testing...'); try { const r = await API.testRegistry(id); Toast.success('Connected! ' + r.data.latency_ms + 'ms'); } catch (e) { Toast.error(e.message); } },

        // Image/Tag browsing
        viewRegistryImages(id) { this._selectedRegistry = id; this.navigate('images'); },
        async loadImages(regId) {
            if (!regId) return; this._selectedRegistry = regId;
            const d = document.getElementById('images-content'); if (!d) return; d.innerHTML = showLoading();
            try {
                const res = await API.getRepositories(regId); const repos = res.data || [];
                if (!repos.length) { d.innerHTML = showEmpty('<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>', 'No images', 'This registry has no images yet.'); return; }
                d.innerHTML = `<div class="section-header"><h2>Repositories (${repos.length})</h2></div><div class="image-list">${repos.map((r, i) => `<div class="image-item" style="animation-delay:${i * 0.04}s" onclick="window.app.viewTags(${regId},'${escapeHtml(r.name)}')"><div class="image-item-info"><div class="image-item-icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg></div><div><div class="image-item-name">${escapeHtml(r.name)}</div><div class="image-item-meta">${r.tag_count || 0} tags</div></div></div><div class="image-item-right"><span class="badge badge-info">${r.tag_count || 0} tags</span><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="9 18 15 12 9 6"/></svg></div></div>`).join('')}</div>`;
            } catch (e) { d.innerHTML = showEmpty('<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/></svg>', 'Error', e.message); }
        },
        async viewTags(regId, repo) {
            const d = document.getElementById('images-content'); if (!d) return; d.innerHTML = showLoading();
            try {
                const res = await API.getTags(regId, repo); const tags = res.data || [];
                d.innerHTML = `<div class="tags-header"><button class="back-btn" onclick="window.app.loadImages(${regId})"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="19" y1="12" x2="5" y2="12"/><polyline points="12 19 5 12 12 5"/></svg> Back</button></div><div class="section-header"><h2><span style="color:var(--text-muted)">Tags for</span> ${escapeHtml(repo)} <span class="badge badge-info" style="margin-left:8px;font-size:0.7rem">${tags.length}</span></h2></div><div id="tags-list">${tags.map((t, i) => `<div class="tag-item" style="animation-delay:${i * 0.04}s"><div class="tag-item-info"><div class="tag-icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M20.59 13.41l-7.17 7.17a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z"/><line x1="7" y1="7" x2="7.01" y2="7"/></svg></div><div><div class="tag-name">${escapeHtml(t.name)}</div>${t.digest ? '<div class="tag-digest">' + truncateDigest(t.digest) + '</div>' : ''}</div></div><div class="tag-actions"><button class="btn btn-sm btn-ghost" onclick="window.app.viewManifest(${regId},'${escapeHtml(repo)}','${escapeHtml(t.name)}')">🔍 Inspect</button><button class="btn btn-sm btn-danger" onclick="window.app.deleteImageTag(${regId},'${escapeHtml(repo)}','${escapeHtml(t.name)}')">Delete</button></div></div>`).join('')}</div>`;
            } catch (e) { d.innerHTML = showEmpty('<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/></svg>', 'Error', e.message); }
        },
        async viewManifest(regId, repo, tag) {
            Toast.info('Loading manifest...'); try {
                const res = await API.getManifest(regId, repo, tag); const m = res.data;
                Modal.open(`${repo}:${tag}`, `<div class="manifest-viewer"><div class="manifest-section"><div class="manifest-section-title">General</div><div class="manifest-detail"><span class="manifest-detail-label">Schema</span><span class="manifest-detail-value">${m.schemaVersion}</span></div><div class="manifest-detail"><span class="manifest-detail-label">Media Type</span><span class="manifest-detail-value">${escapeHtml(m.mediaType || 'N/A')}</span></div><div class="manifest-detail"><span class="manifest-detail-label">Digest</span><span class="manifest-detail-value" title="${escapeHtml(m.digest || '')}">${truncateDigest(m.digest || '', 24)}</span></div><div class="manifest-detail"><span class="manifest-detail-label">Total Size</span><span class="manifest-detail-value">${formatBytes(m.totalSize)}</span></div></div>${m.config ? '<div class="manifest-section"><div class="manifest-section-title">Config</div><div class="manifest-detail"><span class="manifest-detail-label">Type</span><span class="manifest-detail-value">' + escapeHtml(m.config.mediaType) + '</span></div><div class="manifest-detail"><span class="manifest-detail-label">Size</span><span class="manifest-detail-value">' + formatBytes(m.config.size) + '</span></div></div>' : ''}${m.layers && m.layers.length ? '<div class="manifest-section"><div class="manifest-section-title">Layers (' + m.layers.length + ')</div>' + m.layers.map(l => '<div class="layer-item"><span class="layer-digest">' + truncateDigest(l.digest, 20) + '</span><span class="layer-size">' + formatBytes(l.size) + '</span></div>').join('') + '</div>' : ''}</div>`);
            } catch (e) { Toast.error(e.message); }
        },
        async deleteImageTag(regId, repo, tag) { if (!(await Confirm.show('Delete Tag', 'Delete ' + repo + ':' + tag + '?'))) return; try { await API.deleteTag(regId, repo, tag); Toast.success('Deleted!'); this.viewTags(regId, repo); } catch (e) { Toast.error(e.message); } },

        // Storage
        switchStorageTab(type) { document.querySelectorAll('.storage-tab').forEach(t => t.classList.toggle('active', t.dataset.tab === type)); document.querySelectorAll('.storage-form-section').forEach(s => s.classList.remove('active')); const el = document.getElementById('storage-' + type); if (el) el.classList.add('active'); },
        _getStorageData() {
            const type = (document.querySelector('.storage-tab.active') || {}).dataset || 'local';
            const t = typeof type === 'object' ? type.tab : type; const d = { type: t };
            if (t === 'local') d.local_path = document.getElementById('local-path').value;
            else if (t === 's3') { d.s3_endpoint = document.getElementById('s3-endpoint').value; d.s3_region = document.getElementById('s3-region').value; d.s3_bucket = document.getElementById('s3-bucket').value; d.s3_access_key = document.getElementById('s3-access-key').value; d.s3_secret_key = document.getElementById('s3-secret-key').value; d.s3_use_ssl = document.getElementById('s3-use-ssl').checked; }
            else if (t === 'sftp') { d.sftp_host = document.getElementById('sftp-host').value; d.sftp_port = parseInt(document.getElementById('sftp-port').value) || 22; d.sftp_user = document.getElementById('sftp-user').value; d.sftp_password = document.getElementById('sftp-password').value; d.sftp_path = document.getElementById('sftp-path').value; d.sftp_private_key = document.getElementById('sftp-key').value; }
            return d;
        },
        async saveStorage() { try { const res = await API.saveStorageConfig(this._getStorageData()); Toast.success(res.message || 'Saved!'); } catch (e) { Toast.error(e.message); } },
        async testStorage() { Toast.info('Testing...'); try { const r = await API.testStorageConnection(this._getStorageData()); r.data.status === 'warning' ? Toast.warning(r.data.message) : Toast.success(r.data.message); } catch (e) { Toast.error(e.message); } },

        // Embedded registry control
        async embeddedAction(action) {
            Toast.info(action === 'restart' ? 'Restarting...' : action === 'stop' ? 'Stopping...' : 'Starting...');
            try { const fn = { restart: API.restartRegistry, stop: API.stopRegistry, start: API.startRegistry }[action]; const r = await fn(); Toast.success(r.message || 'Done!'); setTimeout(() => this.navigate(this.currentPage), 1500); } catch (e) { Toast.error(e.message); }
        },
        async showRegistryLogs() {
            try { const r = await API.getRegistryLogs(); Modal.open('Registry Logs', '<pre style="background:var(--bg-primary);padding:16px;border-radius:var(--radius-md);font-size:0.8rem;color:var(--text-secondary);max-height:500px;overflow:auto;white-space:pre-wrap;word-break:break-all">' + escapeHtml(r.data.logs || 'No logs') + '</pre>'); } catch (e) { Toast.error(e.message); }
        },

        // Vulnerability Report Methods
        async loadVulnerabilities(regId) {
            this._selectedRegistry = regId;
            this._allVulns = []; // Store for filtering
            const area = document.getElementById('vuln-report-content');
            if (!area) return;
            area.innerHTML = showLoading();

            try {
                const res = await API.listVulnerabilities(regId);
                const vulns = res.data || [];
                this._allVulns = vulns;

                if (!vulns.length) {
                    area.innerHTML = showEmpty('🛡️', 'No vulnerabilities found', 'Scan some images first to see vulnerability reports.');
                    return;
                }

                this.renderVulnerabilitiesTable(vulns);
            } catch (e) {
                area.innerHTML = showEmpty('⚠️', 'Error', e.message);
            }
        },

        renderVulnerabilitiesTable(vulns) {
            const area = document.getElementById('vuln-report-content');
            if (!area) return;

            const severityColor = (sev) => {
                const s = sev.toUpperCase();
                if (s === 'CRITICAL') return '#dc2626';
                if (s === 'HIGH') return '#ea580c';
                if (s === 'MEDIUM') return '#d97706';
                if (s === 'LOW') return '#0284c7';
                return '#64748b';
            };

            const severityBadge = (sev) => {
                const color = severityColor(sev);
                return `<span class="badge" style="background:${color}20;color:${color};padding:4px 8px;font-size:0.75rem;font-weight:600">${sev}</span>`;
            };

            const stats = {
                total: vulns.length,
                critical: vulns.filter(v => v.severity.toUpperCase() === 'CRITICAL').length,
                high: vulns.filter(v => v.severity.toUpperCase() === 'HIGH').length,
                medium: vulns.filter(v => v.severity.toUpperCase() === 'MEDIUM').length,
                low: vulns.filter(v => v.severity.toUpperCase() === 'LOW').length,
            };

            let html = `
                <div class="card fade-in" style="margin-bottom:24px;padding:20px">
                    <h3 style="margin:0 0 16px 0">📊 Summary</h3>
                    <div style="display:flex;gap:16px;flex-wrap:wrap">
                        <div class="registry-stat"><span class="registry-stat-value">${stats.total}</span><span class="registry-stat-label">Total</span></div>
                        <div class="registry-stat"><span class="registry-stat-value" style="color:#dc2626">${stats.critical}</span><span class="registry-stat-label">Critical</span></div>
                        <div class="registry-stat"><span class="registry-stat-value" style="color:#ea580c">${stats.high}</span><span class="registry-stat-label">High</span></div>
                        <div class="registry-stat"><span class="registry-stat-value" style="color:#d97706">${stats.medium}</span><span class="registry-stat-label">Medium</span></div>
                        <div class="registry-stat"><span class="registry-stat-value" style="color:#0284c7">${stats.low}</span><span class="registry-stat-label">Low</span></div>
                    </div>
                </div>

                <div class="card fade-in">
                    <div style="overflow-x:auto">
                        <table style="width:100%;border-collapse:collapse;font-size:0.875rem">
                            <thead>
                                <tr style="border-bottom:2px solid var(--border);color:var(--text-muted);text-align:left">
                                    <th style="padding:12px 16px;font-weight:600">CVE/ID</th>
                                    <th style="padding:12px 16px;font-weight:600">Severity</th>
                                    <th style="padding:12px 16px;font-weight:600">Package</th>
                                    <th style="padding:12px 16px;font-weight:600">Version</th>
                                    <th style="padding:12px 16px;font-weight:600">Fixed In</th>
                                    <th style="padding:12px 16px;font-weight:600">Image</th>
                                    <th style="padding:12px 16px;font-weight:600">Tag</th>
                                    <th style="padding:12px 16px;font-weight:600">Scanner</th>
                                </tr>
                            </thead>
                            <tbody>
                                ${vulns.map((v, idx) => `
                                    <tr class="vuln-row" data-id="${v.id}" data-severity="${v.severity}" data-repo="${v.repository}" data-tag="${v.tag}" data-scanner="${v.scanner}" style="border-bottom:1px solid var(--border);transition:background 0.2s" onmouseover="this.style.background='var(--bg-secondary)'" onmouseout="this.style.background=''">
                                        <td style="padding:12px 16px">
                                            <div style="font-weight:600;color:var(--text-accent)">${escapeHtml(v.id)}</div>
                                            ${v.description ? `<div style="font-size:0.75rem;color:var(--text-muted);margin-top:4px">${escapeHtml(v.description.substring(0, 100))}${v.description.length > 100 ? '...' : ''}</div>` : ''}
                                        </td>
                                        <td style="padding:12px 16px">${severityBadge(v.severity)}</td>
                                        <td style="padding:12px 16px;font-family:monospace;font-size:0.8rem">${escapeHtml(v.package)}</td>
                                        <td style="padding:12px 16px;font-family:monospace;font-size:0.8rem;color:var(--text-muted)">${escapeHtml(v.version)}</td>
                                        <td style="padding:12px 16px;font-family:monospace;font-size:0.8rem">${v.fixed_version ? '<span style="color:#10b981">' + escapeHtml(v.fixed_version) + '</span>' : '<span style="color:var(--text-muted)">-</span>'}</td>
                                        <td style="padding:12px 16px">${escapeHtml(v.repository)}</td>
                                        <td style="padding:12px 16px"><span class="badge badge-secondary">${escapeHtml(v.tag)}</span></td>
                                        <td style="padding:12px 16px"><span class="badge ${v.scanner === 'Trivy' ? 'badge-primary' : 'badge-info'}" style="font-size:0.7rem">${escapeHtml(v.scanner)}</span></td>
                                    </tr>
                                `).join('')}
                            </tbody>
                        </table>
                    </div>
                </div>
            `;

            area.innerHTML = html;
        },

        filterVulnerabilities() {
            const searchTerm = (document.getElementById('vuln-search')?.value || '').toLowerCase();
            const filterRepo = (document.getElementById('vuln-filter-repo')?.value || '').toLowerCase();
            const filterTag = (document.getElementById('vuln-filter-tag')?.value || '').toLowerCase();
            const filterSeverity = (document.getElementById('vuln-filter-severity')?.value || '').toUpperCase();
            const filterScanner = (document.getElementById('vuln-filter-scanner')?.value || '');

            const filtered = this._allVulns.filter(v => {
                // Search in ID, package, and description
                const matchSearch = !searchTerm || 
                    v.id.toLowerCase().includes(searchTerm) || 
                    v.package.toLowerCase().includes(searchTerm) ||
                    (v.description && v.description.toLowerCase().includes(searchTerm));

                const matchRepo = !filterRepo || v.repository.toLowerCase().includes(filterRepo);
                const matchTag = !filterTag || v.tag.toLowerCase().includes(filterTag);
                const matchSeverity = !filterSeverity || v.severity.toUpperCase() === filterSeverity;
                const matchScanner = !filterScanner || v.scanner === filterScanner;

                return matchSearch && matchRepo && matchTag && matchSeverity && matchScanner;
            });

            this.renderVulnerabilitiesTable(filtered);
        },
    };

    window.app = app; window.Modal = Modal; window.Toast = Toast;
    document.addEventListener('DOMContentLoaded', () => app.init());
})();
