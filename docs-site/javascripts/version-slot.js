(function () {
  var scheduled = false;

  function scheduleInit() {
    if (scheduled) {
      return;
    }
    scheduled = true;
    requestAnimationFrame(function () {
      scheduled = false;
      init();
    });
  }

  function updateOutdatedBannerVersion() {
    var el = document.querySelector(".md-version-banner__version");
    if (!el) {
      return;
    }

    var match = location.pathname.match(/\/(\d+\.\d+(?:\.\d+)?)(?:\/|$)/);
    if (match) {
      el.textContent = match[1];
      return;
    }

    if (/\/development(?:\/|$)/.test(location.pathname)) {
      el.textContent = "development";
    }
  }

  function relocateVersion() {
    var slot = document.getElementById("md-version-slot");
    if (!slot) {
      return;
    }

    var mount = document.querySelector(".md-header__topic--version-mount");
    if (mount) {
      var mountVersion = mount.querySelector(".md-version");
      if (mountVersion && !slot.contains(mountVersion)) {
        slot.appendChild(mountVersion);
      }
    }
  }

  function bindVersionLinks() {
    document.querySelectorAll(".md-version__link").forEach(function (link) {
      if (link.dataset.versionFullNav === "true") {
        return;
      }
      link.dataset.versionFullNav = "true";
      link.addEventListener(
        "click",
        function (event) {
          if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) {
            return;
          }
          event.preventDefault();
          event.stopPropagation();
          event.stopImmediatePropagation();
          window.location.assign(link.href);
        },
        true
      );
    });
  }

  function init() {
    updateOutdatedBannerVersion();
    relocateVersion();
    bindVersionLinks();
  }

  function observeMount() {
    var mount = document.querySelector(".md-header__topic--version-mount");
    if (!mount) {
      return;
    }
    new MutationObserver(scheduleInit).observe(mount, {
      childList: true,
    });
  }

  function observeSlot() {
    var slot = document.getElementById("md-version-slot");
    if (!slot) {
      return;
    }
    new MutationObserver(scheduleInit).observe(slot, {
      childList: true,
    });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", function () {
      init();
      observeMount();
      observeSlot();
    });
  } else {
    init();
    observeMount();
    observeSlot();
  }

  window.addEventListener("pageshow", init);
})();
