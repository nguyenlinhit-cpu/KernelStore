// app.js — phần tương tác nhỏ phía trình duyệt cho frontend SSR (HTMX lo phần còn lại).
// Thay các signal/closure của Leptos: toast, ẩn/hiện mật khẩu, gallery, tự sinh slug,
// chọn sao, upload ảnh, gợi ý tìm kiếm và chat realtime qua WebSocket.
(function () {
  "use strict";

  // ── Toast "[INFO]/[WARN]/[ERROR]/[OK]" (server gửi qua HX-Trigger: ks-toast) ──
  var TOAST_CLASS = {
    INFO: "term-info",
    WARN: "term-warn",
    ERROR: "term-error",
    OK: "text-[var(--fg-primary)]",
  };

  function showToast(level, message) {
    var host = document.getElementById("toasts");
    if (!host) return;
    var box = document.createElement("div");
    box.className = "term-box p-2 text-xs font-mono flex items-start gap-2";
    var tag = document.createElement("span");
    tag.className = TOAST_CLASS[level] || "term-info";
    tag.textContent = "[" + level + "]";
    var text = document.createElement("span");
    text.className = "flex-1 break-words";
    text.textContent = message;
    var close = document.createElement("button");
    close.className = "term-muted hover:text-[var(--fg-primary)] shrink-0";
    close.textContent = "x";
    close.onclick = function () { box.remove(); };
    box.append(tag, text, close);
    host.appendChild(box);
    setTimeout(function () { box.remove(); }, 4000); // tự ẩn sau 4 giây
  }

  document.addEventListener("ks-toast", function (e) {
    showToast(e.detail.level, e.detail.message);
  });

  // ── Bỏ tham số rỗng của form lọc (URL gọn như apply() của bản Rust) ──
  document.addEventListener("htmx:configRequest", function (e) {
    var form = e.detail.elt.closest && e.detail.elt.closest("form[data-strip-empty]");
    if (!form || e.detail.verb !== "get") return;
    var params = e.detail.parameters;
    var keys = typeof params.keys === "function" ? Array.from(params.keys()) : Object.keys(params);
    keys.forEach(function (k) {
      var v = typeof params.get === "function" ? params.get(k) : params[k];
      if (typeof v === "string" && v.trim() === "") {
        if (typeof params.delete === "function") params.delete(k); else delete params[k];
      }
    });
  });

  // slugify giống bản Rust: chữ thường, giữ chữ/số (kể cả tiếng Việt), còn lại thành "-".
  function slugify(raw) {
    var out = "";
    var prevDash = false;
    for (var ch of raw.toLowerCase()) {
      if (/[\p{L}\p{N}]/u.test(ch)) {
        out += ch;
        prevDash = false;
      } else if (out !== "" && !prevDash) {
        out += "-";
        prevDash = true;
      }
    }
    return out.replace(/-+$/, "");
  }

  // Xem trước ảnh sản phẩm từ ô danh sách URL (mỗi dòng một URL).
  function renderPreviews(form) {
    var box = form.querySelector("[data-image-previews]");
    var area = form.querySelector("[data-image-urls]");
    if (!box || !area) return;
    box.replaceChildren();
    area.value.split("\n").map(function (l) { return l.trim(); }).filter(Boolean).forEach(function (u) {
      var img = document.createElement("img");
      img.src = u;
      img.alt = "preview";
      img.title = u;
      img.className = "h-14 w-14 object-cover rounded border border-[var(--border)] bg-[var(--bg-tertiary)]";
      box.appendChild(img);
    });
  }

  function setFormError(form, msg) {
    var line = form.querySelector("p.term-error");
    if (!line) return;
    line.textContent = "[ERROR] " + msg;
    line.classList.remove("invisible");
  }

  // ── Uỷ quyền sự kiện (hoạt động cả với nội dung HTMX chèn vào sau) ──
  document.addEventListener("click", function (e) {
    var el;
    if ((el = e.target.closest("[data-toggle-password]"))) {
      var input = document.getElementById(el.dataset.togglePassword);
      var show = input.type === "password";
      input.type = show ? "text" : "password";
      el.querySelector(".eye").classList.toggle("hidden", show);
      el.querySelector(".eye-off").classList.toggle("hidden", !show);
      el.title = show ? "ẩn mật khẩu" : "hiện mật khẩu";
    } else if ((el = e.target.closest("[data-gallery-src]"))) {
      var gallery = el.closest("[data-gallery]");
      var main = gallery.querySelector("[data-gallery-main]");
      if (main) main.src = el.dataset.gallerySrc;
      gallery.querySelectorAll("[data-gallery-src]").forEach(function (b) {
        b.classList.toggle("term-active", b === el);
      });
    } else if ((el = e.target.closest("[data-toggle]"))) {
      var target = document.querySelector(el.dataset.toggle);
      var open = target.classList.toggle("hidden") === false;
      el.textContent = open ? el.dataset.labelOpen : el.dataset.labelClosed;
    } else if ((el = e.target.closest("[data-open-next]"))) {
      el.classList.add("hidden");
      el.nextElementSibling.classList.remove("hidden");
    } else if ((el = e.target.closest("[data-close-form]"))) {
      var form = el.closest("form");
      form.classList.add("hidden");
      form.previousElementSibling.classList.remove("hidden");
    } else if ((el = e.target.closest("[data-star]"))) {
      var picker = el.closest("[data-stars]");
      var n = Number(el.dataset.star);
      picker.querySelector("[data-stars-value]").value = n;
      picker.querySelectorAll("[data-star]").forEach(function (b) {
        b.textContent = Number(b.dataset.star) <= n ? "★" : "☆";
      });
      picker.querySelector("[data-stars-label]").textContent = n + "/5";
    }
  });

  document.addEventListener("input", function (e) {
    var el = e.target;
    var form = el.closest("[data-slugify]");
    if (form && el.matches("[data-slug-source]")) {
      var slug = form.querySelector("[data-slug-target]");
      if (slug && slug.dataset.slugAuto === "1") slug.value = slugify(el.value);
    } else if (form && el.matches("[data-slug-target]")) {
      el.dataset.slugAuto = ""; // tự sửa slug → không tự sinh nữa
    }
    if (el.matches("[data-image-urls]")) renderPreviews(el.closest("form"));
  });

  // Upload ảnh sản phẩm ngay khi chọn file; URL trả về được nối vào danh sách ảnh.
  document.addEventListener("change", function (e) {
    var input = e.target;
    if (!input.matches("[data-upload-images]")) return;
    var form = input.closest("form");
    var files = Array.from(input.files || []);
    input.value = ""; // cho phép chọn lại cùng file
    if (!files.length) return;
    var busy = form.querySelector("[data-uploading]");
    var area = form.querySelector("[data-image-urls]");
    busy.classList.remove("invisible");
    input.disabled = true;
    files.reduce(function (chain, file) {
      return chain.then(function () {
        var fd = new FormData();
        fd.append("file", file, file.name);
        return fetch("/x/seller/upload", { method: "POST", body: fd, credentials: "same-origin" })
          .then(function (r) { return r.json(); })
          .then(function (res) {
            if (res.url) {
              area.value = area.value.trim() === "" ? res.url : area.value + "\n" + res.url;
              renderPreviews(form);
            } else {
              setFormError(form, res.error || "upload failed");
            }
          })
          .catch(function (err) { setFormError(form, "network error: " + err); });
      });
    }, Promise.resolve()).finally(function () {
      busy.classList.add("invisible");
      input.disabled = false;
    });
  });

  // Gợi ý tìm kiếm: ẩn khi rời ô nhập / nhấn Esc / gửi form.
  document.addEventListener("focusout", function (e) {
    var wrap = e.target.closest && e.target.closest("[data-suggest]");
    if (!wrap) return;
    setTimeout(function () { wrap.querySelector("[data-suggest-list]").replaceChildren(); }, 150);
  });
  document.addEventListener("keydown", function (e) {
    var wrap = e.target.closest && e.target.closest("[data-suggest]");
    if (wrap && e.key === "Escape") wrap.querySelector("[data-suggest-list]").replaceChildren();
  });

  // ── Chat realtime ──
  var chatSocket = null;

  function chatBubble(m, me) {
    var mine = m.senderId === me;
    var row = document.createElement("div");
    row.className = "flex " + (mine ? "justify-end" : "justify-start");
    row.dataset.msgId = m.id;
    var bubble = document.createElement("div");
    bubble.className = "max-w-[75%] px-3 py-2 rounded text-sm " +
      (mine ? "bg-[var(--bg-tertiary)] text-[var(--fg-primary)]" : "term-sub");
    var text = document.createElement("p");
    text.className = "whitespace-pre-wrap break-words";
    text.textContent = m.content;
    var time = document.createElement("p");
    time.className = "term-muted text-[10px] mt-1";
    time.textContent = (m.createdAt || "").slice(5, 10) + " " + (m.createdAt || "").slice(11, 16);
    bubble.append(text, time);
    row.appendChild(bubble);
    return row;
  }

  function scrollMessages() {
    var box = document.querySelector("[data-chat-messages]");
    if (box) requestAnimationFrame(function () { box.scrollTop = box.scrollHeight; });
  }

  function initChat(root) {
    var chat = root.querySelector ? root.querySelector("[data-chat]") : null;
    if (!chat && root.matches && root.matches("[data-chat]")) chat = root;
    if (!chat) return;
    if (chatSocket) chatSocket.close();
    chatSocket = new WebSocket(chat.dataset.wsUrl);
    chatSocket.onmessage = function (ev) {
      var m;
      try { m = JSON.parse(ev.data); } catch (_) { return; }
      var box = document.querySelector("[data-chat-messages]");
      var current = document.querySelector("[data-chat]");
      if (box && current && current.dataset.conversation === m.conversationId &&
          !box.querySelector('[data-msg-id="' + m.id + '"]')) {
        box.appendChild(chatBubble(m, current.dataset.me));
        scrollMessages();
      }
      htmx.trigger(document.body, "chat-refresh");
    };
    scrollMessages();
  }

  // Rời trang chat (HTMX thay body) → đóng WebSocket.
  document.addEventListener("htmx:beforeSwap", function (e) {
    if (chatSocket && e.detail.target === document.body) {
      chatSocket.close();
      chatSocket = null;
    }
  });

  // Gửi tin xong: xoá ô nhập, cuộn xuống cuối (lỗi thì giữ nguyên nội dung ô nhập).
  document.addEventListener("htmx:afterRequest", function (e) {
    var form = e.detail.elt.closest && e.detail.elt.closest("[data-chat-form]");
    if (form && e.detail.successful) {
      form.reset();
      scrollMessages();
    }
  });

  htmx.onLoad(initChat);
})();
