/* Keep in sync with DEFAULT_THEME / FAVICON_REV in web/src/lib/theme.tsx (dark only: control) */
(function () {
  var t = "control";
  localStorage.setItem("nats-consol-theme", t);
  document.documentElement.setAttribute("data-theme", t);
  document.documentElement.style.colorScheme = "dark";

  var rev = "20260822b";
  var hrefs = {
    "16": "/favicon-16x16.png?v=" + rev,
    "32": "/favicon-32x32.png?v=" + rev,
    any: "/favicon.png?v=" + rev,
    apple: "/apple-touch-icon.png?v=" + rev,
    alternate: "/favicon.png?v=" + rev,
  };

  var links = document.querySelectorAll("link[data-nc-favicon]");
  for (var i = 0; i < links.length; i++) {
    var slot = links[i].getAttribute("data-nc-favicon");
    if (slot && hrefs[slot]) {
      links[i].setAttribute("href", hrefs[slot]);
    }
  }
})();
