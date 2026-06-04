import { state } from '../state.js';
import * as api from '../api.js';
import { formatBytes } from '../utils.js';
import * as ssh from './ssh.js';
import { EventsOn } from '../../wailsjs/runtime/runtime.js';

export function onConnectionTypeChanged(type) {
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
        ssh.loadSSHHostsDropdown();
    } else if (type === 'Network Share (UNC)') {
        browseBtn.classList.remove('hidden');
        sshPicker.classList.add('hidden');
        sshCreds.classList.add('hidden');
        scanPathInput.value = '\\\\server\\share';
        lblScanPath.innerText = 'Шлях мережевої папки:';
    }
}

export function onSSHActiveDropdownChanged() {
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

export let isScanning = false;

export function toggleScan() {
    const scanBtn = document.getElementById('btn-toggle-scan');
    const progressCard = document.getElementById('scan-progress-card');

    if (isScanning) {
        api.CancelScan();
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
    api.StartScan(connType, hostID, scanPath);
}

export function setupWailsEvents(loadRecentScansCallback) {
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

        if (typeof loadRecentScansCallback === 'function') {
            loadRecentScansCallback();
        }
        
        if (state.scannedResults) {
            const deletedSet = new Set(state.deletePathsQueue);
            state.scannedResults.large_files = state.scannedResults.large_files.filter(f => !deletedSet.has(f.path));
            state.scannedResults.temp_files = state.scannedResults.temp_files.filter(f => !deletedSet.has(f.path));
            state.scannedResults.log_files = state.scannedResults.log_files.filter(f => !deletedSet.has(f.path));
            
            renderScanResults(state.scannedResults);
        }
    });
}

