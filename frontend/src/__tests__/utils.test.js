import { describe, it, expect } from 'vitest';
import { formatBytes, escapeHtml, escapeAttribute, jsArg } from '../utils.js';

describe('formatBytes', () => {
  it('should return "0 B" for 0', () => {
    expect(formatBytes(0)).toBe('0 B');
  });

  it('should format KB values', () => {
    expect(formatBytes(1024)).toBe('1 KB');
    expect(formatBytes(2048)).toBe('2 KB');
    expect(formatBytes(1536)).toBe('1.5 KB');
  });

  it('should format MB values', () => {
    expect(formatBytes(1048576)).toBe('1 MB');
    expect(formatBytes(1572864)).toBe('1.5 MB');
    expect(formatBytes(2097152)).toBe('2 MB');
  });

  it('should format GB values', () => {
    expect(formatBytes(1073741824)).toBe('1 GB');
    expect(formatBytes(1610612736)).toBe('1.5 GB');
  });
});

describe('escapeHtml', () => {
  it('should escape &', () => {
    expect(escapeHtml('&')).toBe('&amp;');
  });

  it('should escape <', () => {
    expect(escapeHtml('<')).toBe('&lt;');
  });

  it('should escape >', () => {
    expect(escapeHtml('>')).toBe('&gt;');
  });

  it('should escape double quotes', () => {
    expect(escapeHtml('"')).toBe('&quot;');
  });

  it('should escape single quotes', () => {
    expect(escapeHtml("'")).toBe('&#039;');
  });

  it('should escape combined special characters', () => {
    expect(escapeHtml('<script>"&&\'</script>')).toBe('&lt;script&gt;&quot;&amp;&amp;&#039;&lt;/script&gt;');
  });

  it('should handle null', () => {
    expect(escapeHtml(null)).toBe('');
  });

  it('should handle undefined', () => {
    expect(escapeHtml(undefined)).toBe('');
  });

  it('should handle empty string', () => {
    expect(escapeHtml('')).toBe('');
  });
});

describe('escapeAttribute', () => {
  it('should delegate to escapeHtml', () => {
    expect(escapeAttribute('<">')).toBe('&lt;&quot;&gt;');
  });

  it('should escape single quotes for attribute context', () => {
    expect(escapeAttribute("it's")).toBe('it&#039;s');
  });
});

describe('jsArg', () => {
  it('should JSON stringify a plain string', () => {
    expect(jsArg('hello')).toBe(JSON.stringify('hello'));
  });

  it('should escape < as \\u003c', () => {
    expect(jsArg('<')).toBe('"\\u003c"');
  });

  it('should escape > as \\u003e', () => {
    expect(jsArg('>')).toBe('"\\u003e"');
  });

  it('should escape & as \\u0026', () => {
    expect(jsArg('&')).toBe('"\\u0026"');
  });

  it('should escape \\u2028', () => {
    expect(jsArg('\u2028')).toBe('"\\u2028"');
  });

  it('should escape \\u2029', () => {
    expect(jsArg('\u2029')).toBe('"\\u2029"');
  });

  it('should handle null', () => {
    expect(jsArg(null)).toBe(JSON.stringify(''));
  });

  it('should handle undefined', () => {
    expect(jsArg(undefined)).toBe(JSON.stringify(''));
  });
});
