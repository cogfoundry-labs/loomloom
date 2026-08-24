
// Initializes every .compare-frame on the page independently (the case
// study now renders this widget twice -- the frozen live top-of-page one,
// and the full-page screenshot one behind the "see the full page" toggle
// -- see the render_compare_widget_html comment on why none of its inner
// pieces carry ids any more). Each instance's pieces are found relative to
// its own .compare-frame, not via a single fixed set of page-wide ids.
Array.prototype.forEach.call(document.querySelectorAll('.compare-frame'), function(frameEl){
  var widget = frameEl.querySelector('.compare');
  if (!widget) return; // script.js is shared by pages with no compare widget (embed/ch-*.html)
  var inner = frameEl.querySelector('.compare-inner');
  var layer = frameEl.querySelector('.after-layer');
  var divider = frameEl.querySelector('.divider');
  var handle = frameEl.querySelector('.handle');
  var beforeImg = inner.querySelector(':scope > iframe, :scope > img');
  var afterImg = layer.querySelector('iframe, img');
  var isLive = beforeImg.tagName === 'IFRAME';

  // Screenshot mode only: the widget's own frame is a fixed 16:9 box (CSS
  // aspect-ratio) so it reads like a video player regardless of content
  // length -- real vertical scrolling happens *inside* it. Both images
  // render at their real natural aspect ratio (no cropping), and `inner`'s
  // height is set to the taller of the two: a redesign that changed real
  // page length (this one compressed 8340px down to 2932px) means one side
  // runs out of real content before the other, shown honestly, not padded
  // or stretched to match. Live-embed mode needs none of this: both
  // iframes are permanently non-interactive and frozen to the top of each
  // real page (see the CSS comment on that rule) -- there's no scroll
  // height to measure or match here at all.
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
    // right bottom left): 0 from top/right/bottom, pct% off the LEFT edge,
    // so the after-layer (the redesign) is only ever revealed to the right
    // of the handle, matching the page's own "BEFORE -> AFTER" labels
    // (before on the left, after on the right).
    layer.style.clipPath = 'inset(0 0 0 ' + pct + '%)';
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
  // Dragging starts only from the handle itself, not anywhere in the
  // widget -- listeners live on the handle, paired with setPointerCapture
  // so a fast drag stays glued to the handle regardless of what's under
  // the cursor mid-gesture.
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
  handle.addEventListener('pointercancel', function(){
    dragging = false;
    dragRect = null;
  });
  handle.addEventListener('keydown', function(e){
    var current = parseFloat(handle.getAttribute('aria-valuenow')) || 50;
    if (e.key === 'ArrowLeft'){ applyPct(current - 5); e.preventDefault(); }
    else if (e.key === 'ArrowRight'){ applyPct(current + 5); e.preventDefault(); }
    else if (e.key === 'Home'){ applyPct(0); e.preventDefault(); }
    else if (e.key === 'End'){ applyPct(100); e.preventDefault(); }
  });
});

// The "see the full page" toggle: the second .compare-frame (screenshot
// mode, hidden by default) only exists so the top-of-page live widget
// above doesn't need scrolling at all -- see the CSS comment on
// .compare-frame.is-live iframe for why that widget is frozen. Plain
// show/hide, no data to fetch: both screenshots are already real files in
// this build, same as the frozen widget's own poster-frame images.
(function(){
  var btn = document.getElementById('compareFullToggleBtn');
  var panel = document.getElementById('compareFull');
  if (!btn || !panel) return;
  btn.addEventListener('click', function(){
    var showing = !panel.hidden;
    panel.hidden = showing;
    btn.setAttribute('aria-expanded', String(!showing));
    btn.textContent = showing ? btn.getAttribute('data-show-label') : btn.getAttribute('data-hide-label');
    if (!showing){
      // A real, confirmed bug: this panel starts `hidden` (display:none),
      // so its .compare-inner's height -- computed once from each image's
      // natural size times the *then-zero* widget.clientWidth (see the
      // layout() comment above) -- got permanently set to 0px before this
      // click ever happened. With the after-layer's height:100% resolving
      // against that real (if zero) explicit height, it collapsed to
      // nothing and clipped away 100% of its own content -- the before
      // image, a normal in-flow block, wasn't affected and rendered fine,
      // which is exactly why only "before" was ever visible no matter
      // where the handle was dragged. layout() never re-runs on its own
      // once the panel becomes visible -- only a resize event or a fresh
      // image load re-triggers it -- so it has to be forced here, now that
      // widget.clientWidth is finally real.
      window.dispatchEvent(new Event('resize'));
      panel.scrollIntoView({behavior: 'smooth', block: 'nearest'});
    }
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
