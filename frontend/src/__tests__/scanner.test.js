import { vi, describe, it, expect } from 'vitest';

vi.mock('../../wailsjs/go/main/App.js', () => ({
  CalculateHealthScore: vi.fn(),
}));

vi.mock('../../wailsjs/runtime/runtime.js', () => ({
  EventsOn: vi.fn(),
}));

vi.mock('../api.js', () => ({}));

vi.mock('../ui/ssh.js', () => ({
  loadSSHHostsDropdown: vi.fn(),
}));

import { formatBytes } from '../utils.js';
import { renderMarkdown } from '../ui/scanner.js';

describe('formatBytes (re-exported from utils)', () => {
  it('should format 0 bytes', () => {
    expect(formatBytes(0)).toBe('0 B');
  });

  it('should format MB values', () => {
    expect(formatBytes(1048576)).toBe('1 MB');
  });

  it('should format GB values', () => {
    expect(formatBytes(1073741824)).toBe('1 GB');
  });
});

describe('renderMarkdown', () => {
  it('should return empty string for null/undefined/empty input', () => {
    expect(renderMarkdown(null)).toBe('');
    expect(renderMarkdown(undefined)).toBe('');
    expect(renderMarkdown('')).toBe('');
  });

  it('should render h1 headers', () => {
    const result = renderMarkdown('# Hello World');
    expect(result).toBe('<h1>Hello World</h1>');
  });

  it('should render h2 headers', () => {
    const result = renderMarkdown('## Section Title');
    expect(result).toBe('<h2>Section Title</h2>');
  });

  it('should render h3 headers', () => {
    const result = renderMarkdown('### Sub Section');
    expect(result).toBe('<h3>Sub Section</h3>');
  });

  it('should render bold text', () => {
    expect(renderMarkdown('**bold**')).toBe('<strong>bold</strong>');
    expect(renderMarkdown('**bold** and **also**')).toBe('<strong>bold</strong> and <strong>also</strong>');
  });

  it('should render list items', () => {
    const result = renderMarkdown('- item one\n- item two\n- item three');
    expect(result).toContain('<li>item one</li>');
    expect(result).toContain('<li>item two</li>');
    expect(result).toContain('<li>item three</li>');
  });

  it('should render delete:// links', () => {
    const result = renderMarkdown('[Delete file](delete:///tmp/cache.dat)');
    expect(result).toContain('<a class="delete-link"');
    expect(result).toContain('href="#"');
    expect(result).toContain('data-delete-path="/tmp/cache.dat"');
    expect(result).toContain('Delete file');
  });

  it('should render delete:// links with encoded paths', () => {
    const result = renderMarkdown('[Delete](delete:///path%20with%20spaces)');
    expect(result).toContain('data-delete-path="/path with spaces"');
  });

  it('should render action:// links', () => {
    const result = renderMarkdown('[Prune Docker](action://prune-docker)');
    expect(result).toContain('<button class="btn btn-secondary btn-sm"');
    expect(result).toContain('Prune Docker');
  });

  it('should escape HTML in markdown content', () => {
    const result = renderMarkdown('<script>alert("xss")</script>');
    expect(result).not.toContain('<script>');
    expect(result).toContain('&lt;script&gt;');
  });

  it('should handle mixed markdown with headers and bold', () => {
    const result = renderMarkdown('# **Header** with bold');
    expect(result).toBe('<h1><strong>Header</strong> with bold</h1>');
  });

  it('should handle delete:// link with special characters in path', () => {
    const result = renderMarkdown('[Delete](delete:///path/with/&/symbols)');
    expect(result).toContain('data-delete-path');
    expect(result).toContain('Delete');
  });
});
