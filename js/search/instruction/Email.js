// ============================================================
//  /Email 指令 — 打开系统邮箱客户端，可指定收件人
// ============================================================
window.EmailCommand = {
  name: 'Email',
  description: '打开邮箱客户端',

  execute: function (email) {
    // 打开系统默认邮箱客户端；传入 email 则预填收件人
    window.location.href = email ? ('mailto:' + email) : 'mailto:';
  }
};
