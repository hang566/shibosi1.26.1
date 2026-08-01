function switchTab(tabId, event) {
  document.querySelectorAll('.tab-pane').forEach(pane => pane.classList.remove('active'));
  document.querySelectorAll('.tab-browser').forEach(tab => tab.classList.remove('active'));
  
  const targetPane = document.getElementById(tabId);
  if (targetPane) {
    targetPane.classList.add('active');
  }
  
  if (event && event.target) {
    event.target.classList.add('active');
  }
}

function toggleCollapse(collapseId) {
  const collapse = document.getElementById(collapseId);
  if (collapse) {
    collapse.classList.toggle('show');
  }
}

function switchPill(contentId, event) {
  document.querySelectorAll('.nav-pill').forEach(pill => pill.classList.remove('active'));
  document.querySelectorAll('.pill-content').forEach(content => content.classList.remove('active'));
  
  const targetContent = document.getElementById(contentId);
  if (targetContent) {
    targetContent.classList.add('active');
  }
  
  if (event && event.target) {
    event.target.classList.add('active');
  }
}