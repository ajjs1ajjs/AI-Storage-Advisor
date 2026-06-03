import './app.css';
import { EventsOn } from '../wailsjs/runtime/runtime.js';
import {
    GetTheme,
    SaveTheme,
    GetRecentScans,
    GetSSHHosts,
    AddSSHHost,
    EditSSHHost,
    DeleteSSHHost,
    SaveScanRules,
    GetScanRules,
    SaveAIProvider,
    GetAIProviders,
    TestAIProviderConnection,
    GetAIModels,
    StartScan,
    CancelScan,
    DryRunCleanup,
    SafeDeleteFiles,
    GenerateAIRecommendation,
    GetStorageForecast,
    BrowseFolder
} from '../wailsjs/go/main/App.js';

// Application State
let state = {
    theme: 'dark',
    currentTab: 'dashboard',
    scannedResults: null,
    savedSSHHosts: [],
    savedAIProviders: [],
    activeAIProvider: null,
    scanRules: [],
    deletePathsQueue: [],
    editingSSHId: null
};

// Initialisations on DOM Loaded
document.addEventListener('DOMContentLoaded', () => {
    initApp();
    setupEventListeners();
    setupWailsEvents();
});

async function initApp() {
    // 1. Theme configuration
    state.theme = await GetTheme();
    applyTheme(state.theme);

    // 2. Load recent scans on dashboard
    loadRecentScans();

    // 3. Load configurations in background
    loadAIProviders();
    loadScanRules();
    loadSSHHosts();
}

function applyTheme(theme) {
    const body = document.body;
    const themeBtn = document.getElementById('btn-theme-toggle');
    const themeText = themeBtn.querySelector('.theme-text');
    const themeIcon = themeBtn.querySelector('.theme-icon');

    if (theme === 'light') {
        body.classList.remove('dark-theme');
        body.classList.add('light-theme');
        themeText.innerText = 'Темна тема';
        themeIcon.innerText = '🌙';
    } else {
        body.classList.remove('light-theme');
        body.classList.add('dark-theme');
        themeText.innerText = 'Світла тема';
        themeIcon.innerText = '☀️';
    }
}

// ----------------------------------------------------
// Navigation Tab Switches
// ----------------------------------------------------
function switchTab(tabId) {
    state.currentTab = tabId;
    
    // Toggle Nav Buttons class
    document.querySelectorAll('.nav-btn').forEach(btn => {
        btn.classList.remove('active');
    });
    
    // Toggle Content section
    document.querySelectorAll('.tab-content').forEach(section => {
        section.classList.add('hidden');
    });

    if (tabId === 'dashboard') {
        document.getElementById('btn-nav-dashboard').classList.add('active');
        document.getElementById('tab-dashboard').classList.remove('hidden');
        loadRecentScans();
    } else if (tabId === 'settings') {
        document.getElementById('btn-nav-settings').classList.add('active');
        document.getElementById('tab-settings').classList.remove('hidden');
        renderSSHHostsTable();
    }
}

// Settings sub-sidebar switcher
function switchSettingsForm(formId, clickedItem) {
    document.querySelectorAll('.sub-nav-item').forEach(item => {
        item.classList.remove('active');
    });
    clickedItem.classList.add('active');

    document.querySelectorAll('.settings-form-block').forEach(form => {
        form.classList.remove('active');
    });
    document.getElementById(formId).classList.add('active');
}

// ----------------------------------------------------
// Event Setup
// ----------------------------------------------------
function setupEventListeners() {
    // Nav menu links
    document.getElementById('btn-nav-dashboard').addEventListener('click', () => switchTab('dashboard'));
    document.getElementById('btn-nav-settings').addEventListener('click', () => switchTab('settings'));

    // Theme toggle button
    document.getElementById('btn-theme-toggle').addEventListener('click', () => {
        const newTheme = state.theme === 'dark' ? 'light' : 'dark';
        state.theme = newTheme;
        SaveTheme(newTheme);
        applyTheme(newTheme);
    });

    // Settings sub-nav items
    document.querySelectorAll('.sub-nav-item').forEach(item => {
        item.addEventListener('click', (e) => {
            const formTarget = e.target.getAttribute('data-form');
            switchSettingsForm(formTarget, e.target);
        });
    });

    // Connection selector change
    document.getElementById('select-conn-type').addEventListener('change', (e) => {
        onConnectionTypeChanged(e.target.value);
    });

    // Browse Folder
    document.getElementById('btn-browse-folder').addEventListener('click', async () => {
        try {
            const folder = await BrowseFolder();
            if (folder) {
                document.getElementById('input-scan-path').value = folder;
            }
        } catch (err) {
            console.error("Browse directory failed:", err);
        }
    });

    // Start Scan button click
    document.getElementById('btn-toggle-scan').addEventListener('click', toggleScan);

    // AI recommendation button
    document.getElementById('btn-generate-ai').addEventListener('click', generateAIRecommendation);

    // Bulk deletion AI click
    document.getElementById('btn-bulk-delete-ai').addEventListener('click', triggerAIRecommendationsCleanup);

    // Accordion tabs switching
    document.querySelectorAll('.accordion-tab').forEach(tab => {
        tab.addEventListener('click', (e) => {
            document.querySelectorAll('.accordion-tab').forEach(t => t.classList.remove('active'));
            document.querySelectorAll('.accordion-panel').forEach(p => p.classList.remove('active'));

            e.target.classList.add('active');
            const paneId = e.target.getAttribute('data-target');
            document.getElementById(paneId).classList.add('active');
        });
    });

    // Settings AI Forms hooks
    document.getElementById('select-ai-provider-type').addEventListener('change', (e) => {
        renderAIProviderFormFields(e.target.value);
    });
    document.getElementById('btn-fetch-models').addEventListener('click', fetchAIModelsList);
    document.getElementById('btn-test-ai').addEventListener('click', testAIProviderConnection);
    document.getElementById('btn-save-ai').addEventListener('click', saveAIProviderConfiguration);

    // Rules saves
    document.getElementById('btn-save-rules').addEventListener('click', saveRulesConfiguration);

    // SSH Hosts CRUD clicks
    document.getElementById('btn-add-ssh').addEventListener('click', openSSHHostEditorAdd);
    document.getElementById('btn-ssh-cancel').addEventListener('click', closeSSHHostEditor);
    document.getElementById('btn-ssh-save').addEventListener('click', saveSSHHostDetails);
    document.getElementById('select-ssh-editor-auth').addEventListener('change', (e) => {
        onSSHAuthTypeChanged(e.target.value);
    });

    // SSH active dropdown loader change
    document.getElementById('select-ssh-server').addEventListener('change', onSSHActiveDropdownChanged);

    // Deletion Modal cancellations/confirms
    document.getElementById('btn-delete-cancel').addEventListener('click', () => {
        document.getElementById('modal-delete-confirm').classList.add('hidden');
    });
    document.getElementById('btn-delete-execute').addEventListener('click', executeDeletionsQueue);
}

