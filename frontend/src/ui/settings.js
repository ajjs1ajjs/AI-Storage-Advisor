import { state } from '../state.js';
import * as api from '../api.js';

export async function loadAIProviders() {
    try {
        const providers = await api.GetAIProviders();
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

        try {
            const lang = await api.GetSetting('ai_language');
            if (lang) {
                document.getElementById('select-ai-language').value = lang;
            }
        } catch (e) {
            console.error("Failed to load AI Language:", e);
        }

    } catch (err) {
        console.error("AI load failed:", err);
    }
}

export function renderAIProviderFormFields(type) {
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

export async function fetchAIModelsList() {
    const type = document.getElementById('select-ai-provider-type').value;
    const url = document.getElementById('input-ai-url').value.trim();
    const key = document.getElementById('input-ai-key').value.trim();
    const btn = document.getElementById('btn-fetch-models');

    btn.innerText = '⏳ Отримання...';
    btn.setAttribute('disabled', 'true');

    try {
        const models = await api.GetAIModels(type, key, url);
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

export async function testAIProviderConnection() {
    const type = document.getElementById('select-ai-provider-type').value;
    const url = document.getElementById('input-ai-url').value.trim();
    const key = document.getElementById('input-ai-key').value.trim();
    const model = document.getElementById('select-ai-model').value;
    const temp = parseFloat(document.getElementById('input-ai-temp').value) || 0.7;

    const btn = document.getElementById('btn-test-ai');
    btn.innerText = '🔌 Тестування...';

    try {
        const res = await api.TestAIProviderConnection(type, key, url, model, temp);
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

export async function saveAIProviderConfiguration() {
    const type = document.getElementById('select-ai-provider-type').value;
    const url = document.getElementById('input-ai-url').value.trim();
    const key = document.getElementById('input-ai-key').value.trim();
    const model = document.getElementById('select-ai-model').value;
    const temp = parseFloat(document.getElementById('input-ai-temp').value) || 0.7;
    const lang = document.getElementById('select-ai-language').value || 'Ukrainian';

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
        await api.SaveAIProvider(type, type, JSON.stringify(config), 1);
        await api.SaveSetting('ai_language', lang);
        alert('Налаштування провайдера ШІ збережено!');
        loadAIProviders();
    } catch (err) {
        alert(`Помилка збереження: ${err}`);
    }
}

export async function loadScanRules() {
    try {
        const rulesJSON = await api.GetScanRules();
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

export async function saveRulesConfiguration() {
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
        await api.SaveScanRules(JSON.stringify(activeRules));
        alert('Правила сканування успішно збережено!');
    } catch (err) {
        alert(`Помилка збереження правил: ${err}`);
    }
}
