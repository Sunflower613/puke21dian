(function () {
    const THEME_KEY = 'blackjack_theme';

    function applyTheme(theme) {
        const nextTheme = theme === 'dark' ? 'dark' : 'light';
        document.documentElement.dataset.theme = nextTheme;
        localStorage.setItem(THEME_KEY, nextTheme);
        document.querySelectorAll('[data-theme-toggle]').forEach((button) => {
            button.textContent = nextTheme === 'dark' ? '☀' : '🌙';
            button.setAttribute('aria-label', nextTheme === 'dark' ? '切换到白天模式' : '切换到黑夜模式');
            button.title = nextTheme === 'dark' ? '切换到白天模式' : '切换到黑夜模式';
        });
    }

    window.initThemeToggle = function initThemeToggle() {
        const savedTheme = localStorage.getItem(THEME_KEY) || 'light';
        applyTheme(savedTheme);

        document.querySelectorAll('[data-theme-toggle]').forEach((button) => {
            button.addEventListener('click', () => {
                const currentTheme = document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light';
                applyTheme(currentTheme === 'dark' ? 'light' : 'dark');
            });
        });
    };
})();