// ----------------------------------------------------
// SSH server list credentials change
// ----------------------------------------------------
function onConnectionTypeChanged(type) {
    const browseBtn = document.getElementById('btn-browse-folder');
    const sshPicker = document.getElementById('ssh-server-picker-group');
    const sshCreds = document.getElementById('ssh-creds-container');
    const scanPathInput = document.getElementById('input-scan-path');
    const lblScanPath = document.getElementById('lbl-scan-path');

    if (type === 'Local Scan') {
        browseBtn.classList.remove('hidden');
        sshPicker.classList.add('hidden');
        sshCreds.classList.add('hidden');
        scanPathInput.value = '.';
        lblScanPath.innerText = 'Шлях сканування:';
    } else if (type === 'SSH Remote Linux') {
        browseBtn.classList.add('hidden');
        sshPicker.classList.remove('hidden');
        sshCreds.classList.remove('hidden');
        scanPathInput.value = 'Автоматичний пошук';
        lblScanPath.innerText = 'Віддалена папка:';
        loadSSHHostsDropdown();
    } else if (type === 'Network Share (UNC)') {
        browseBtn.classList.remove('hidden');
        sshPicker.classList.add('hidden');
        sshCreds.classList.add('hidden');
        scanPathInput.value = '\\\\server\\share';
        lblScanPath.innerText = 'Шлях мережевої папки:';
    }
}

function onSSHActiveDropdownChanged() {
    const hostID = document.getElementById('select-ssh-server').value;
    const hostInput = document.getElementById('input-ssh-host');
    const userInput = document.getElementById('input-ssh-user');
    const passInput = document.getElementById('input-ssh-pass');

    if (!hostID) {
        hostInput.value = '';
        userInput.value = '';
        passInput.value = '';
        return;
    }

    const host = state.savedSSHHosts.find(h => h.id == hostID);
    if (host) {
        hostInput.value = host.host;
        userInput.value = host.username;
        if (host.auth_type === 'password') {
            passInput.value = host.credentials;
        } else {
            passInput.value = '';
        }
    }
}

// ----------------------------------------------------
// Local & Remote Scanning Flow
// ----------------------------------------------------
let isScanning = false;

function toggleScan() {
    const scanBtn = document.getElementById('btn-toggle-scan');
    const progressCard = document.getElementById('scan-progress-card');

    if (isScanning) {
        CancelScan();
        scanBtn.innerText = '🔍 Запустити сканування';
        progressCard.classList.add('hidden');
        isScanning = false;
        return;
    }

    const connType = document.getElementById('select-conn-type').value;
    const scanPath = document.getElementById('input-scan-path').value.trim();

    let hostID = 0;
    if (connType === 'SSH Remote Linux') {
        const hostDropdown = document.getElementById('select-ssh-server');
        hostID = parseInt(hostDropdown.value) || 0;

        const hostVal = document.getElementById('input-ssh-host').value.trim();
        const userVal = document.getElementById('input-ssh-user').value.trim();
        if (!hostVal || !userVal) {
            alert('Помилка: Вкажіть адресу та користувача для SSH!');
            return;
        }
    }

    isScanning = true;
    scanBtn.innerText = '🛑 Зупинити сканування';
    progressCard.classList.remove('hidden');
    document.getElementById('scan-status-text').innerText = 'Запуск сканування...';
    document.getElementById('scan-stats-text').innerText = 'Файлів: 0 | Об\'єм: 0 B';

    // Start background scanner via Wails Go call
    StartScan(connType, hostID, scanPath);
}

