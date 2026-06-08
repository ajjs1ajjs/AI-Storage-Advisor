import { describe, it, expect, beforeEach } from 'vitest';
import { state } from '../state.js';

describe('initial state', () => {
  it('should have the correct default structure', () => {
    expect(state).toEqual({
      theme: 'dark',
      currentTab: 'dashboard',
      scannedResults: null,
      savedSSHHosts: [],
      savedAIProviders: [],
      activeAIProvider: null,
      scanRules: [],
      deletePathsQueue: [],
      editingSSHId: null,
      chatHistory: [],
      aiProviders: [],
    });
  });
});

describe('state mutations', () => {
  beforeEach(() => {
    state.currentTab = 'dashboard';
    state.scannedResults = null;
    state.deletePathsQueue = [];
    state.chatHistory = [];
    state.editingSSHId = null;
    state.activeAIProvider = null;
    state.savedSSHHosts = [];
    state.savedAIProviders = [];
    state.scanRules = [];
  });

  it('should allow changing currentTab', () => {
    state.currentTab = 'settings';
    expect(state.currentTab).toBe('settings');
  });

  it('should allow setting scannedResults', () => {
    const results = { files_scanned: 100, total_size: 2048 };
    state.scannedResults = results;
    expect(state.scannedResults).toEqual(results);
  });

  it('should allow modifying deletePathsQueue', () => {
    state.deletePathsQueue = ['/path/to/file1', '/path/to/file2'];
    expect(state.deletePathsQueue).toEqual(['/path/to/file1', '/path/to/file2']);
    state.deletePathsQueue.push('/path/to/file3');
    expect(state.deletePathsQueue).toHaveLength(3);
  });

  it('should allow adding items to chatHistory', () => {
    state.chatHistory.push({ role: 'user', content: 'hello' });
    expect(state.chatHistory).toHaveLength(1);
    expect(state.chatHistory[0].role).toBe('user');
  });

  it('should allow setting savedSSHHosts', () => {
    const hosts = [{ id: 1, host: '192.168.1.1', username: 'root' }];
    state.savedSSHHosts = hosts;
    expect(state.savedSSHHosts).toEqual(hosts);
  });
});
