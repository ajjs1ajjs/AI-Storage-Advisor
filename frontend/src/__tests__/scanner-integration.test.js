import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest';

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

function makeFile(index) {
  return {
    path: `/path/to/file_${index}.txt`,
    size: 1024 * (index % 100),
    size_formatted: `${(index % 100)} KB`,
    modified: '2026-01-01',
    category: 'logs',
    rule_match: index % 3 === 0 ? 'log file' : '-',
    flagged: index % 10 === 0,
  };
}

function createTableDom(tableId) {
  const container = document.createElement('div');
  container.className = 'table-container';
  container.style.height = '400px';

  const table = document.createElement('table');
  table.id = tableId;

  const thead = document.createElement('thead');
  const tr = document.createElement('tr');
  ['Шлях до файлу', 'Розмір', 'Причина', ''].forEach((text, i) => {
    const th = document.createElement('th');
    th.textContent = text;
    if (i === 3) th.className = 'col-action';
    tr.appendChild(th);
  });
  thead.appendChild(tr);
  table.appendChild(thead);

  table.appendChild(document.createElement('tbody'));
  container.appendChild(table);
  document.body.appendChild(container);

  return container;
}

/**
 * In jsdom, clientHeight is 0 for elements not laid out by a browser.
 * This helper patches it so VirtualScroller can compute visible range.
 */
function setClientHeight(el, height) {
  Object.defineProperty(el, 'clientHeight', {
    value: height,
    configurable: true,
    writable: false,
  });
}

/**
 * Wait for the next requestAnimationFrame callback to fire.
 * Uses the real RAF so VirtualScroller's async _render completes.
 */
function waitRaf() {
  return new Promise(resolve => requestAnimationFrame(resolve));
}

/**
 * Normalize a CSS style string (collapse whitespace, trim) for assertion.
 */
function normStyle(cssText) {
  return cssText.replace(/\s+/g, ' ').trim();
}

