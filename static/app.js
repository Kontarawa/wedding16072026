(function () {
  const base = "/wedding/invitation";

  function invitationHash() {
    const p = window.location.pathname.replace(/\/$/, "");
    const parts = p.split("/").filter(Boolean);
    const i = parts.indexOf("invitation");
    if (i === -1) return "";
    return parts[i + 1] || "";
  }

  const hash = invitationHash();

  const greetingEl = document.getElementById("greeting");
  const firstInput = document.getElementById("first_name");
  const lastInput = document.getElementById("last_name");
  const form = document.getElementById("rsvp-form");
  const errEl = document.getElementById("form-error");
  const mainFlow = document.getElementById("main-flow");
  const pageRoot = document.querySelector(".page");
  const thanks = document.getElementById("thanks-panel");
  const btnShare = document.getElementById("btn-share-invite");

  function setGreeting(first, last) {
    const f = (first || "").trim();
    const l = (last || "").trim();
    if (f || l) {
      const name = [f, l].filter(Boolean).join(" ");
      greetingEl.textContent = `Дорогие ${name}!`;
    } else {
      greetingEl.textContent = "Дорогой Гость!";
    }
  }

  async function loadGuest() {
    if (!hash) {
      setGreeting("", "");
      return;
    }
    try {
      const res = await fetch(`${base}/api/guest/${encodeURIComponent(hash)}`, { headers: { Accept: "application/json" } });
      if (!res.ok) return;
      const data = await res.json();
      const g = data && data.guest;
      if (g && (g.first_name || g.last_name)) {
        if (g.first_name) firstInput.value = g.first_name;
        if (g.last_name) lastInput.value = g.last_name;
        setGreeting(g.first_name, g.last_name);
      } else {
        setGreeting("", "");
      }
    } catch (_) {
      setGreeting("", "");
    }
  }

  function showError(msg) {
    errEl.textContent = msg || "";
    errEl.classList.toggle("hidden", !msg);
  }

  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    showError("");
    const alcoholChoices = [...form.querySelectorAll('input[name="alcohol"]:checked')].map((el) => el.value);
    const payload = {
      first_name: firstInput.value.trim(),
      last_name: lastInput.value.trim(),
      attendance: form.querySelector('input[name="attendance"]:checked')?.value || "",
      transfer: form.querySelector('input[name="transfer"]:checked')?.value || "",
      alcohol: alcoholChoices.join(", "),
      hash: hash || "",
      submitted_at: new Date().toISOString(),
    };
    if (!payload.first_name || !payload.last_name) {
      showError("Укажите имя и фамилию.");
      return;
    }
    if (!payload.attendance || !payload.transfer || !alcoholChoices.length) {
      showError("Пожалуйста, ответьте на все вопросы.");
      return;
    }
    const url = hash ? `${base}/answer/${encodeURIComponent(hash)}` : `${base}/answer`;
    try {
      const res = await fetch(url, {
        method: "POST",
        headers: { "Content-Type": "application/json", Accept: "application/json" },
        body: JSON.stringify(payload),
      });
      if (!res.ok) {
        const t = await res.text();
        throw new Error(t || "Ошибка отправки");
      }

      let serverSheetsConfigured = false;
      try {
        const cfgRes = await fetch(`${base}/api/sheets-config`, { headers: { Accept: "application/json" } });
        if (cfgRes.ok) {
          const cfg = await cfgRes.json();
          serverSheetsConfigured = !!cfg.google_sheets_configured;
        }
      } catch (_) {
        /* офлайн или статика без API */
      }

      if (!serverSheetsConfigured) {
        const webapp = document.querySelector('meta[name="google-sheets-webapp"]')?.getAttribute("content")?.trim();
        if (webapp) {
          fetch(webapp, {
            method: "POST",
            mode: "no-cors",
            headers: { "Content-Type": "text/plain;charset=utf-8" },
            body: JSON.stringify(payload),
          }).catch(() => {});
        }
      }

      enterThanksState();
    } catch (err) {
      showError("Не удалось отправить форму. Попробуйте позже или напишите нам лично.");
      console.error(err);
    }
  });

  function enterThanksState() {
    mainFlow.classList.add("hidden");
    thanks.classList.remove("hidden");
    thanks.classList.add("is-inview");
    if (pageRoot) pageRoot.classList.add("thanks-only");
    thanks.scrollIntoView({ behavior: "smooth", block: "start" });
  }

  /** Текст для «Поделиться»: что за мероприятие, когда (дата и время), где (без данных анкеты) */
  function buildMemoText() {
    const lines = [
      "Свадьба Алины и Романа.",
      "",
      "Когда?",
      "16 июля 2026 (четверг) 15:00.",
      "",
      "Где?",
      "Белая Веранда, ул. Советская 81А, с. Грибоедово.",
    ];
    return lines.join("\n");
  }

  function downloadTextFile(filename, text) {
    const blob = new Blob([text], { type: "text/plain;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = filename;
    a.rel = "noopener";
    a.style.display = "none";
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    setTimeout(() => URL.revokeObjectURL(url), 1500);
  }

  if (btnShare) {
    btnShare.addEventListener("click", async () => {
      const text = buildMemoText();
      const title = "Свадьба Алины и Романа";

      if (navigator.share) {
        try {
          await navigator.share({
            title,
            text,
            url: window.location.href,
          });
          return;
        } catch (err) {
          if (err && err.name === "AbortError") return;
        }
      }
      downloadTextFile("svadba-alina-i-roman.txt", text);
    });
  }

  const heroImg = document.querySelector(".hero-cover");
  if (heroImg) {
    heroImg.addEventListener("error", () => {
      heroImg.src =
        "data:image/svg+xml," +
        encodeURIComponent(
          `<svg xmlns='http://www.w3.org/2000/svg' width='640' height='800' viewBox='0 0 640 800'><defs><linearGradient id='g' x1='0' y1='0' x2='1' y2='1'><stop offset='0' stop-color='#e8ebe5'/><stop offset='1' stop-color='#d5ddd0'/></linearGradient></defs><rect width='640' height='800' fill='url(#g)'/><text x='50%' y='46%' dominant-baseline='middle' text-anchor='middle' fill='#4a5a44' font-family='system-ui,sans-serif' font-size='28'>Алина и Роман</text><text x='50%' y='52%' dominant-baseline='middle' text-anchor='middle' fill='#6a7a62' font-family='system-ui,sans-serif' font-size='15'>static/couple.png</text></svg>`
        );
    });
  }

  function initReveal() {
    const nodes = document.querySelectorAll(".reveal:not(#thanks-panel)");
    if (!nodes.length) return;
    if (!("IntersectionObserver" in window)) {
      nodes.forEach((el) => el.classList.add("is-inview"));
      return;
    }
    const io = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (!entry.isIntersecting) return;
          entry.target.classList.add("is-inview");
          io.unobserve(entry.target);
        });
      },
      { rootMargin: "0px 0px -8% 0px", threshold: 0.08 }
    );
    nodes.forEach((el, i) => {
      el.style.setProperty("--d", `${Math.min(i * 45, 200)}ms`);
      io.observe(el);
    });
  }

  loadGuest();
  initReveal();
})();
