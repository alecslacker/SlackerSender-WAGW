/* Splash boot SlackerSender — dipindah dari inline <script> agar CSP script-src 'self' berlaku.
 * Copyright © 2026 Duta Corpora Indonesia — All rights reserved */
// Splash dilepas setelah durasi minimum TERPENUHI dan aplikasi sudah
// ter-render (atau paksa lepas setelah 9 detik bila bundle gagal).
// User login → landing-entry redirect ke /dashboard, splash ikut hilang.
(function () {
  var splash = document.getElementById('splash');
  var MIN_MS = 3800;
  var started = Date.now();
  var done = false;
  function hide() {
    if (done) return;
    done = true;
    splash.classList.add('is-done');
    setTimeout(function () { splash.remove(); }, 550);
  }
  function appReady() {
    var root = document.getElementById('root');
    return root && root.childElementCount > 0;
  }
  function wait() {
    if (appReady() && Date.now() - started >= MIN_MS) return hide();
    if (Date.now() - started > 9000) return hide();
    setTimeout(wait, 60);
  }
  wait();
})();