export function renderScanResults(results) {
    state.scannedResults = results;
    document.getElementById('scan-results-container').classList.remove('hidden');

    document.getElementById('stat-total-size').innerText = results.total_size_formatted;
    document.getElementById('stat-file-count').innerText = `${results.files_scanned} файлів`;
    
    const flaggedCount = results.rules_flagged_count || 0;
    const flaggedSize = results.rules_flagged_size || 0;
    document.getElementById('stat-flagged-size').innerText = formatBytes(flaggedSize);
    document.getElementById('stat-flagged-count').innerText = `${flaggedCount} файлів за правилами`;

    loadStorageExhaustionForecast();

    renderSreHealthScore(results);

    document.getElementById('btn-generate-ai').removeAttribute('disabled');
    document.getElementById('btn-generate-ai').innerText = 'Згенерувати рекомендації ШІ';

    populateFilesTable('table-large-files', results.large_files);
    populateFilesTable('table-temp-files', results.temp_files);
    populateFilesTable('table-log-files', results.log_files);
    renderDuplicatesPane(results.duplicate_groups);

    const sreData = results.sre_data;
    const devopsCard = document.getElementById('devops-sre-card');
    const winSreCard = document.getElementById('windows-sre-card');

    if (sreData && sreData.docker_active) {
        devopsCard.classList.remove('hidden');
        const containerBody = document.querySelector('#table-sre-containers tbody');
        containerBody.innerHTML = '';
        
        const connType = document.getElementById('select-conn-type').value;
        const hostDropdown = document.getElementById('select-ssh-server');
        const hostID = parseInt(hostDropdown.value) || 0;

        sreData.containers.forEach(c => {
            containerBody.innerHTML += `<tr>
                <td>${c.name}</td>
                <td>${c.image}</td>
                <td class="${c.write_size > 1024*1024*1024 ? 'text-danger bold' : ''}">${c.write_size_formatted}</td>
                <td>${c.virtual_size_formatted}</td>
                <td class="col-action">
                    <button class="btn btn-secondary btn-icon" onclick="clearContainerLogs('${connType}', ${hostID}, '${c.id}', event)" title="Очистити логи контейнера">🧹</button>
                </td>
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

    const packageCacheCard = document.getElementById('package-cache-sre-card');
    if (sreData && sreData.package_caches && sreData.package_caches.length > 0) {
        packageCacheCard.classList.remove('hidden');
        const cacheBody = document.querySelector('#table-sre-package-caches tbody');
        cacheBody.innerHTML = '';
        
        const connType = document.getElementById('select-conn-type').value;
        const hostDropdown = document.getElementById('select-ssh-server');
        const hostID = parseInt(hostDropdown.value) || 0;

        sreData.package_caches.forEach(pc => {
            const escapedCleanCmd = pc.clean_cmd.replace(/\\/g, '\\\\').replace(/'/g, "\\'");
            const escapedPath = pc.path.replace(/\\/g, '\\\\').replace(/'/g, "\\'");
            cacheBody.innerHTML += `<tr>
                <td>${pc.name}</td>
                <td style="word-break: break-all;">${pc.path}</td>
                <td class="${pc.size > 1024*1024*1024 ? 'text-danger bold' : ''}">${pc.size_formatted}</td>
                <td class="col-action-wide">
                    <button class="btn btn-secondary btn-sm" onclick="clearPackageCache('${connType}', ${hostID}, '${escapedCleanCmd}', '${escapedPath}', event)">🧹 Очистити кеш</button>
                </td>
            </tr>`;
        });
    } else {
        packageCacheCard.classList.add('hidden');
    }
}

export function renderSreHealthScore(results) {
    const sreData = results.sre_data;
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

    updateHealthRingUI(95, []); 
}

export function updateHealthRingUI(score, warnings) {
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

export async function loadStorageExhaustionForecast() {
    const scanPath = document.getElementById('input-scan-path').value.trim();
    try {
        const fc = await api.GetStorageForecast(scanPath);
        if (fc && fc.status !== 'insufficient_data') {
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

export function populateFilesTable(tableId, filesList) {
    const tbody = document.querySelector(`#${tableId} tbody`);
    tbody.innerHTML = '';

    if (!filesList || filesList.length === 0) {
        tbody.innerHTML = '<tr><td colspan="4" class="text-center" style="color: var(--text-muted); font-style: italic;">Файлів не знайдено</td></tr>';
        return;
    }

    const rows = [];
    filesList.forEach(f => {
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

export function renderDuplicatesPane(dupGroups) {
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

window.confirmSingleDelete = async function(buttonEl, filePath) {
    state.deletePathsQueue = [filePath];
    showDeleteConfirmationModal();
};

export async function showDeleteConfirmationModal() {
    const countEl = document.getElementById('delete-files-count');
    const sizeEl = document.getElementById('delete-files-size');
    const restrictedBox = document.getElementById('delete-restricted-box');
    const restrictedCount = document.getElementById('delete-restricted-count');
    const executeBtn = document.getElementById('btn-delete-execute');

    try {
        const connType = document.getElementById('select-conn-type').value;
        const hostDropdown = document.getElementById('select-ssh-server');
        const hostID = parseInt(hostDropdown.value) || 0;
        
        const dryRun = await api.DryRunCleanup(connType, hostID, state.deletePathsQueue);
        
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

        document.getElementById('modal-delete-confirm').classList.remove('hidden');
    } catch (err) {
        alert("Dry run validation failed: " + err);
    }
}

export function executeDeletionsQueue() {
    const useRecycleBin = document.getElementById('chk-recycle-bin').checked;
    
    if (!useRecycleBin) {
        const conf = confirm("УВАГА: Ви вимкнули переміщення в Кошик. Файли буде видалено назавжди!\nПродовжити?");
        if (!conf) return;
    }

    document.getElementById('btn-delete-execute').setAttribute('disabled', 'true');
    document.getElementById('btn-delete-cancel').setAttribute('disabled', 'true');

    document.getElementById('delete-progress-block').classList.remove('hidden');
    document.getElementById('delete-progress-fill').style.width = '0%';
    document.getElementById('delete-progress-file-text').innerText = 'Підготовка до видалення...';

    const connType = document.getElementById('select-conn-type').value;
    const hostDropdown = document.getElementById('select-ssh-server');
    const hostID = parseInt(hostDropdown.value) || 0;

    api.SafeDeleteFiles(connType, hostID, state.deletePathsQueue, useRecycleBin);
}

export async function generateAIRecommendation() {
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

    const connType = document.getElementById('select-conn-type').value;
    const sreData = state.scannedResults.sre_data;
    let targetOS = 'Unknown';
    if (connType === 'SSH Remote Linux') {
        targetOS = 'Linux';
    } else if (sreData && sreData.windows_active) {
        targetOS = 'Windows';
    } else {
        targetOS = 'Local OS';
    }

    let summary = `Connection Type: ${connType}\n`;
    summary += `Target Operating System: ${targetOS}\n`;
    summary += `Total Drive Capacity Scanned: ${state.scannedResults.total_size_formatted}\n`;
    summary += `Total Files Traversed: ${state.scannedResults.files_scanned}\n`;
    summary += `Flagged Size: ${formatBytes(state.scannedResults.rules_flagged_size || 0)}\n`;

    if (sreData && sreData.docker_active) {
        summary += `\nDocker SRE Metrics:\n`;
        if (sreData.containers && sreData.containers.length > 0) {
            summary += `- Active Containers Write Layers:\n`;
            sreData.containers.forEach(c => {
                summary += `  * Container: ${c.name} (Image: ${c.image}) - Write size: ${c.write_size_formatted}, Virtual size: ${c.virtual_size_formatted}\n`;
            });
        }
        if (sreData.volumes && sreData.volumes.length > 0) {
            summary += `- Docker Volumes:\n`;
            sreData.volumes.forEach(v => {
                summary += `  * Volume: ${v.name} - Size: ${v.size_formatted}\n`;
            });
        }
    }

    if (sreData && sreData.windows_active && sreData.folders) {
        summary += `\nWindows System Folders Metrics:\n`;
        Object.entries(sreData.folders).forEach(([key, f]) => {
            summary += `- ${key.toUpperCase()}: Path "${f.path}" contains ${f.count} files, Size: ${f.size_formatted}\n`;
        });
    }

    let dupWaste = 0;
    if (state.scannedResults.duplicate_groups) {
        Object.values(state.scannedResults.duplicate_groups).forEach(paths => {
            if (paths.length > 1) {
                dupWaste += (paths.length - 1) * paths[0].size;
            }
        });
    }
    if (dupWaste > 0) {
        summary += `\nDuplicate Files Space Waste: ${formatBytes(dupWaste)}\n`;
    }

    if (sreData && sreData.package_caches && sreData.package_caches.length > 0) {
        summary += `\nPackage Manager Caches:\n`;
        sreData.package_caches.forEach(pc => {
            summary += `- ${pc.name}: Path "${pc.path}", Size: ${pc.size_formatted} (Clean command: "${pc.clean_cmd}")\n`;
        });
    }

    try {
        const filesList = [
            ...state.scannedResults.large_files,
            ...state.scannedResults.temp_files,
            ...state.scannedResults.log_files
        ];

        const rec = await api.GenerateAIRecommendation(summary, filesList);
        
        state.chatHistory = [
            { role: 'assistant', content: rec }
        ];

        chatBrowser.innerHTML = `<div class="chat-bubble assistant">${renderMarkdown(rec)}</div>`;
        document.getElementById('ai-chat-input-bar').classList.remove('hidden');
        
        updateBulkDeleteButtonVisibility();

        chatBrowser.scrollTop = chatBrowser.scrollHeight;
    } catch (err) {
        chatBrowser.innerHTML = `<p class="placeholder-text text-danger">⚠️ Помилка ШІ: ${err.message || err}</p>`;
        bulkDeleteBtn.classList.add('hidden');
        document.getElementById('ai-chat-input-bar').classList.add('hidden');
    } finally {
        aiBtn.removeAttribute('disabled');
        aiBtn.innerText = 'Згенерувати рекомендації ШІ';
    }
}

window.clearContainerLogs = async function(connType, hostID, containerID, e) {
    const btn = e ? e.currentTarget : null;
    const oldText = btn ? btn.innerText : '';
    if (btn) {
        btn.setAttribute('disabled', 'true');
        btn.innerText = '⏳';
    }
    try {
        await api.ClearContainerLogs(connType, hostID, containerID);
        alert('Логи контейнера успішно очищено!');
        if (btn) {
            const sizeCell = btn.parentElement.previousElementSibling.previousElementSibling;
            if (sizeCell) {
                sizeCell.innerText = '0.00 B';
                sizeCell.classList.remove('text-danger', 'bold');
            }
        }
    } catch (err) {
        alert('Помилка очищення логів: ' + err);
    } finally {
        if (btn) {
            btn.removeAttribute('disabled');
            btn.innerText = oldText;
        }
    }
};

window.clearPackageCache = async function(connType, hostID, cleanCmd, cachePath, e) {
    const btn = e ? e.currentTarget : null;
    const oldText = btn ? btn.innerText : '';
    if (btn) {
        btn.setAttribute('disabled', 'true');
        btn.innerText = '⏳';
    }
    try {
        await api.ClearPackageCache(connType, hostID, cleanCmd, cachePath);
        alert('Кеш успішно очищено!');
        if (btn) {
            const sizeCell = btn.parentElement.previousElementSibling;
            if (sizeCell) {
                sizeCell.innerText = '0.00 B';
                sizeCell.classList.remove('text-danger', 'bold');
            }
        }
    } catch (err) {
        alert('Помилка очищення кешу: ' + err);
    } finally {
        if (btn) {
            btn.removeAttribute('disabled');
            btn.innerText = oldText;
        }
    }
};

window.pruneDockerSystem = async function(e) {
    const btn = e ? e.currentTarget : null;
    const oldText = btn ? btn.innerText : '';
    if (btn) {
        btn.setAttribute('disabled', 'true');
        btn.innerText = '⏳';
    }
    try {
        const connType = document.getElementById('select-conn-type').value;
        const hostDropdown = document.getElementById('select-ssh-server');
        const hostID = parseInt(hostDropdown.value) || 0;
        
        await api.PruneDockerSystem(connType, hostID);
        alert('Docker System Prune виконано успішно!');
    } catch (err) {
        alert('Помилка виконання Prune Docker: ' + err);
    } finally {
        if (btn) {
            btn.removeAttribute('disabled');
            btn.innerText = oldText;
        }
    }
};

window.vacuumJournaldLogs = async function(e) {
    const btn = e ? e.currentTarget : null;
    const oldText = btn ? btn.innerText : '';
    if (btn) {
        btn.setAttribute('disabled', 'true');
        btn.innerText = '⏳';
    }
    try {
        const connType = document.getElementById('select-conn-type').value;
        const hostDropdown = document.getElementById('select-ssh-server');
        const hostID = parseInt(hostDropdown.value) || 0;
        
        await api.VacuumJournaldLogs(connType, hostID);
        alert('Journald Logs успішно очищено (залишено останні 3 дні)!');
    } catch (err) {
        alert('Помилка очищення Journald Logs: ' + err);
    } finally {
        if (btn) {
            btn.removeAttribute('disabled');
            btn.innerText = oldText;
        }
    }
};

window.clearWindowsEventLogs = async function(e) {
    const btn = e ? e.currentTarget : null;
    const oldText = btn ? btn.innerText : '';
    if (btn) {
        btn.setAttribute('disabled', 'true');
        btn.innerText = '⏳';
    }
    try {
        const connType = document.getElementById('select-conn-type').value;
        const hostDropdown = document.getElementById('select-ssh-server');
        const hostID = parseInt(hostDropdown.value) || 0;
        
        await api.ClearWindowsEventLogs(connType, hostID);
        alert('Windows Event Logs успішно очищено!');
    } catch (err) {
        alert('Помилка очищення Windows Event Logs: ' + err);
    } finally {
        if (btn) {
            btn.removeAttribute('disabled');
            btn.innerText = oldText;
        }
    }
};

export async function sendAIChatMessage() {
    const input = document.getElementById('input-ai-chat');
    const message = input.value.trim();
    if (!message) return;

    input.value = '';
    
    const chatBrowser = document.getElementById('ai-chat-browser');
    const sendBtn = document.getElementById('btn-send-ai-chat');

    state.chatHistory.push({ role: 'user', content: message });
    chatBrowser.innerHTML += `<div class="chat-bubble user">${escapeHtml(message)}</div>`;
    chatBrowser.scrollTop = chatBrowser.scrollHeight;

    sendBtn.setAttribute('disabled', 'true');
    sendBtn.innerText = '⏳';

    try {
        const response = await api.QueryAIChat(state.chatHistory);
        state.chatHistory.push({ role: 'assistant', content: response });
        chatBrowser.innerHTML += `<div class="chat-bubble assistant">${renderMarkdown(response)}</div>`;
        updateBulkDeleteButtonVisibility();
    } catch (err) {
        chatBrowser.innerHTML += `<div class="chat-bubble assistant text-danger">⚠️ Помилка ШІ: ${err.message || err}</div>`;
    } finally {
        sendBtn.removeAttribute('disabled');
        sendBtn.innerText = 'Надіслати';
        chatBrowser.scrollTop = chatBrowser.scrollHeight;
    }
}

export function updateBulkDeleteButtonVisibility() {
    const connType = document.getElementById('select-conn-type').value;
    const bulkDeleteBtn = document.getElementById('btn-bulk-delete-ai');
    const chatBrowser = document.getElementById('ai-chat-browser');
    const links = chatBrowser.querySelectorAll('a.delete-link');

    if (links.length === 0) {
        bulkDeleteBtn.classList.add('hidden');
    } else {
        bulkDeleteBtn.classList.remove('hidden');
    }
}

export function escapeHtml(text) {
    return text
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#039;");
}

window.confirmSingleDeleteFromLink = function(filePath) {
    state.deletePathsQueue = [filePath];
    showDeleteConfirmationModal();
};

export function renderMarkdown(md) {
    if (!md) return "";
    let html = md;
    
    html = html.replace(/### (.*)/g, '<h3>$1</h3>');
    html = html.replace(/## (.*)/g, '<h2>$1</h2>');
    html = html.replace(/# (.*)/g, '<h1>$1</h1>');
    
    html = html.replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>');
    
    html = html.replace(/^\s*-\s+(.*)/gm, '<li>$1</li>');
    
    html = html.replace(/\[([^\]]+)\]\(delete:\/\/([^\)]+)\)/g, (match, label, urlPath) => {
        const decoded = decodeURIComponent(urlPath);
        const escaped = decoded.replace(/\\/g, '\\\\').replace(/'/g, "\\'");
        return `<a class="delete-link" href="#" onclick="confirmSingleDeleteFromLink('${escaped}'); return false;">${label}</a>`;
    });

    html = html.replace(/\[([^\]]+)\]\(action:\/\/([^\)]+)\)/g, (match, label, actionId) => {
        return `<button class="btn btn-secondary btn-sm" onclick="triggerAIAction('${actionId}', event)" style="margin-left: 10px;">⚡ ${label}</button>`;
    });

    return html;
}

window.triggerAIAction = function(actionId, event) {
    if (actionId === 'prune-docker') {
        if (window.pruneDockerSystem) window.pruneDockerSystem(event);
    } else if (actionId === 'vacuum-journald') {
        if (window.vacuumJournaldLogs) window.vacuumJournaldLogs(event);
    } else if (actionId === 'clear-event-logs') {
        if (window.clearWindowsEventLogs) window.clearWindowsEventLogs(event);
    } else if (actionId.startsWith('clear-package-cache:')) {
        const pkgName = actionId.split(':')[1].trim().toLowerCase();
        
        const sreData = state.scannedResults.sre_data;
        if (!sreData || !sreData.package_caches) {
            alert('Дані про кеші не знайдені в результатах сканування.');
            return;
        }

        const cache = sreData.package_caches.find(c => c.name.toLowerCase() === pkgName);
        if (!cache) {
            alert(`Кеш для пакетного менеджера '${pkgName}' не знайдено.`);
            return;
        }

        const connType = document.getElementById('select-conn-type').value;
        const hostID = parseInt(document.getElementById('select-ssh-server').value) || 0;
        
        if (window.clearPackageCache) {
            window.clearPackageCache(connType, hostID, cache.clean_cmd, cache.path, event);
        }
    } else {
        alert('Ця дія поки не автоматизована.');
    }
};

export function triggerAIRecommendationsCleanup() {
    const chatBrowser = document.getElementById('ai-chat-browser');
    const links = chatBrowser.querySelectorAll('a.delete-link');
    
    if (links.length === 0) {
        alert('ШІ не залишив конкретних посилань для видалення в рекомендаціях.');
        return;
    }

    const paths = [];
    links.forEach(a => {
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