// Setup background events streaming
function setupWailsEvents() {
    EventsOn('scan:progress', data => {
        document.getElementById('scan-status-text').innerText = `Сканування: ${data.status}`;
        document.getElementById('scan-stats-text').innerText = `Файлів: ${data.files_scanned} | Об'єм: ${formatBytes(data.total_size)}`;
    });

    EventsOn('scan:finished', data => {
        isScanning = false;
        document.getElementById('btn-toggle-scan').innerText = '🔍 Запустити сканування';
        document.getElementById('scan-progress-card').classList.add('hidden');

        if (data.error) {
            alert(`Помилка сканування: ${data.error}`);
            return;
        }

        if (data.cancelled) {
            alert('Сканування скасовано користувачем.');
            return;
        }

        // Successfully loaded results
        renderScanResults(data);
    });

    EventsOn('delete:progress', data => {
        const idx = data.current_index;
        const total = data.total_files;
        const file = data.current_file;
        
        document.getElementById('delete-progress-file-text').innerText = `Видалення (${idx + 1}/${total}): .../${file.split(/[\\/]/).pop()}`;
        const percent = ((idx + 1) / total) * 100;
        document.getElementById('delete-progress-fill').style.width = `${percent}%`;
    });

    EventsOn('delete:finished', data => {
        document.getElementById('delete-progress-block').classList.add('add-hide', 'hidden');
        document.getElementById('modal-delete-confirm').classList.add('hidden');
        
        // Re-enable deletion controls
        document.getElementById('btn-delete-execute').removeAttribute('disabled');
        document.getElementById('btn-delete-cancel').removeAttribute('disabled');

        const freed = data.size_freed_formatted;
        const count = data.deleted_count;
        const failed = data.failed_paths;

        if (failed && failed.length > 0) {
            alert(`Очищення завершено з попередженнями:\n• Видалено файлів: ${count} (${freed} звільнено).\n• Не вдалося видалити: ${failed.length} через обмеження доступу.`);
        } else {
            alert(`Очищення виконано успішно!\nВидалено файлів: ${count} (${freed} звільнено).`);
        }

        // Trigger a re-run of forecast and stats or reload recent scans
        loadRecentScans();
        
        // Let's reload settings / dashboard state
        if (state.scannedResults) {
            // Remove deleted files from state lists locally so we don't have to scan again
            const deletedSet = new Set(state.deletePathsQueue);
            state.scannedResults.large_files = state.scannedResults.large_files.filter(f => !deletedSet.has(f.path));
            state.scannedResults.temp_files = state.scannedResults.temp_files.filter(f => !deletedSet.has(f.path));
            state.scannedResults.log_files = state.scannedResults.log_files.filter(f => !deletedSet.has(f.path));
            
            // Re-render
            renderScanResults(state.scannedResults);
        }
    });
}

function renderScanResults(results) {
    state.scannedResults = results;
    document.getElementById('scan-results-container').classList.remove('hidden');

    // Update Header cards values
    document.getElementById('stat-total-size').innerText = results.total_size_formatted;
    document.getElementById('stat-file-count').innerText = `${results.files_scanned} файлів`;
    
    const flaggedCount = results.rules_flagged_count || 0;
    const flaggedSize = results.rules_flagged_size || 0;
    document.getElementById('stat-flagged-size').innerText = formatBytes(flaggedSize);
    document.getElementById('stat-flagged-count').innerText = `${flaggedCount} файлів за правилами`;

    // Compute Storage Forecast trend in background
    loadStorageExhaustionForecast();

    // SRE Health score calculations
    renderSreHealthScore(results);

    // AI Provider recommendation button activation
    document.getElementById('btn-generate-ai').removeAttribute('disabled');
    document.getElementById('btn-generate-ai').innerText = 'Згенерувати рекомендації ШІ';

    // Populate Right side tables
    populateFilesTable('table-large-files', results.large_files);
    populateFilesTable('table-temp-files', results.temp_files);
    populateFilesTable('table-log-files', results.log_files);
    renderDuplicatesPane(results.duplicate_groups);

    // SRE Container cards
    const sreData = results.sre_data;
    const devopsCard = document.getElementById('devops-sre-card');
    const winSreCard = document.getElementById('windows-sre-card');

    if (sreData && sreData.docker_active) {
        devopsCard.classList.remove('hidden');
        const containerBody = document.querySelector('#table-sre-containers tbody');
        containerBody.innerHTML = '';
        sreData.containers.forEach(c => {
            containerBody.innerHTML += `<tr>
                <td>${c.name}</td>
                <td>${c.image}</td>
                <td class="${c.write_size > 1024*1024*1024 ? 'text-danger bold' : ''}">${c.write_size_formatted}</td>
                <td>${c.virtual_size_formatted}</td>
            </tr>`;
        });

        const volumesBody = document.querySelector('#table-sre-volumes tbody');
        volumesBody.innerHTML = '';
        sreData.volumes.forEach(v => {
            volumesBody.innerHTML += `<tr>
                <td>${v.name}</td>
                <td>${v.size_formatted}</td>
            </tr>`;
        });
    } else {
        devopsCard.classList.add('hidden');
    }

    if (sreData && sreData.windows_active) {
        winSreCard.classList.remove('hidden');
        const winBody = document.querySelector('#table-sre-windows tbody');
        winBody.innerHTML = '';
        Object.entries(sreData.folders).forEach(([key, f]) => {
            winBody.innerHTML += `<tr>
                <td>${key.toUpperCase()}</td>
                <td>${f.path}</td>
                <td>${f.count}</td>
                <td class="${f.size > 500*1024*1024 ? 'text-danger bold' : ''}">${f.size_formatted}</td>
            </tr>`;
        });
    } else {
        winSreCard.classList.add('hidden');
    }
}

