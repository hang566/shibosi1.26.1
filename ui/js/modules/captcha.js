function generateCaptcha(length) {
  const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789';
  let result = '';
  for (let i = 0; i < length; i++) {
    result += chars.charAt(Math.floor(Math.random() * chars.length));
  }
  return result;
}

function createCaptcha(options) {
  const opts = {
    container: 'captcha-container',
    length: 4,
    onSuccess: null,
    onError: null,
    ...options
  };

  const container = document.getElementById(opts.container);
  if (!container) {
    console.error('Captcha container not found:', opts.container);
    return;
  }

  let currentCaptcha = generateCaptcha(opts.length);

  container.innerHTML = `
    <div class="captcha-box" id="captcha-box">${currentCaptcha}</div>
    <input type="text" class="captcha-input" id="captcha-input" placeholder="输入验证码" maxlength="${opts.length}">
    <button class="captcha-refresh" id="captcha-refresh">🔄</button>
  `;

  const box = document.getElementById('captcha-box');
  const input = document.getElementById('captcha-input');
  const refreshBtn = document.getElementById('captcha-refresh');

  const refresh = () => {
    currentCaptcha = generateCaptcha(opts.length);
    box.textContent = currentCaptcha;
    input.value = '';
    input.classList.remove('captcha-error', 'captcha-success');
  };

  box.addEventListener('click', refresh);
  refreshBtn.addEventListener('click', refresh);

  input.addEventListener('input', () => {
    input.classList.remove('captcha-error', 'captcha-success');
  });

  return {
    validate: () => {
      const value = input.value.trim();
      if (value === currentCaptcha) {
        input.classList.add('captcha-success');
        if (opts.onSuccess) opts.onSuccess();
        return true;
      } else {
        input.classList.add('captcha-error');
        if (opts.onError) opts.onError();
        setTimeout(refresh, 500);
        return false;
      }
    },
    refresh,
    getValue: () => input.value.trim(),
    setValue: (val) => { input.value = val; },
    clear: () => { input.value = ''; input.classList.remove('captcha-error', 'captcha-success'); }
  };
}

function initCaptcha() {
  const containers = document.querySelectorAll('.captcha-auto');
  containers.forEach((container, index) => {
    const id = `captcha-container-${index}`;
    container.id = id;
    container.classList.add('captcha-container');
    window[`captcha${index}`] = createCaptcha({ container: id });
  });
}

document.addEventListener('DOMContentLoaded', initCaptcha);