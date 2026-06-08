export class VirtualScroller {
    constructor(options = {}) {
        this.container = options.container;
        this.items = [];
        this.renderItem = options.renderItem || (() => '');
        this.itemHeight = options.itemHeight || 40;
        this.buffer = options.buffer || 5;
        this.onScroll = options.onScroll || null;

        this.container.style.overflowY = 'auto';
        this.container.style.position = 'relative';
        if (options.noOverflowX !== true) {
            this.container.style.overflowX = 'hidden';
        }
        if (options.style) {
            this.container.style.cssText += options.style;
        }

        this.spacer = document.createElement('div');
        this.spacer.style.cssText = 'pointer-events:none;';

        this.viewport = document.createElement('div');
        this.viewport.style.cssText = 'position:absolute;top:0;left:0;right:0;width:100%;';

        this.container.appendChild(this.spacer);
        this.container.appendChild(this.viewport);

        this._handleScroll = this._handleScroll.bind(this);
        this.container.addEventListener('scroll', this._handleScroll, { passive: true });

        if (typeof ResizeObserver !== 'undefined') {
            this._resizeObserver = new ResizeObserver(() => this._handleScroll());
            this._resizeObserver.observe(this.container);
        }

        this._rafId = null;
    }

    setItems(items) {
        const prevLen = this.items.length;
        this.items = items || [];
        if (this.items.length !== prevLen) {
            this._updateSpacer();
        }
        this._handleScroll();
    }

    setItemHeight(height) {
        this.itemHeight = height;
        this._updateSpacer();
        this._handleScroll();
    }

    _updateSpacer() {
        this.spacer.style.height = (this.items.length * this.itemHeight) + 'px';
    }

    destroy() {
        this.container.removeEventListener('scroll', this._handleScroll);
        if (this._resizeObserver) {
            this._resizeObserver.disconnect();
        }
        if (this._rafId) {
            cancelAnimationFrame(this._rafId);
        }
        this.spacer.remove();
        this.viewport.remove();
    }

    _handleScroll() {
        if (this._rafId) return;
        this._rafId = requestAnimationFrame(() => {
            this._rafId = null;
            this._render();
        });
    }

    scrollToIndex(index) {
        this.container.scrollTop = index * this.itemHeight;
    }

    flush() {
        if (this._rafId) {
            cancelAnimationFrame(this._rafId);
            this._rafId = null;
        }
        this._render();
    }

    _render() {
        const { container, items, itemHeight, buffer, viewport } = this;
        if (!items || !items.length) {
            viewport.innerHTML = '';
            return;
        }

        const scrollTop = container.scrollTop;
        const visibleHeight = container.clientHeight;

        let startIdx = Math.floor(scrollTop / itemHeight) - buffer;
        let endIdx = Math.ceil((scrollTop + visibleHeight) / itemHeight) + buffer;

        startIdx = Math.max(0, startIdx);
        endIdx = Math.min(items.length, endIdx);

        if (startIdx >= endIdx) {
            viewport.innerHTML = '';
            return;
        }

        const fragment = document.createDocumentFragment();
        for (let i = startIdx; i < endIdx; i++) {
            const el = this.renderItem(items[i], i);
            if (el instanceof Node) {
                el.style.position = 'absolute';
                el.style.top = (i * itemHeight) + 'px';
                el.style.left = '0';
                el.style.right = '0';
                fragment.appendChild(el);
            }
        }

        viewport.innerHTML = '';
        viewport.appendChild(fragment);

        if (this.onScroll) {
            this.onScroll({ startIdx, endIdx, scrollTop });
        }
    }
}
