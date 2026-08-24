
(function(){
  var widget = document.getElementById('compareWidget');
  if (!widget) return; // script.js is shared by pages with no compare widget (embed/ch-*.html)
  var inner = document.getElementById('compareInner');
  var layer = document.getElementById('afterLayer');
  var handle = document.getElementById('compareHandle');
  var beforeImg = document.getElementById('beforeImg');
  var afterImg = document.getElementById('afterImg');

  // The widget's own frame is a fixed 16:9 box (CSS aspect-ratio) so it reads
  // like a video player regardless of content length -- real vertical
  // scrolling happens *inside* it. Both images render at their real natural
  // aspect ratio (no cropping), and `inner`'s height is set to the taller of
  // the two: a redesign that changed real page length (this one compressed
  // 8340px down to 2932px) means one side runs out of real content before
  // the other, which is shown honestly, not padded or stretched to match.
  // Live-embed mode (real <iframe>s, see the CSS comment above this same
  // widget's rules): neither side's real HEIGHT is readable from here, so
  // there's nothing to measure or set there -- the fixed 16:9 .compare-frame
  // box plus this CSS mode's height:100% rules already size both sides
  // correctly on their own. WIDTH is a different story: an iframe is a real,
  // live document that lays itself out against its own actual rendered
  // width, unlike a bitmap <img> that just gets visually windowed by a
  // narrower crop without its own content reflowing. So the after side
  // still needs the exact same "size it to the full widget width, let
  // .after-layer's own narrower width visually clip it" trick as the image
  // case below -- confirmed the hard way: without this, the live "after"
  // page rendered as if the browser were only as wide as whatever sliver
  // was currently revealed, triggering mobile/narrow-breakpoint CSS at any
  // reveal position other than ~100%.
  var isLive = beforeImg.tagName === 'IFRAME';
  function layout(){
    var w = widget.clientWidth;
    if (isLive) {
      afterImg.style.width = w + 'px';
      return;
    }
    var beforeH = beforeImg.naturalWidth ? w * (beforeImg.naturalHeight / beforeImg.naturalWidth) : 0;
    var afterH = afterImg.naturalWidth ? w * (afterImg.naturalHeight / afterImg.naturalWidth) : 0;
    inner.style.height = Math.max(beforeH, afterH) + 'px';
    afterImg.style.width = w + 'px'; // full-widget width, then clipped narrower by .after-layer's own width -- same reveal-window trick as before
  }
  function whenReady(img, cb){
    if (isLive) { cb(); return; }
    if (img.complete && img.naturalWidth) cb();
    else img.addEventListener('load', cb);
  }
  whenReady(beforeImg, layout);
  whenReady(afterImg, layout);
  window.addEventListener('resize', layout);

  function applyPct(pct){
    pct = Math.min(Math.max(pct, 0), 100);
    layer.style.width = pct + '%';
    handle.style.left = pct + '%';
    handle.setAttribute('aria-valuenow', Math.round(pct));
  }
  function setPos(clientX){
    var rect = widget.getBoundingClientRect();
    var x = Math.min(Math.max(clientX - rect.left, 0), rect.width);
    applyPct((x / rect.width) * 100);
  }
  // Dragging starts only from the handle itself, not anywhere in the widget
  // -- the widget's background now has a real, independent job (native
  // vertical scroll), so a pointerdown anywhere used to fight that gesture
  // by also jumping the horizontal reveal. The handle is a precise, visible
  // grab target; scrolling the rest of the widget no longer touches it.
  var dragging = false;
  handle.addEventListener('pointerdown', function(e){ dragging = true; e.preventDefault(); });
  window.addEventListener('pointermove', function(e){ if (dragging) setPos(e.clientX); });
  window.addEventListener('pointerup', function(){ dragging = false; });
  handle.addEventListener('keydown', function(e){
    var current = parseFloat(handle.getAttribute('aria-valuenow')) || 50;
    if (e.key === 'ArrowLeft'){ applyPct(current - 5); e.preventDefault(); }
    else if (e.key === 'ArrowRight'){ applyPct(current + 5); e.preventDefault(); }
    else if (e.key === 'Home'){ applyPct(0); e.preventDefault(); }
    else if (e.key === 'End'){ applyPct(100); e.preventDefault(); }
  });
})();
function getBaseDir(){
  // The directory the deployed page actually lives in, resolved at click
  // time -- correct whether this is being previewed on a local server or
  // already live on GitHub Pages, without needing a build-time URL baked
  // in. A `<link rel="canonical">` tag (set if --canonical-url was given at
  // build time) wins when present, since it's a real, deliberately-declared
  // deploy target; otherwise falls back to wherever the page actually is
  // right now.
  var canonical = document.querySelector('link[rel="canonical"]');
  var url = (canonical ? canonical.href : location.href).split('#')[0].split('?')[0];
  return url.substring(0, url.lastIndexOf('/') + 1);
}
function copyPageLink(){
  copyText(getBaseDir() + 'index.html', event.target);
}
function viewSource(e){
  // The old version linked href="index.html" -- that's just this same
  // page, so clicking it did nothing observable. view-source: on the
  // page's own real, resolved URL actually shows the real HTML, matching
  // what a developer clicking "View source" expects.
  e.preventDefault();
  window.open('view-source:' + location.href.split('#')[0].split('?')[0], '_blank');
}
function escAttr(s){
  // The generated <iframe> is copy-pasted as raw HTML text onto someone
  // else's page -- `title` is real content (a subject name, a category
  // label) that can legitimately contain a literal quote character, which
  // would otherwise terminate the title="..." attribute early and produce
  // broken markup for the person pasting it.
  return String(s).replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}
function copyCompareEmbed(title){
  // The widget itself is now a fixed 16:9 frame (not content-length-driven),
  // so the embed fragment's real height is predictable from its rendered
  // width: measured for real at a representative 700px-wide embed, the
  // frame plus its surrounding chrome (wrap padding, the "via" attribution
  // line) renders at ~486px -- 520 leaves real margin without being
  // needlessly oversized the way a flat 700px default was before this rev.
  var code = '<iframe src="' + getBaseDir() + 'embed/compare.html" width="100%" height="520" '
    + 'style="border:1px solid #ddd;max-width:900px;" loading="lazy" title="' + escAttr(title) + '"></iframe>';
  copyText(code, event.target);
}
function copyChapterEmbed(num, title){
  // A chapter's real height depends on its own before/after fact text
  // length (often the longer driver than the narrative prose) and varies
  // per chapter -- 480 comfortably fit every chapter in the run this was
  // measured against (worst case ~421px), but a future run with longer
  // real facts could still need more; there's no generic height that's
  // exactly right for arbitrary future content.
  var code = '<iframe src="' + getBaseDir() + 'embed/ch-' + num + '.html" width="100%" height="480" '
    + 'style="border:1px solid #ddd;max-width:700px;" loading="lazy" title="' + escAttr(title) + '"></iframe>';
  copyText(code, event.target);
}
function copyText(text, btn){
  if (navigator.clipboard) { navigator.clipboard.writeText(text); }
  var original = btn.textContent;
  btn.textContent = 'Copied';
  setTimeout(function(){ btn.textContent = original; }, 1500);
}
