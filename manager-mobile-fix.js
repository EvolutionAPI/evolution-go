// manager-mobile-fix.js — inject hamburger + drawer behavior for mobile
(function(){
  function init(){
    var sidebar = document.querySelector('div.hidden.md\\:flex.bg-sidebar');
    var header = document.querySelector('header.flex.h-16');
    if(!sidebar || !header) return false;
    if(document.getElementById('hamburger-btn')) return true; // already injected

    // mark sidebar for targeting
    sidebar.id = 'mobile-sidebar';

    // backdrop
    var backdrop = document.createElement('div');
    backdrop.id = 'mobile-sidebar-backdrop';
    backdrop.addEventListener('click', closeDrawer);
    document.body.appendChild(backdrop);

    // hamburger button — insert at start of header
    var btn = document.createElement('button');
    btn.id = 'hamburger-btn';
    btn.setAttribute('aria-label','Abrir menu');
    btn.style.cssText = 'display:flex;align-items:center;justify-content:center;width:40px;height:40px;border-radius:8px;background:transparent;border:none;cursor:pointer;flex-shrink:0;';
    btn.innerHTML = '<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><line x1="3" y1="6" x2="21" y2="6"/><line x1="3" y1="12" x2="21" y2="12"/><line x1="3" y1="18" x2="21" y2="18"/></svg>';
    btn.addEventListener('click', function(e){
      e.stopPropagation();
      var isOpen = sidebar.classList.contains('open');
      isOpen ? closeDrawer() : openDrawer();
    });

    // insert as first child of header
    header.insertBefore(btn, header.firstChild);

    // close drawer when clicking a nav link
    sidebar.querySelectorAll('a').forEach(function(a){
      a.addEventListener('click', closeDrawer);
    });

    // close on escape
    document.addEventListener('keydown', function(e){
      if(e.key === 'Escape') closeDrawer();
    });

    function openDrawer(){
      sidebar.classList.add('open');
      backdrop.classList.add('open');
      document.body.style.overflow = 'hidden';
    }
    function closeDrawer(){
      sidebar.classList.remove('open');
      backdrop.classList.remove('open');
      document.body.style.overflow = '';
    }
    return true;
  }

  // SPA: retry until React mounts (sem limite — bundle pode demorar em rede lenta)
  var timer = setInterval(function(){
    if(init()) clearInterval(timer);
  }, 300);
  // also re-init on navigation (history changes)
  var _pushState = history.pushState;
  history.pushState = function(){ _pushState.apply(this, arguments); setTimeout(init, 400); };
  window.addEventListener('popstate', function(){ setTimeout(init, 400); });
})();
