import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

describe('VirtualScroller', () => {
    let container;

    beforeEach(() => {
        container = document.createElement('div');
        container.style.height = '400px';
        document.body.appendChild(container);
    });

    afterEach(() => {
        document.body.removeChild(container);
    });

    it('should create scroller with container', async () => {
        const { VirtualScroller } = await import('../ui/virtual-scroll.js');
        const vs = new VirtualScroller({ container, itemHeight: 40 });
        expect(vs.container).toBe(container);
        expect(vs.itemHeight).toBe(40);
        expect(vs.buffer).toBe(5);
        vs.destroy();
    });

    it('should render visible items', async () => {
        const { VirtualScroller } = await import('../ui/virtual-scroll.js');
        const items = Array.from({ length: 100 }, (_, i) => ({ id: i, name: `File ${i}` }));

        const vs = new VirtualScroller({
            container,
            itemHeight: 40,
            renderItem: (item) => {
                const div = document.createElement('div');
                div.textContent = item.name;
                div.className = 'file-row';
                return div;
            }
        });

        vs.setItems(items);
        vs.flush();

        const rows = container.querySelectorAll('.file-row');
        expect(rows.length).toBeGreaterThan(0);
        expect(rows.length).toBeLessThanOrEqual(25);

        vs.destroy();
    });

    it('should update on scroll', async () => {
        const { VirtualScroller } = await import('../ui/virtual-scroll.js');
        const items = Array.from({ length: 1000 }, (_, i) => ({ id: i, name: `File ${i}` }));

        const vs = new VirtualScroller({
            container,
            itemHeight: 40,
            renderItem: (item) => {
                const div = document.createElement('div');
                div.textContent = item.name;
                div.className = 'file-row';
                return div;
            }
        });

        vs.setItems(items);
        vs.flush();

        container.scrollTop = 2000;
        container.dispatchEvent(new Event('scroll'));
        vs.flush();

        const rows = container.querySelectorAll('.file-row');
        const firstText = rows[0]?.textContent || '';
        expect(firstText).toMatch(/File (4[5-9]|5[0-5])/);

        vs.destroy();
    });

    it('should handle empty items', async () => {
        const { VirtualScroller } = await import('../ui/virtual-scroll.js');
        const vs = new VirtualScroller({ container });
        vs.setItems([]);
        vs.flush();
        expect(container.querySelector('.file-row')).toBeNull();
        vs.destroy();
    });

    it('should destroy cleanly', async () => {
        const { VirtualScroller } = await import('../ui/virtual-scroll.js');
        const vs = new VirtualScroller({ container, itemHeight: 40 });
        vs.setItems([{ id: 1, name: 'test' }]);
        vs.destroy();
        expect(container.children.length).toBe(0);
    });

    it('should scroll to index', async () => {
        const { VirtualScroller } = await import('../ui/virtual-scroll.js');
        const items = Array.from({ length: 500 }, (_, i) => ({ id: i, name: `File ${i}` }));
        const vs = new VirtualScroller({
            container,
            itemHeight: 40,
            renderItem: (item) => {
                const div = document.createElement('div');
                div.textContent = item.name;
                div.className = 'file-row';
                return div;
            }
        });
        vs.setItems(items);
        vs.scrollToIndex(100);
        expect(container.scrollTop).toBe(4000);
        vs.destroy();
    });
});
