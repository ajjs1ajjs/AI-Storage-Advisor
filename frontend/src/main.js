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
    BrowseFolder,
    QueryAIChat,
    ClearContainerLogs,
    ClearPackageCache,
    IsVaultInitialized,
    IsVaultUnlocked,
    UnlockVault,
    InitializeVault,
    LockVault,
    ChangeMasterPassword
} from './api.js';

import { state } from './state.js';
import { formatBytes } from './utils.js';
import * as ssh from './ui/ssh.js';
import * as settings from './ui/settings.js';
import * as scanner from './ui/scanner.js';

// Initialisations on DOM Loaded
document.addEventListener('DOMContentLoaded', () => {
    initApp();
    setupEventListeners();
    scanner.setupWailsEvents(loadRecentScans);
});

async function initApp() {
    // 1. Theme configuration
    state.theme = await GetTheme();
    applyTheme(state.theme);

    // 2. Check vault status
    const vaultInit = await IsVaultInitialized();
    const vaultUnlocked = await IsVaultUnlocked();

    if (!vaultUnlocked) {
        if (vaultInit) {
            showVaultUnlockModal();
        } else {
            showVaultInitModal();
        }
    } else {
        // Vault already unlocked — load everything
        await loadVaultDependentData();
    }

    // 3. Load recent scans on dashboard
    loadRecentScans();
}

async function loadVaultDependentData() {
    await settings.loadAIProviders();
    await settings.loadScanRules();
    await ssh.loadSSHHosts();
}

// Vault unlock modal
async function showVaultUnlockModal() {
    const overlay = document.getElementById('modal-vault-overlay');
    document.getElementById('vault-modal-title').innerText = '🔐 Розблокувати сховище';
    document.getElementById('vault-modal-desc').innerText = 'Введіть master-пароль для доступу до збережених облікових даних (SSH, AI ключі).';
    document.getElementById('vault-confirm-group').classList.add('hidden');
    document.getElementById('vault-error-msg').style.display = 'none';
    overlay.classList.remove('hidden');

    document.getElementById('btn-vault-submit').onclick = async () => {
        const password = document.getElementById('input-vault-password').value;
        document.getElementById('vault-error-msg').style.display = 'none';
        try {
            await UnlockVault(password);
            overlay.classList.add('hidden');
            document.getElementById('input-vault-password').value = '';
            await loadVaultDependentData();
            loadRecentScans();
        } catch (err) {
            document.getElementById('vault-error-msg').innerText = 'Помилка: ' + (err.message || err);
            document.getElementById('vault-error-msg').style.display = 'block';
        }
    };

    document.getElementById('input-vault-password').onkeydown = (e) => {
        if (e.key === 'Enter') {
            document.getElementById('btn-vault-submit').click();
        }
    };

    setTimeout(() => document.getElementById('input-vault-password').focus(), 100);
}

// Vault initialization modal (first run)
async function showVaultInitModal() {
    const overlay = document.getElementById('modal-vault-overlay');
    document.getElementById('vault-modal-title').innerText = '🔐 Створити master-пароль';
    document.getElementById('vault-modal-desc').innerText = 'Це перший запуск. Створіть master-пароль для захисту збережених облікових даних (SSH, AI ключі).';
    document.getElementById('vault-confirm-group').classList.remove('hidden');
    document.getElementById('vault-error-msg').style.display = 'none';
    overlay.classList.remove('hidden');

    document.getElementById('btn-vault-submit').onclick = async () => {
        const password = document.getElementById('input-vault-password').value;
        const confirm = document.getElementById('input-vault-password-confirm').value;
        document.getElementById('vault-error-msg').style.display = 'none';

        if (password.length < 8) {
            document.getElementById('vault-error-msg').innerText = 'Помилка: пароль повинен бути щонайменше 8 символів.';
            document.getElementById('vault-error-msg').style.display = 'block';
            return;
        }
        if (password !== confirm) {
            document.getElementById('vault-error-msg').innerText = 'Помилка: паролі не співпадають.';
            document.getElementById('vault-error-msg').style.display = 'block';
            return;
        }
        try {
            await InitializeVault(password);
            overlay.classList.add('hidden');
            document.getElementById('input-vault-password').value = '';
            document.getElementById('input-vault-password-confirm').value = '';
            await loadVaultDependentData();
            loadRecentScans();
        } catch (err) {
            document.getElementById('vault-error-msg').innerText = 'Помилка: ' + (err.message || err);
            document.getElementById('vault-error-msg').style.display = 'block';
        }
    };

    document.getElementById('input-vault-password').onkeydown = (e) => {
        if (e.key === 'Enter') {
            document.getElementById('btn-vault-submit').click();
        }
    };
    document.getElementById('input-vault-password-confirm').onkeydown = (e) => {
        if (e.key === 'Enter') {
            document.getElementById('btn-vault-submit').click();
        }
    };

    setTimeout(() => document.getElementById('input-vault-password').focus(), 100);
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
        ssh.renderSSHHostsTable();
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
        scanner.onConnectionTypeChanged(e.target.value);
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
    document.getElementById('btn-toggle-scan').addEventListener('click', scanner.toggleScan);

    // AI recommendation button
    document.getElementById('btn-generate-ai').addEventListener('click', scanner.generateAIRecommendation);

    // Bulk deletion AI click
    document.getElementById('btn-bulk-delete-ai').addEventListener('click', scanner.triggerAIRecommendationsCleanup);

    // Send AI Chat message
    document.getElementById('btn-send-ai-chat').addEventListener('click', scanner.sendAIChatMessage);
    document.getElementById('input-ai-chat').addEventListener('keydown', (e) => {
        if (e.key === 'Enter') {
            scanner.sendAIChatMessage();
        }
    });

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

    // AI settings
    document.getElementById('select-ai-provider-type').addEventListener('change', (e) => {
        settings.renderAIProviderFormFields(e.target.value);
    });
    document.getElementById('btn-fetch-models').addEventListener('click', settings.fetchAIModelsList);
    document.getElementById('btn-test-ai').addEventListener('click', settings.testAIProviderConnection);
    document.getElementById('btn-save-ai').addEventListener('click', settings.saveAIProviderConfiguration);

    // Rules saves
    document.getElementById('btn-save-rules').addEventListener('click', settings.saveRulesConfiguration);

    // SSH Hosts CRUD clicks
    document.getElementById('btn-add-ssh').addEventListener('click', ssh.openSSHHostEditorAdd);
    document.getElementById('btn-ssh-cancel').addEventListener('click', ssh.closeSSHHostEditor);
    document.getElementById('btn-ssh-save').addEventListener('click', ssh.saveSSHHostDetails);
    document.getElementById('select-ssh-editor-auth').addEventListener('change', (e) => {
        ssh.onSSHAuthTypeChanged(e.target.value);
    });

    // SSH active dropdown loader change
    document.getElementById('select-ssh-server').addEventListener('change', scanner.onSSHActiveDropdownChanged);

    // Deletion Modal cancellations/confirms
    document.getElementById('btn-delete-cancel').addEventListener('click', () => {
        document.getElementById('modal-delete-confirm').classList.add('hidden');
    });
    document.getElementById('btn-delete-execute').addEventListener('click', scanner.executeDeletionsQueue);
}

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
