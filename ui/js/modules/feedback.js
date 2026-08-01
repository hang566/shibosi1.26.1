function showToast(msg, duration, type) {
  const message = msg || '这是一条消息提示';
  const delay = duration || 2000;
  const toastType = type || 'info';

  const toast = document.createElement('div');
  toast.className = 'toast toast-' + toastType;
  toast.textContent = message;
  document.body.appendChild(toast);

  setTimeout(() => {
    toast.style.opacity = '0';
    toast.style.transform = 'translateX(-50%) translateY(20px)';
    setTimeout(() => toast.remove(), 200);
  }, delay);
}

function showPopconfirm(text) {
  const message = text || '确定执行此操作？';

  return new Promise(function(resolve) {
    const popconfirm = document.createElement('div');
    popconfirm.className = 'popconfirm';
    
    popconfirm.innerHTML = `
      <div class="popconfirm-content">
        <div class="popconfirm-header">
          <span class="popconfirm-icon">⚠</span>
          <span class="popconfirm-title">确认操作</span>
        </div>
        <div class="popconfirm-body">${message}</div>
        <div class="popconfirm-footer">
          <button class="popconfirm-cancel">取消</button>
          <button class="btn-danger popconfirm-confirm">确定</button>
        </div>
      </div>
    `;

    const cancelBtn = popconfirm.querySelector('.popconfirm-cancel');
    const confirmBtn = popconfirm.querySelector('.popconfirm-confirm');

    cancelBtn.addEventListener('click', function() {
      popconfirm.remove();
      resolve(false);
    });

    confirmBtn.addEventListener('click', function() {
      popconfirm.remove();
      resolve(true);
    });

    document.body.appendChild(popconfirm);
    
    setTimeout(() => {
      popconfirm.classList.add('active');
    }, 10);
  });
}

function showNotification(title, body) {
  const notificationTitle = title || '消息提醒';
  const notificationBody = body || '您有新的待办事项';

  if ('Notification' in window && Notification.permission === 'granted') {
    new Notification(notificationTitle, { body: notificationBody });
  } else if ('Notification' in window && Notification.permission !== 'denied') {
    Notification.requestPermission().then(permission => {
      if (permission === 'granted') {
        new Notification(notificationTitle, { body: notificationBody });
      }
    });
  }
}

function showLoading(text) {
  const loadingText = text || '加载中...';
  
  const overlay = document.createElement('div');
  overlay.className = 'loading-overlay';
  overlay.id = 'loading-overlay';
  
  overlay.innerHTML = `
    <div class="loading-overlay-content">
      <span class="loading"></span>
      <span>${loadingText}</span>
    </div>
  `;
  
  document.body.appendChild(overlay);
  
  setTimeout(() => {
    overlay.classList.add('active');
  }, 10);
}

function hideLoading() {
  const overlay = document.getElementById('loading-overlay');
  if (overlay) {
    overlay.classList.remove('active');
    setTimeout(() => overlay.remove(), 200);
  }
}

function togglePopover(popoverId) {
  const popover = document.getElementById(popoverId);
  if (popover) {
    popover.classList.toggle('active');
  }
}

function closeAllPopovers() {
  document.querySelectorAll('.popover').forEach(p => p.classList.remove('active'));
}

document.addEventListener('click', function(e) {
  if (!e.target.closest('.popover')) {
    closeAllPopovers();
  }
});

document.addEventListener('DOMContentLoaded', function() {
  document.querySelectorAll('.slider-input').forEach(function(slider) {
    function updateProgress() {
      const min = parseFloat(slider.getAttribute('min')) || 0;
      const max = parseFloat(slider.getAttribute('max')) || 100;
      const value = parseFloat(slider.value);
      const progress = ((value - min) / (max - min)) * 100;
      slider.style.setProperty('--progress', progress + '%');
    }
    updateProgress();
    slider.addEventListener('input', updateProgress);
  });
});