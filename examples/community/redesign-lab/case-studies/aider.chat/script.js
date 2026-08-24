
(function(){
  var widget = document.getElementById('compareWidget');
  if (!widget) return; // script.js is shared by pages with no compare widget (embed/ch-*.html)
  var inner = document.getElementById('compareInner');
  var layer = document.getElementById('afterLayer');
  var divider = document.getElementById('compareDivider');
  var handle = document.getElementById('compareHandle');
  var beforeImg = document.getElementById('beforeImg');
  var afterImg = document.getElementById('afterImg');
  var isLive = beforeImg.tagName === 'IFRAME';

  // Screenshot mode only: the widget's own frame is a fixed 16:9 box (CSS
  // aspect-ratio) so it reads like a video player regardless of content
  // length -- real vertical scrolling happens *inside* it. Both images
  // render at their real natural aspect ratio (no cropping), and `inner`'s
  // height is set to the taller of the two: a redesign that changed real
  // page length (this one compressed 8340px down to 2932px) means one side
  // runs out of real content before the other, shown honestly, not padded
  // or stretched to match. Live-embed mode needs none of this: neither
  // side's real height is readable (the before iframe is cross-origin),
  // and with .after-layer now always the full widget width (see the CSS
  // comment on that rule), each iframe's own plain width:100%;height:100%
  // already sizes both sides correctly with no JS involvement at all.
  function layout(){
    if (isLive) return;
    var w = widget.clientWidth;
    var beforeH = beforeImg.naturalWidth ? w * (beforeImg.naturalHeight / beforeImg.naturalWidth) : 0;
    var afterH = afterImg.naturalWidth ? w * (afterImg.naturalHeight / afterImg.naturalWidth) : 0;
    inner.style.height = Math.max(beforeH, afterH) + 'px';
  }
  function whenReady(img, cb){
    if (isLive) return;
    if (img.complete && img.naturalWidth) cb();
    else img.addEventListener('load', cb);
  }
  whenReady(beforeImg, layout);
  whenReady(afterImg, layout);
  window.addEventListener('resize', layout);

  function applyPct(pct){
    pct = Math.min(Math.max(pct, 0), 100);
    // clip-path, not width -- see the .after-layer CSS comment. inset(top
    // right bottom left): 0 from top/bottom/left, (100-pct)% off the right
    // edge, so pct=100 clips nothing (fully revealed) and pct=0 clips
    // everything (fully hidden), matching the old width-based behavior
    // exactly, just computed on the GPU instead of forcing layout.
    layer.style.clipPath = 'inset(0 ' + (100 - pct) + '% 0 0)';
    divider.style.left = pct + '%';
    handle.style.left = pct + '%';
    handle.setAttribute('aria-valuenow', Math.round(pct));
  }
  // Drag smoothness, confirmed the hard way on a real deployed live-embed
  // page (two composited iframes, one with an actively-decoding <video> --
  // real per-frame compositing cost neither the old static-screenshot
  // widget nor a simple drag demo has to pay):
  // 1. widget.getBoundingClientRect() forces a synchronous layout
  //    recalculation -- calling it on every single raw pointermove event
  //    (which can fire well over 60 times/sec on a fast mouse) was real,
  //    avoidable work on top of the compositing cost above. The widget's
  //    own box doesn't move mid-drag, so it's captured once at
  //    pointerdown instead.
  // 2. Every pointermove wrote directly to style.width/style.left
  //    synchronously, with no batching -- on a page already busy
  //    compositing a live video, this can produce more style writes than
  //    the browser can actually paint, which reads as stutter, not
  //    smooth motion. Coalesced into one applyPct() per animation frame.
  var dragging = false;
  var dragRect = null;
  var pendingPct = null;
  var rafScheduled = false;
  function scheduleApply(){
    if (rafScheduled) return;
    rafScheduled = true;
    requestAnimationFrame(function(){
      rafScheduled = false;
      if (pendingPct !== null){ applyPct(pendingPct); pendingPct = null; }
    });
  }
  function setPos(clientX){
    var x = Math.min(Math.max(clientX - dragRect.left, 0), dragRect.width);
    pendingPct = (x / dragRect.width) * 100;
    scheduleApply();
  }
  // Dragging starts only from the handle itself, not anywhere in the widget
  // -- the widget's background now has a real, independent job (native
  // vertical scroll), so a pointerdown anywhere used to fight that gesture
  // by also jumping the horizontal reveal. The handle is a precise, visible
  // grab target; scrolling the rest of the widget no longer touches it.
  //
  // Listeners live on the handle itself, not window, paired with
  // setPointerCapture: without capture, a fast drag that crosses over
  // either live iframe mid-gesture can hand pointer events to that
  // iframe's own document instead of continuing to reach this page's
  // listeners -- a real gap the old static-screenshot version never had
  // anything underneath it to hit. Capture pins every event for this
  // gesture to the handle regardless of what's visually under the
  // cursor, iframe or not.
  handle.addEventListener('pointerdown', function(e){
    dragging = true;
    dragRect = widget.getBoundingClientRect();
    handle.setPointerCapture(e.pointerId);
    e.preventDefault();
  });
  handle.addEventListener('pointermove', function(e){ if (dragging) setPos(e.clientX); });
  handle.addEventListener('pointerup', function(e){
    dragging = false;
    dragRect = null;
    handle.releasePointerCapture(e.pointerId);
  });
  handle.addEventListener('pointercancel', function(){ dragging = false; dragRect = null; });
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
