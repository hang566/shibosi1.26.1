function initScrollspy(menuSelector, targetSelector) {
  const menu = document.querySelector(menuSelector);
  if (!menu) return;

  const navItems = menu.querySelectorAll('a[href^="#"]');
  const targets = document.querySelectorAll(targetSelector);

  function updateActiveNav() {
    let currentSection = '';

    targets.forEach(target => {
      const rect = target.getBoundingClientRect();
      if (rect.top <= 60 && rect.bottom >= 60) {
        currentSection = target.id;
      }
    });

    navItems.forEach(item => {
      item.classList.toggle('active', item.getAttribute('href') === '#' + currentSection);
    });
  }

  window.addEventListener('scroll', updateActiveNav);
  updateActiveNav();
}