describe('VirtualScroller integration with scanner.js', () => {
  afterEach(() => {
    vi.resetModules();
  });

  describe('VirtualScroller class directly (with scanner.js parameters)', () => {
    let container;

    beforeEach(() => {
      container = document.createElement('div');
      container.className = 'table-container';
      container.style.height = '400px';
      setClientHeight(container, 400);
      document.body.appendChild(container);
    });

    afterEach(() => {
      if (container && container.parentNode) {
        document.body.removeChild(container);
      }
    });

    it('should create VirtualScroller and render items', async () => {
      const { VirtualScroller } = await import('../ui/virtual-scroll.js');

      const vs = new VirtualScroller({
        container,
        noOverflowX: true,
        itemHeight: 40,
        renderItem: (item) => {
          const row = document.createElement('div');
          row.className = 'file-row';
          if (item.flagged) row.classList.add('flagged');
          row.textContent = item.path;
          return row;
        },
      });

      const files = Array.from({ length: 1000 }, (_, i) => makeFile(i));
      vs.setItems(files);
      await waitRaf();

      const rows = container.querySelectorAll('.file-row');
      expect(rows.length).toBeGreaterThan(0);
      expect(rows.length).toBeLessThan(30);

      expect(rows[0].textContent).toContain('file_0');

      const flaggedRows = container.querySelectorAll('.file-row.flagged');
      expect(flaggedRows.length).toBeGreaterThan(0);

      vs.destroy();
    });

    it('should render first and last items when scrolled to extremes', async () => {
      const { VirtualScroller } = await import('../ui/virtual-scroll.js');

      const vs = new VirtualScroller({
        container,
        noOverflowX: true,
        itemHeight: 40,
        renderItem: (item) => {
          const row = document.createElement('div');
          row.className = 'file-row';
          row.textContent = `file_${item.id}`;
          return row;
        },
      });

      const items = Array.from({ length: 500 }, (_, i) => ({ id: i }));
      vs.setItems(items);
      await waitRaf();

      let rows = container.querySelectorAll('.file-row');
      const firstText = rows[0].textContent;
      expect(firstText).toBe('file_0');

      vs.flush();
      container.scrollTop = 40 * 400;
      container.dispatchEvent(new Event('scroll'));
      await waitRaf();

      rows = container.querySelectorAll('.file-row');
      const scrolledText = rows[0].textContent;
      expect(scrolledText).not.toBe(firstText);
      expect(scrolledText).toMatch(/file_39[0-9]/);

      vs.destroy();
    });

    it('should handle empty file list', async () => {
      const { VirtualScroller } = await import('../ui/virtual-scroll.js');

      const vs = new VirtualScroller({
        container,
        noOverflowX: true,
        itemHeight: 40,
        renderItem: (item) => {
          const div = document.createElement('div');
          div.className = 'file-row';
          div.textContent = item.path;
          return div;
        },
      });

      vs.setItems([]);
      await waitRaf();

      const rows = container.querySelectorAll('.file-row');
      expect(rows.length).toBe(0);

      vs.destroy();
    });

    it('should update items when setItems is called again', async () => {
      const { VirtualScroller } = await import('../ui/virtual-scroll.js');

      const vs = new VirtualScroller({
        container,
        noOverflowX: true,
        itemHeight: 40,
        renderItem: (item) => {
          const div = document.createElement('div');
          div.className = 'file-row';
          div.textContent = item.path;
          return div;
        },
      });

      vs.setItems([{ path: '/first.txt' }]);
      await waitRaf();
      let rows = container.querySelectorAll('.file-row');
      expect(rows.length).toBe(1);
      expect(rows[0].textContent).toBe('/first.txt');

      vs.setItems([{ path: '/second.txt' }, { path: '/third.txt' }]);
      await waitRaf();
      rows = container.querySelectorAll('.file-row');
      expect(rows.length).toBe(2);
      expect(rows[0].textContent).toBe('/second.txt');

      vs.destroy();
    });

    it('should dynamically update visible count via setItemHeight', async () => {
      const { VirtualScroller } = await import('../ui/virtual-scroll.js');

      const vs = new VirtualScroller({
        container,
        noOverflowX: true,
        itemHeight: 60,
        renderItem: (item) => {
          const div = document.createElement('div');
          div.className = 'file-row';
          div.textContent = `Item ${item.id}`;
          return div;
        },
      });

      const items = Array.from({ length: 50 }, (_, i) => ({ id: i }));
      vs.setItems(items);
      await waitRaf();

      // 400px / 60px ≈ 6 visible + buffer ≈ 16 items
      let rows = container.querySelectorAll('.file-row');
      expect(rows.length).toBeGreaterThan(5);
      expect(rows.length).toBeLessThanOrEqual(20);

      vs.setItemHeight(20);
      await waitRaf();

      // 400px / 20px ≈ 20 visible + buffer ≈ 30 items
      rows = container.querySelectorAll('.file-row');
      expect(rows.length).toBeGreaterThan(10);
      expect(rows.length).toBeLessThanOrEqual(50);

      vs.destroy();
    });

    it('should update visible items on scroll', async () => {
      const { VirtualScroller } = await import('../ui/virtual-scroll.js');

      const vs = new VirtualScroller({
        container,
        noOverflowX: true,
        itemHeight: 40,
        renderItem: (item) => {
          const div = document.createElement('div');
          div.className = 'file-row';
          div.textContent = `file_${item.id}`;
          return div;
        },
      });

      const items = Array.from({ length: 500 }, (_, i) => ({ id: i }));
      vs.setItems(items);
      await waitRaf();

      let rows = container.querySelectorAll('.file-row');
      const firstText = rows[0].textContent;

      container.scrollTop = 40 * 400;
      container.dispatchEvent(new Event('scroll'));
      await waitRaf();

      rows = container.querySelectorAll('.file-row');
      const scrolledText = rows[0].textContent;
      expect(scrolledText).not.toBe(firstText);
      expect(scrolledText).toMatch(/file_39[0-9]/);

      vs.destroy();
    });

    it('should call onScroll callback when provided', async () => {
      const { VirtualScroller } = await import('../ui/virtual-scroll.js');
      const onScroll = vi.fn();

      const vs = new VirtualScroller({
        container,
        noOverflowX: true,
        itemHeight: 40,
        renderItem: (item) => {
          const div = document.createElement('div');
          div.className = 'file-row';
          div.textContent = item.path;
          return div;
        },
        onScroll,
      });

      vs.setItems(Array.from({ length: 100 }, (_, i) => ({ path: `file_${i}` })));
      await waitRaf();

      expect(onScroll).toHaveBeenCalledTimes(1);
      expect(onScroll).toHaveBeenCalledWith(
        expect.objectContaining({
          startIdx: expect.any(Number),
          endIdx: expect.any(Number),
          scrollTop: expect.any(Number),
        }),
      );

      vs.destroy();
    });

    it('should not set overflowX when noOverflowX is true', async () => {
      const { VirtualScroller } = await import('../ui/virtual-scroll.js');

      const vs = new VirtualScroller({
        container,
        noOverflowX: true,
        itemHeight: 40,
        renderItem: () => document.createElement('div'),
      });

      expect(container.style.overflowY).toBe('auto');
      expect(container.style.position).toBe('relative');
      expect(container.style.overflowX).toBe('');

      vs.destroy();
    });

    it('should set overflowX hidden when noOverflowX is falsy', async () => {
      const { VirtualScroller } = await import('../ui/virtual-scroll.js');

      const vs = new VirtualScroller({
        container,
        itemHeight: 40,
        renderItem: () => document.createElement('div'),
      });

      expect(container.style.overflowX).toBe('hidden');

      vs.destroy();
    });

    it('should position rendered items absolutely', async () => {
      const { VirtualScroller } = await import('../ui/virtual-scroll.js');

      const vs = new VirtualScroller({
        container,
        noOverflowX: true,
        itemHeight: 40,
        renderItem: (item, index) => {
          const div = document.createElement('div');
          div.className = 'file-row';
          div.textContent = `Row ${index}`;
          return div;
        },
      });

      vs.setItems(Array.from({ length: 50 }, (_, i) => ({ id: i })));
      await waitRaf();

      const rows = container.querySelectorAll('.file-row');
      rows.forEach((row, i) => {
        expect(row.style.position).toBe('absolute');
        expect(row.style.top).toBe(`${i * 40}px`);
      });

      vs.destroy();
    });

    it('should update spacer height based on item count', async () => {
      const { VirtualScroller } = await import('../ui/virtual-scroll.js');

      const vs = new VirtualScroller({
        container,
        noOverflowX: true,
        itemHeight: 40,
        renderItem: () => document.createElement('div'),
      });

      vs.setItems(Array.from({ length: 50 }, (_, i) => ({ id: i })));
      expect(vs.spacer.style.height).toBe('2000px');

      vs.setItems(Array.from({ length: 100 }, (_, i) => ({ id: i })));
      expect(vs.spacer.style.height).toBe('4000px');

      vs.destroy();
    });

    it('should clear spacer and viewport on destroy', async () => {
      const { VirtualScroller } = await import('../ui/virtual-scroll.js');

      const vs = new VirtualScroller({
        container,
        noOverflowX: true,
        itemHeight: 40,
        renderItem: () => document.createElement('div'),
      });

      const spacer = vs.spacer;
      const viewport = vs.viewport;
      expect(container.contains(spacer)).toBe(true);
      expect(container.contains(viewport)).toBe(true);

      vs.destroy();

      expect(container.contains(spacer)).toBe(false);
      expect(container.contains(viewport)).toBe(false);
    });
  });

  describe('populateFilesTable integration', () => {
    let nextId = 1;

    function uniqueTableId() {
      return `table-large-files-${nextId++}`;
    }

    afterEach(() => {
      const remnants = document.querySelectorAll('.table-container');
      remnants.forEach(el => {
        if (el.parentNode) el.parentNode.removeChild(el);
      });
    });

    it('should replace table with virtual scroller header and viewport', async () => {
      const { populateFilesTable } = await import('../ui/scanner.js');
      const tableId = uniqueTableId();
      const container = createTableDom(tableId);

      const table = document.getElementById(tableId);
      expect(table).toBeTruthy();

      populateFilesTable(tableId, Array.from({ length: 500 }, (_, i) => makeFile(i)));
      await waitRaf();

      const tableAfter = document.getElementById(tableId);
      expect(tableAfter).toBeNull();

      // After populateFilesTable, container children are:
      //   [0] sticky header div
      //   [1] scroll spacer div
      //   [2] viewport div (position: absolute)
      expect(container.children.length).toBe(3);

      const headerDiv = container.children[0];
      expect(headerDiv.children.length).toBe(4);

      const viewport = container.children[2];
      expect(viewport.children.length).toBeGreaterThan(0);
      expect(viewport.children.length).toBeLessThan(30);

      const firstRow = viewport.children[0];
      expect(firstRow.style.display).toBe('grid');
      expect(firstRow.children.length).toBe(4);
      expect(firstRow.children[0].textContent).toContain('file_0');
      expect(firstRow.children[1].textContent).toMatch(/KB/);

      document.body.removeChild(container);
    });

    it('should reuse VirtualScroller instance on subsequent calls', async () => {
      const { populateFilesTable } = await import('../ui/scanner.js');
      const tableId = uniqueTableId();
      const container = createTableDom(tableId);

      const files1 = Array.from({ length: 100 }, (_, i) => makeFile(i));
      populateFilesTable(tableId, files1);
      await waitRaf();

      const viewport = container.children[2];
      const firstCallPath = viewport.children[0].children[0].textContent;
      expect(firstCallPath).toContain('file_0');

      const files2 = Array.from({ length: 50 }, (_, i) => makeFile(i + 200));
      populateFilesTable(tableId, files2);
      await waitRaf();

      expect(viewport.children.length).toBeGreaterThan(0);
      const firstPath = viewport.children[0].children[0].textContent;
      expect(firstPath).toContain('file_200');

      document.body.removeChild(container);
    });

    it('should show empty message when no files exist', async () => {
      const { populateFilesTable } = await import('../ui/scanner.js');
      const tableId = uniqueTableId();
      const container = createTableDom(tableId);

      populateFilesTable(tableId, []);
      await waitRaf();

      const tbody = container.querySelector('tbody');
      expect(tbody).toBeTruthy();
      expect(tbody.innerHTML).toContain('Файлів не знайдено');

      document.body.removeChild(container);
    });

    it('should render empty state when existing instance gets empty list', async () => {
      const { populateFilesTable } = await import('../ui/scanner.js');
      const tableId = uniqueTableId();
      const container = createTableDom(tableId);

      populateFilesTable(tableId, Array.from({ length: 10 }, (_, i) => makeFile(i)));
      await waitRaf();

      populateFilesTable(tableId, []);
      await waitRaf();

      const viewport = container.children[2];
      expect(viewport.innerHTML).toBe('');

      document.body.removeChild(container);
    });

    it('should render rule_match metadata in each row', async () => {
      const { populateFilesTable } = await import('../ui/scanner.js');
      const tableId = uniqueTableId();
      const container = createTableDom(tableId);

      const files = [makeFile(0), makeFile(1)];
      populateFilesTable(tableId, files);
      await waitRaf();

      const viewport = container.children[2];
      expect(viewport.children[0].children[2].textContent.trim()).toBe('log file');
      expect(viewport.children[1].children[2].textContent.trim()).toBe('-');

      document.body.removeChild(container);
    });

    it('should include a delete button in the action column of each row', async () => {
      const { populateFilesTable } = await import('../ui/scanner.js');
      const tableId = uniqueTableId();
      const container = createTableDom(tableId);

      populateFilesTable(tableId, Array.from({ length: 5 }, (_, i) => makeFile(i)));
      await waitRaf();

      const viewport = container.children[2];
      const firstRow = viewport.children[0];
      const actionCell = firstRow.children[3];
      const deleteBtn = actionCell.querySelector('.btn-icon.delete');
      expect(deleteBtn).toBeTruthy();
      expect(deleteBtn.innerHTML).toContain('🗑️');

      document.body.removeChild(container);
    });

    it('should set container overflow styles via VirtualScroller constructor', async () => {
      const { populateFilesTable } = await import('../ui/scanner.js');
      const tableId = uniqueTableId();
      const container = createTableDom(tableId);

      populateFilesTable(tableId, Array.from({ length: 10 }, (_, i) => makeFile(i)));
      await waitRaf();

      expect(container.style.overflowY).toBe('auto');
      expect(container.style.position).toBe('relative');
      expect(container.style.overflowX).toBe('');

      document.body.removeChild(container);
    });

    it('should support two independent table instances simultaneously', async () => {
      const { populateFilesTable } = await import('../ui/scanner.js');
      const tableId1 = uniqueTableId();
      const tableId2 = uniqueTableId();
      const container1 = createTableDom(tableId1);
      const container2 = createTableDom(tableId2);

      populateFilesTable(tableId1, Array.from({ length: 100 }, (_, i) => makeFile(i)));
      populateFilesTable(tableId2, Array.from({ length: 50 }, (_, i) => makeFile(i)));
      await waitRaf();

      expect(container1.children.length).toBe(3);
      expect(container2.children.length).toBe(3);
      expect(container1.children[2].children.length).toBeGreaterThan(0);
      expect(container2.children[2].children.length).toBeGreaterThan(0);

      document.body.removeChild(container1);
      document.body.removeChild(container2);
    });
  });
});
