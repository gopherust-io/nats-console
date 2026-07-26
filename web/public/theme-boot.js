/* Keep in sync with THEME_IDS / THEMES in web/src/lib/theme.tsx (DEFAULT_THEME = control) */
(function () {
  var t = localStorage.getItem("nats-consol-theme");
  var themes = ["control", "control-light"];
  if (!t || themes.indexOf(t) === -1) {
    t = "control";
  }
  document.documentElement.setAttribute("data-theme", t);
})();
