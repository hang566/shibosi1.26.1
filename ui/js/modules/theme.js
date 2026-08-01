function setTheme(theme) {
  let targetTheme = '';

  if (theme === 'light') {
    targetTheme = '';
  } else if (theme === 'system') {
    targetTheme = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'theme-dark' : '';
  } else if (theme === 'time') {
    const hour = new Date().getHours();
    targetTheme = (hour >= 19 || hour < 7) ? 'theme-dark' : '';
  } else {
    targetTheme = 'theme-' + theme;
  }

  document.body.className = targetTheme;
  localStorage.setItem('primoUI_theme', theme);
}

function resetTheme() {
  localStorage.removeItem('primoUI_theme');
  document.body.className = '';
}

document.addEventListener('DOMContentLoaded', function() {
  const savedTheme = localStorage.getItem('primoUI_theme') || 'light';
  setTheme(savedTheme);
});