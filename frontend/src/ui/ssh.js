import { state } from '../state.js';
import * as api from '../api.js';
import { escapeHtml } from '../utils.js';

export async function loadSSHHosts() {
    try {
        const hosts = await api.GetSSHHosts();
        state.savedSSHHosts = hosts;
        renderSSHHostsTable();
    } catch (err) {
        console.error("SSH load failed:", err);
    }
}

export function loadSSHHostsDropdown() {
    const select = document.getElementById('select-ssh-server');
    if (!select) return;
    select.innerHTML = '<option value="">-- Пряме введення --</option>';
    state.savedSSHHosts.forEach(h => {
        select.innerHTML += `<option value="${escapeHtml(h.id)}">${escapeHtml(h.name)} (${escapeHtml(h.host)})</option>`;
    });
}

export function renderSSHHostsTable() {
    const tbody = document.querySelector('#table-ssh-hosts tbody');
    if (!tbody) return;
    tbody.innerHTML = '';

    if (state.savedSSHHosts.length === 0) {
        tbody.innerHTML = '<tr><td colspan="6" class="text-center" style="color: var(--text-muted); font-style: italic;">Немає збережених хостів</td></tr>';
        return;
    }

    state.savedSSHHosts.forEach(h => {
        tbody.innerHTML += `<tr>
            <td>${escapeHtml(h.name)}</td>
            <td>${escapeHtml(h.host)}</td>
            <td>${escapeHtml(h.port)}</td>
            <td>${escapeHtml(h.username)}</td>
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

export function openSSHHostEditorEdit(id) {
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
    document.getElementById('input-ssh-editor-passphrase').value = host.key_passphrase || '';

    onSSHAuthTypeChanged(host.auth_type);
    document.getElementById('modal-ssh-editor').classList.remove('hidden');
}

export function openSSHHostEditorAdd() {
    state.editingSSHId = null;
    document.getElementById('ssh-modal-title').innerText = 'Додати SSH хост';
    document.getElementById('input-ssh-name').value = '';
    document.getElementById('input-ssh-editor-host').value = '';
    document.getElementById('input-ssh-editor-port').value = '22';
    document.getElementById('input-ssh-editor-user').value = 'root';
    document.getElementById('select-ssh-editor-auth').value = 'password';
    document.getElementById('input-ssh-editor-cred').value = '';
    document.getElementById('input-ssh-editor-passphrase').value = '';

    onSSHAuthTypeChanged('password');
    document.getElementById('modal-ssh-editor').classList.remove('hidden');
}

export function closeSSHHostEditor() {
    document.getElementById('modal-ssh-editor').classList.add('hidden');
}

export function onSSHAuthTypeChanged(authType) {
    const label = document.getElementById('lbl-ssh-editor-cred');
    const input = document.getElementById('input-ssh-editor-cred');
    const passGroup = document.getElementById('ssh-editor-passphrase-group');
    if (!label || !input) return;

    if (authType === 'password') {
        label.innerText = 'Пароль віддаленого сервера:';
        input.placeholder = 'Введіть пароль';
        if (passGroup) passGroup.classList.add('hidden');
    } else {
        label.innerText = 'Шлях до файлу приватного ключа:';
        input.placeholder = 'напр. C:\\Users\\Admin\\.ssh\\id_rsa';
        if (passGroup) passGroup.classList.remove('hidden');
    }
}

export async function saveSSHHostDetails() {
    const name = document.getElementById('input-ssh-name').value.trim();
    const host = document.getElementById('input-ssh-editor-host').value.trim();
    const port = parseInt(document.getElementById('input-ssh-editor-port').value) || 22;
    const username = document.getElementById('input-ssh-editor-user').value.trim();
    const authType = document.getElementById('select-ssh-editor-auth').value;
    const credentials = document.getElementById('input-ssh-editor-cred').value.trim();
    const passphrase = document.getElementById('input-ssh-editor-passphrase') ? document.getElementById('input-ssh-editor-passphrase').value : '';

    if (!name || !host || !username) {
        alert('Помилка: Будь ласка, заповніть назву, хост та користувача!');
        return;
    }

    try {
        if (state.editingSSHId) {
            await api.EditSSHHost(state.editingSSHId, name, host, port, username, authType, credentials, passphrase);
        } else {
            await api.AddSSHHost(name, host, port, username, authType, credentials, passphrase);
        }
        closeSSHHostEditor();
        loadSSHHosts();
    } catch (err) {
        alert(`Помилка збереження хоста: ${err}`);
    }
}

export async function deleteSSHHostItem(id) {
    const conf = confirm("Ви впевнені, що хочете видалити цей SSH-хост?");
    if (!conf) return;

    try {
        await api.DeleteSSHHost(id);
        loadSSHHosts();
    } catch (err) {
        alert(`Помилка видалення: ${err}`);
    }
}

// Bind to window for inline HTML onclick handlers
window.openSSHHostEditorEdit = openSSHHostEditorEdit;
window.deleteSSHHostItem = deleteSSHHostItem;
