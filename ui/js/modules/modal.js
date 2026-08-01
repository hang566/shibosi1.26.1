function openModal(id) {
  const modal = document.getElementById(id || 'modal');
  if (modal) {
    modal.classList.add('active');
    console.log('[Modal Debug] Modal opened:', id, 'Timestamp:', Date.now());
    console.trace('[Modal Debug] Call stack:');
  }
}

function closeModal(id) {
  const modal = document.getElementById(id || 'modal');
  if (modal) {
    modal.classList.remove('active');
  }
}

document.addEventListener('DOMContentLoaded', function() {
  document.querySelectorAll('.modal').forEach(function(modal) {
    modal.addEventListener('click', function(e) {
      if (e.target === modal) {
        closeModal(modal.id);
      }
    });
  });
});