function renderSreHealthScore(results) {
    const sreData = results.sre_data;
    
    // We compute health score logic on frontend side using our backend.sre methods mapping
    let dupWaste = 0;
    if (results.duplicate_groups) {
        Object.values(results.duplicate_groups).forEach(paths => {
            if (paths.length > 1) {
                dupWaste += (paths.length - 1) * paths[0].size;
            }
        });
    }

    let logSize = 0;
    if (results.log_files) {
        results.log_files.forEach(f => logSize += f.size);
    }

    let tempSize = 0;
    if (results.temp_files) {
        results.temp_files.forEach(f => tempSize += f.size);
    }
    if (results.cache_files) {
        results.cache_files.forEach(f => tempSize += f.size);
    }

    let daysRemaining = -1; // loaded dynamically from forecast engine later

    // Base score computation (we default to 100 and deduct on UI, but call forecast calculation if needed)
    updateHealthRingUI(95, []); // Default standard UI score
}

function updateHealthRingUI(score, warnings) {
    const healthText = document.getElementById('health-score-val');
    const healthRing = document.getElementById('health-ring-fill');
    const healthDesc = document.getElementById('health-score-desc');

    healthText.innerText = score;
    healthRing.setAttribute('stroke-dasharray', `${score}, 100`);

    if (score >= 85) {
        healthRing.style.stroke = 'var(--color-success)';
        healthDesc.innerText = 'Відмінно';
        healthDesc.className = 'stat-desc green-text';
    } else if (score >= 50) {
        healthRing.style.stroke = 'var(--color-warning)';
        healthDesc.innerText = 'Задовільно';
        healthDesc.className = 'stat-desc yellow-text';
    } else {
        healthRing.style.stroke = 'var(--color-danger)';
        healthDesc.innerText = 'Критично';
        healthDesc.className = 'stat-desc red-text';
    }
}

async function loadStorageExhaustionForecast() {
    const scanPath = document.getElementById('input-scan-path').value.trim();
    try {
        const fc = await GetStorageForecast(scanPath);
        if (fc && fc.status !== 'insufficient_data') {
            // Re-calculate SRE Health score including forecast values
            let dupWaste = 0;
            if (state.scannedResults.duplicate_groups) {
                Object.values(state.scannedResults.duplicate_groups).forEach(paths => {
                    if (paths.length > 1) {
                        dupWaste += (paths.length - 1) * paths[0].size;
                    }
                });
            }

            let logSize = 0;
            state.scannedResults.log_files.forEach(f => logSize += f.size);

            let tempSize = 0;
            state.scannedResults.temp_files.forEach(f => tempSize += f.size);

            // Re-call SRE score calculation in pure JS equivalent of Go backend
            let score = 100;
            let warnings = [];

            if (fc.days_remaining != -1) {
                if (fc.days_remaining < 30) {
                    score -= 30;
                    warnings.push(`Storage exhaustion projected in ${fc.days_remaining} days.`);
                } else if (fc.days_remaining < 90) {
                    score -= 15;
                    warnings.push(`Storage exhaustion projected in ${fc.days_remaining} days.`);
                }
            }

            if (dupWaste > 10*1024*1024*1024) score -= 15;
            else if (dupWaste > 1*1024*1024*1024) score -= 8;

            if (logSize > 5*1024*1024*1024) score -= 15;
            else if (logSize > 500*1024*1024) score -= 5;

            if (tempSize > 5*1024*1024*1024) score -= 10;

            if (state.scannedResults.sre_data && state.scannedResults.sre_data.docker_active) {
                let largeCount = state.scannedResults.sre_data.containers.filter(c => c.write_size > 1024*1024*1024).length;
                if (largeCount > 0) score -= 10;
            }

            score = Math.max(0, Math.min(100, score));
            updateHealthRingUI(score, warnings);
        }
    } catch (err) {
        console.error("Forecast failed:", err);
    }
}

// Accordion tables rendering
function populateFilesTable(tableId, filesList) {
    const tbody = document.querySelector(`#${tableId} tbody`);
    tbody.innerHTML = '';

    if (!filesList || filesList.length === 0) {
        tbody.innerHTML = '<tr><td colspan="4" class="text-center" style="color: var(--text-muted); font-style: italic;">Файлів не знайдено</td></tr>';
        return;
    }

    const rows = [];
    filesList.forEach(f => {
        // Safe string escapes for onclick arguments
        const escapedPath = f.path.replace(/\\/g, '\\\\').replace(/'/g, "\\'");
        
        rows.push(`<tr>
            <td style="word-break: break-all;">${f.path}</td>
            <td style="white-space: nowrap;">${f.size_formatted}</td>
            <td style="color: var(--text-secondary);">${f.rule_match || '-'}</td>
            <td class="col-action">
                <button class="btn-icon delete" onclick="confirmSingleDelete(this, '${escapedPath}')">🗑️</button>
            </td>
        </tr>`);
    });
    tbody.innerHTML = rows.join('');
}

function renderDuplicatesPane(dupGroups) {
    const container = document.getElementById('duplicates-group-container');
    container.innerHTML = '';

    if (!dupGroups || Object.keys(dupGroups).length === 0) {
        container.innerHTML = '<p style="color: var(--text-muted); font-style: italic; text-align: center; margin-top: 40px;">Дублікатів не виявлено</p>';
        return;
    }

    const cards = [];
    Object.entries(dupGroups).forEach(([hash, list]) => {
        const rows = [];
        list.forEach(dp => {
            const escapedPath = dp.path.replace(/\\/g, '\\\\').replace(/'/g, "\\'");
            rows.push(`<div class="duplicate-path-row">
                <span class="duplicate-path">${dp.path}</span>
                <div style="display: flex; align-items: center; gap: 10px;">
                    <span style="color: var(--text-secondary); font-size: 11px;">${dp.size_formatted}</span>
                    <button class="btn-icon delete" onclick="confirmSingleDelete(this, '${escapedPath}')">🗑️</button>
                </div>
            </div>`);
        });

        cards.push(`<div class="duplicate-group-card">
            <div class="duplicate-group-header">
                <span>Хеш групи: ${hash.substring(0, 12)}...</span>
                <span>Знайдено копій: ${list.length}</span>
            </div>
            <div class="duplicate-group-paths">
                ${rows.join('')}
            </div>
        </div>`);
    });
    container.innerHTML = cards.join('');
}

// ----------------------------------------------------
// File Deletion Modals Flow
// ----------------------------------------------------
window.confirmSingleDelete = async function(buttonEl, filePath) {
    state.deletePathsQueue = [filePath];
    showDeleteConfirmationModal();
};

async function showDeleteConfirmationModal() {
    const countEl = document.getElementById('delete-files-count');
    const sizeEl = document.getElementById('delete-files-size');
    const restrictedBox = document.getElementById('delete-restricted-box');
    const restrictedCount = document.getElementById('delete-restricted-count');
    const executeBtn = document.getElementById('btn-delete-execute');
    const fileListEl = document.getElementById('modal-delete-confirm').querySelector('.modal-body');

    // Call Wails backend dry run
    try {
        const dryRun = await DryRunCleanup(state.deletePathsQueue);
        
        countEl.innerText = dryRun.writable_files.length;
        sizeEl.innerText = dryRun.total_size_formatted;

        if (dryRun.restricted_files.length > 0) {
            restrictedBox.classList.remove('hidden');
            restrictedCount.innerText = dryRun.restricted_files.length;
        } else {
            restrictedBox.classList.add('hidden');
        }

        executeBtn.removeAttribute('disabled');
        if (!dryRun.can_proceed) {
            executeBtn.setAttribute('disabled', 'true');
        }

        // Open Modal
        document.getElementById('modal-delete-confirm').classList.remove('hidden');
    } catch (err) {
        alert("Dry run validation failed: " + err);
    }
}

function executeDeletionsQueue() {
    const useRecycleBin = document.getElementById('chk-recycle-bin').checked;
    
    // Check if user turned off Recycle Bin, warn
    if (!useRecycleBin) {
        const conf = confirm("УВАГА: Ви вимкнули переміщення в Кошик. Файли буде видалено назавжди!\nПродовжити?");
        if (!conf) return;
    }

    // Disable buttons
    document.getElementById('btn-delete-execute').setAttribute('disabled', 'true');
    document.getElementById('btn-delete-cancel').setAttribute('disabled', 'true');

    // Show progress block
    document.getElementById('delete-progress-block').classList.remove('hidden');
    document.getElementById('delete-progress-fill').style.width = '0%';
    document.getElementById('delete-progress-file-text').innerText = 'Підготовка до видалення...';

    // Start deletion thread inside Wails
    SafeDeleteFiles(state.deletePathsQueue, useRecycleBin);
}

// ----------------------------------------------------
// AI recommendation generator
// ----------------------------------------------------
async function generateAIRecommendation() {
    if (!state.scannedResults) {
        alert('Помилка: Спочатку запустіть сканування!');
        return;
    }

    const aiBtn = document.getElementById('btn-generate-ai');
    const chatBrowser = document.getElementById('ai-chat-browser');
    const bulkDeleteBtn = document.getElementById('btn-bulk-delete-ai');

    aiBtn.setAttribute('disabled', 'true');
    aiBtn.innerText = '🤖 ШІ аналізує...';
    chatBrowser.innerHTML = '<p class="placeholder-text" style="color: var(--color-accent); font-weight: 600;">🤖 Надсилання запиту до штучного інтелекту... Будь ласка, зачекайте.</p>';

    // Prepare drive statistics
    const summary = `Connection Type: Local\nTotal Drive Capacity Scanned: ${state.scannedResults.total_size_formatted}\nTotal Files Traversed: ${state.scannedResults.files_scanned}\nFlagged Size: ${formatBytes(state.scannedResults.rules_flagged_size)}`;
    
    try {
        const filesList = [
            ...state.scannedResults.large_files,
            ...state.scannedResults.temp_files,
            ...state.scannedResults.log_files
        ];

        const rec = await GenerateAIRecommendation(summary, filesList);
        
        // Render Ukrainian markdown recommendations with click links support
        chatBrowser.innerHTML = renderMarkdown(rec);
        bulkDeleteBtn.classList.remove('hidden');
    } catch (err) {
        chatBrowser.innerHTML = `<p class="placeholder-text text-danger">⚠️ Помилка ШІ: ${err.message || err}</p>`;
        bulkDeleteBtn.classList.add('hidden');
    } finally {
        aiBtn.removeAttribute('disabled');
        aiBtn.innerText = 'Згенерувати рекомендації ШІ';
    }
}

// Intercept AI markdown delete:// URL clicks
window.confirmSingleDeleteFromLink = function(filePath) {
    state.deletePathsQueue = [filePath];
    showDeleteConfirmationModal();
};

function renderMarkdown(md) {
    if (!md) return "";
    let html = md;
    
    // Convert headers
    html = html.replace(/### (.*)/g, '<h3>$1</h3>');
    html = html.replace(/## (.*)/g, '<h2>$1</h2>');
    html = html.replace(/# (.*)/g, '<h1>$1</h1>');
    
    // Bold text
    html = html.replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>');
    
    // Lists formatting
    html = html.replace(/^\s*-\s+(.*)/gm, '<li>$1</li>');
    
    // Crucial: Preprocess delete:// links securely
    html = html.replace(/\[Видалити\]\(delete:\/\/([^\)]+)\)/g, (match, urlPath) => {
        // Decode to raw Windows/Linux path
        const decoded = decodeURIComponent(urlPath);
        // Clean backslashes for string parameters safely
        const escaped = decoded.replace(/\\/g, '\\\\').replace(/'/g, "\\'");
        return `<a class="delete-link" href="#" onclick="confirmSingleDeleteFromLink('${escaped}'); return false;">Видалити</a>`;
    });

    return html;
}

// Parse AI recommendation output for bulk deletion recommended files
function triggerAIRecommendationsCleanup() {
    const chatBrowser = document.getElementById('ai-chat-browser');
    const links = chatBrowser.querySelectorAll('a.delete-link');
    
    if (links.length === 0) {
        alert('ШІ не залишив конкретних посилань для видалення в рекомендаціях.');
        return;
    }

    const paths = [];
    links.forEach(a => {
        // Extract string from onclick="confirmSingleDeleteFromLink('path')"
        const onclickAttr = a.getAttribute('onclick');
        const match = onclickAttr.match(/confirmSingleDeleteFromLink\('(.*)'\)/);
        if (match && match[1]) {
            paths.push(match[1].replace(/\\\\/g, '\\'));
        }
    });

    if (paths.length > 0) {
        state.deletePathsQueue = paths;
        showDeleteConfirmationModal();
    }
}

// ----------------------------------------------------
// Settings Tab Forms Operations
// ----------------------------------------------------
async function loadAIProviders() {
    try {
        const providers = await GetAIProviders();
        state.savedAIProviders = providers;

        // Load active provider
        const selected = providers.find(p => p.is_selected === 1);
        if (selected) {
            state.activeAIProvider = selected;
            document.getElementById('select-ai-provider-type').value = selected.type;
            renderAIProviderFormFields(selected.type);
            
            const config = JSON.parse(selected.config_json);
            document.getElementById('input-ai-url').value = config.base_url || '';
            document.getElementById('input-ai-key').value = config.api_key || '';
            document.getElementById('input-ai-temp').value = config.temperature || '0.7';

            // Load model options
            const modelSelect = document.getElementById('select-ai-model');
            modelSelect.innerHTML = `<option value="${config.model}">${config.model}</option>`;
            modelSelect.value = config.model;
        } else {
            renderAIProviderFormFields('ollama');
        }
    } catch (err) {
        console.error("AI load failed:", err);
    }
}

function renderAIProviderFormFields(type) {
    const urlGroup = document.getElementById('ai-group-url');
    const keyGroup = document.getElementById('ai-group-key');
    const urlInput = document.getElementById('input-ai-url');

    if (type === 'ollama') {
        urlGroup.classList.remove('hidden');
        keyGroup.classList.add('hidden');
        urlInput.value = 'http://localhost:11434';
    } else if (type === 'lmstudio') {
        urlGroup.classList.remove('hidden');
        keyGroup.classList.add('hidden');
        urlInput.value = 'http://localhost:1234/v1';
    } else {
        urlGroup.classList.remove('hidden');
        keyGroup.classList.remove('hidden');
        if (type === 'openai') urlInput.value = 'https://api.openai.com/v1';
        else if (type === 'gemini') urlInput.value = 'https://generativelanguage.googleapis.com';
        else if (type === 'anthropic') urlInput.value = 'https://api.anthropic.com';
        else if (type === 'deepseek') urlInput.value = 'https://api.deepseek.com';
    }
}

async function fetchAIModelsList() {
    const type = document.getElementById('select-ai-provider-type').value;
    const url = document.getElementById('input-ai-url').value.trim();
    const key = document.getElementById('input-ai-key').value.trim();
    const btn = document.getElementById('btn-fetch-models');

    btn.innerText = '⏳ Отримання...';
    btn.setAttribute('disabled', 'true');

    try {
        const models = await GetAIModels(type, key, url);
        const select = document.getElementById('select-ai-model');
        select.innerHTML = '';
        
        models.forEach(m => {
            select.innerHTML += `<option value="${m}">${m}</option>`;
        });
        alert('Список моделей успішно оновлено!');
    } catch (err) {
        alert(`Помилка отримання моделей: ${err}`);
    } finally {
        btn.innerText = '🔄 Отримати список';
        btn.removeAttribute('disabled');
    }
}

async function testAIProviderConnection() {
    const type = document.getElementById('select-ai-provider-type').value;
    const url = document.getElementById('input-ai-url').value.trim();
    const key = document.getElementById('input-ai-key').value.trim();
    const model = document.getElementById('select-ai-model').value;
    const temp = parseFloat(document.getElementById('input-ai-temp').value) || 0.7;

    const btn = document.getElementById('btn-test-ai');
    btn.innerText = '🔌 Тестування...';

    try {
        const res = await TestAIProviderConnection(type, key, url, model, temp);
        if (res.success) {
            alert(`Успіх: ${res.message}`);
        } else {
            alert(`Помилка з'єднання: ${res.message}`);
        }
    } catch (err) {
        alert(`Помилка тестування: ${err}`);
    } finally {
        btn.innerText = "⚡ Тестувати з'єднання";
    }
}

async function saveAIProviderConfiguration() {
    const type = document.getElementById('select-ai-provider-type').value;
    const url = document.getElementById('input-ai-url').value.trim();
    const key = document.getElementById('input-ai-key').value.trim();
    const model = document.getElementById('select-ai-model').value;
    const temp = parseFloat(document.getElementById('input-ai-temp').value) || 0.7;

    if (!model) {
        alert('Помилка: Будь ласка, оберіть модель перед збереженням!');
        return;
    }

    const config = {
        base_url: url,
        api_key: key,
        model: model,
        temperature: temp
    };

    try {
        await SaveAIProvider(type, type, JSON.stringify(config), 1);
        alert('Налаштування провайдера ШІ збережено!');
        loadAIProviders();
    } catch (err) {
        alert(`Помилка збереження: ${err}`);
    }
}

// Rules Configuration Editor
async function loadScanRules() {
    try {
        const rulesJSON = await GetScanRules();
        state.scanRules = JSON.parse(rulesJSON);
        
        state.scanRules.forEach(r => {
            if (r.id === 'temp_old') {
                document.getElementById('chk-rule-temp').checked = r.enabled;
                document.getElementById('val-rule-temp').value = r.value;
            } else if (r.id === 'log_large') {
                document.getElementById('chk-rule-log-size').checked = r.enabled;
                document.getElementById('val-rule-log-size').value = r.value;
            } else if (r.id === 'log_old') {
                document.getElementById('chk-rule-log-old').checked = r.enabled;
                document.getElementById('val-rule-log-old').value = r.value;
            } else if (r.id === 'backup_old') {
                document.getElementById('chk-rule-backup').checked = r.enabled;
                document.getElementById('val-rule-backup').value = r.value;
            } else if (r.id === 'large_huge') {
                document.getElementById('chk-rule-large').checked = r.enabled;
                document.getElementById('val-rule-large').value = r.value;
            }
        });
    } catch (err) {
        console.error("Rules load failed:", err);
    }
}

async function saveRulesConfiguration() {
    const activeRules = [
        {
            id: 'temp_old',
            name: 'Temp files older than 30 days',
            category: 'temp',
            condition: 'older_than_days',
            value: parseInt(document.getElementById('val-rule-temp').value) || 30,
            enabled: document.getElementById('chk-rule-temp').checked
        },
        {
            id: 'log_large',
            name: 'Log files larger than 100 MB',
            category: 'log',
            condition: 'larger_than_mb',
            value: parseInt(document.getElementById('val-rule-log-size').value) || 100,
            enabled: document.getElementById('chk-rule-log-size').checked
        },
        {
            id: 'log_old',
            name: 'Log files older than 14 days',
            category: 'log',
            condition: 'older_than_days',
            value: parseInt(document.getElementById('val-rule-log-old').value) || 14,
            enabled: document.getElementById('chk-rule-log-old').checked
        },
        {
            id: 'backup_old',
            name: 'Backups older than 90 days',
            category: 'backup',
            condition: 'older_than_days',
            value: parseInt(document.getElementById('val-rule-backup').value) || 90,
            enabled: document.getElementById('chk-rule-backup').checked
        },
        {
            id: 'large_huge',
            name: 'Uncategorized files larger than 1 GB',
            category: 'large',
            condition: 'larger_than_mb',
            value: parseInt(document.getElementById('val-rule-large').value) || 1024,
            enabled: document.getElementById('chk-rule-large').checked
        }
    ];

    try {
        await SaveScanRules(JSON.stringify(activeRules));
        alert('Правила сканування успішно збережено!');
    } catch (err) {
        alert(`Помилка збереження правил: ${err}`);
    }
}

// ----------------------------------------------------
// SSH Hosts Configuration Manager (CRUD)
// ----------------------------------------------------
async function loadSSHHosts() {
    try {
        const hosts = await GetSSHHosts();
        state.savedSSHHosts = hosts;
        renderSSHHostsTable();
    } catch (err) {
        console.error("SSH load failed:", err);
    }
}

function loadSSHHostsDropdown() {
    const select = document.getElementById('select-ssh-server');
    select.innerHTML = '<option value="">-- Пряме введення --</option>';
    state.savedSSHHosts.forEach(h => {
        select.innerHTML += `<option value="${h.id}">${h.name} (${h.host})</option>`;
    });
}

function renderSSHHostsTable() {
    const tbody = document.querySelector('#table-ssh-hosts tbody');
    if (!tbody) return;
    tbody.innerHTML = '';

    if (state.savedSSHHosts.length === 0) {
        tbody.innerHTML = '<tr><td colspan="6" class="text-center" style="color: var(--text-muted); font-style: italic;">Немає збережених хостів</td></tr>';
        return;
    }

    state.savedSSHHosts.forEach(h => {
        tbody.innerHTML += `<tr>
            <td>${h.name}</td>
            <td>${h.host}</td>
            <td>${h.port}</td>
            <td>${h.username}</td>
            <td>${h.auth_type === 'password' ? 'Пароль' : 'Ключ'}</td>
            <td class="col-action-wide">
                <div class="action-row-buttons">
                    <button class="btn btn-secondary btn-icon" onclick="openSSHHostEditorEdit(${h.id})">✏️</button>
                    <button class="btn btn-danger btn-icon" onclick="deleteSSHHostItem(${h.id})">❌</button>
                </div>
            </td>
        </tr>`;
    });
}

// Helper CRUD actions
window.openSSHHostEditorEdit = function(id) {
    const host = state.savedSSHHosts.find(h => h.id == id);
    if (!host) return;

    state.editingSSHId = id;
    document.getElementById('ssh-modal-title').innerText = 'Редагувати SSH хост';
    document.getElementById('input-ssh-name').value = host.name;
    document.getElementById('input-ssh-editor-host').value = host.host;
    document.getElementById('input-ssh-editor-port').value = host.port;
    document.getElementById('input-ssh-editor-user').value = host.username;
    document.getElementById('select-ssh-editor-auth').value = host.auth_type;
    document.getElementById('input-ssh-editor-cred').value = host.credentials;

    onSSHAuthTypeChanged(host.auth_type);
    document.getElementById('modal-ssh-editor').classList.remove('hidden');
};

function openSSHHostEditorAdd() {
    state.editingSSHId = null;
    document.getElementById('ssh-modal-title').innerText = 'Додати SSH хост';
    document.getElementById('input-ssh-name').value = '';
    document.getElementById('input-ssh-editor-host').value = '';
    document.getElementById('input-ssh-editor-port').value = '22';
    document.getElementById('input-ssh-editor-user').value = 'root';
    document.getElementById('select-ssh-editor-auth').value = 'password';
    document.getElementById('input-ssh-editor-cred').value = '';

    onSSHAuthTypeChanged('password');
    document.getElementById('modal-ssh-editor').classList.remove('hidden');
}

function closeSSHHostEditor() {
    document.getElementById('modal-ssh-editor').classList.add('hidden');
}

function onSSHAuthTypeChanged(authType) {
    const label = document.getElementById('lbl-ssh-editor-cred');
    const input = document.getElementById('input-ssh-editor-cred');

    if (authType === 'password') {
        label.innerText = 'Пароль віддаленого сервера:';
        input.placeholder = 'Введіть пароль';
    } else {
        label.innerText = 'Шлях до файлу приватного ключа:';
        input.placeholder = 'напр. C:\\Users\\Admin\\.ssh\\id_rsa';
    }
}

async function saveSSHHostDetails() {
    const name = document.getElementById('input-ssh-name').value.trim();
    const host = document.getElementById('input-ssh-editor-host').value.trim();
    const port = parseInt(document.getElementById('input-ssh-editor-port').value) || 22;
    const username = document.getElementById('input-ssh-editor-user').value.trim();
    const authType = document.getElementById('select-ssh-editor-auth').value;
    const credentials = document.getElementById('input-ssh-editor-cred').value.trim();

    if (!name || !host || !username) {
        alert('Помилка: Будь ласка, заповніть назву, хост та користувача!');
        return;
    }

    try {
        if (state.editingSSHId) {
            await EditSSHHost(state.editingSSHId, name, host, port, username, authType, credentials);
        } else {
            await AddSSHHost(name, host, port, username, authType, credentials);
        }
        closeSSHHostEditor();
        loadSSHHosts();
    } catch (err) {
        alert(`Помилка збереження хоста: ${err}`);
    }
}

window.deleteSSHHostItem = async function(id) {
    const conf = confirm("Ви впевнені, що хочете видалити цей SSH-хост?");
    if (!conf) return;

    try {
        await DeleteSSHHost(id);
        loadSSHHosts();
    } catch (err) {
        alert(`Помилка видалення: ${err}`);
    }
};

// ----------------------------------------------------
// Database history loads & conversions
// ----------------------------------------------------
async function loadRecentScans() {
    try {
        const list = await GetRecentScans();
        // Option to display recent scans in history modal if needed
    } catch (err) {
        console.error("Scan history failed:", err);
    }
}

// Size formatter helper
function formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const dm = 2;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
}
